package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/deagy/lana/internal/approval"
	"github.com/deagy/lana/internal/output"
	"github.com/deagy/lana/internal/providers"
	"github.com/deagy/lana/internal/runner"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/storage"
	"github.com/deagy/lana/internal/tools/impl"
)

var (
	runProvider          string
	runModel             string
	runOutput            string
	runApprovalMode      string
	runTimeout           int
	runSaveSession       bool
	runMaxTurns          int
)

var runCmd = &cobra.Command{
	Use:   "run <prompt>",
	Short: "Execute a prompt non-interactively",
	Long: `Execute a prompt with agent, streaming results in specified format.

This is designed for scripting and automation. Use --output json for structured output.
Approval is auto-denied by default (--approve-all to auto-approve).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Error: configuration not loaded\n")
			os.Exit(output.ExitConfigError)
		}

		prompt := args[0]

		// Get provider and model
		providerName := runProvider
		if providerName == "" {
			providerName = cfg.Provider.Name
		}

		model := runModel
		if model == "" {
			model = cfg.Provider.Model
		}

		// Create session store
		storeDir := cfg.Session.StorePath
		store, err := storage.NewFileStore(storeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating session store: %v\n", err)
			os.Exit(output.ExitSessionError)
		}
		defer store.Close()

		// Create session
		sessionID, err := store.Create(context.Background(), session.CreateOpts{
			Model:    model,
			Provider: providerName,
			Title:    fmt.Sprintf("run: %s", truncatePrompt(prompt, 30)),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating session: %v\n", err)
			os.Exit(output.ExitSessionError)
		}

		// Create provider
		factory := providers.NewFactory(
			providerName,
			model,
			cfg.Provider.Endpoint,
			cfg.Provider.APIKey,
		)
		providerClient, err := factory.Create()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating provider: %v\n", err)
			os.Exit(output.ExitProviderError)
		}

		// Set approval mode
		var policyMode approval.Mode
		switch runApprovalMode {
		case "ask":
			policyMode = approval.AskMode
		case "auto-edit":
			policyMode = approval.AutoEditMode
		case "full-auto":
			policyMode = approval.FullAutoMode
		default:
			policyMode = approval.AskMode
		}
		policy := approval.NewStaticPolicy(policyMode)

		// Initialize tool registry
		workspace, _ := os.Getwd()
		registry, err := impl.InitializeRegistry(workspace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing tools: %v\n", err)
			os.Exit(output.ExitGeneralError)
		}

		// Create formatter
		formatter := output.NewFormatter(runOutput)

		// Execute with timeout
		ctx := context.Background()
		if runTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(runTimeout)*time.Second)
			defer cancel()
		}

		// Run execution loop (types properly typed now)
		r := runner.NewNonInteractiveRunner(
			sessionID,
			store,
			providerClient,
			registry,
			policy,
			formatter,
			runMaxTurns,
		)
		err = r.Run(ctx, prompt)
		if err != nil {
			if runOutput == "json" || runOutput == "jsonl" {
				result := output.Result{
					Status:    "error",
					Error:     err.Error(),
					Timestamp: time.Now().Unix(),
				}
				data, _ := json.Marshal(result)
				fmt.Println(string(data))
			} else {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(output.ExitToolError)
		}

		// Clean up if not saving
		if !runSaveSession {
			store.Delete(context.Background(), sessionID)
		}

		os.Exit(output.ExitSuccess)
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&runProvider, "provider", "p", "", "provider name (default from config)")
	runCmd.Flags().StringVarP(&runModel, "model", "m", "", "model name (default from config)")
	runCmd.Flags().StringVarP(&runOutput, "output", "o", "plain", "output format: plain, json, jsonl")
	runCmd.Flags().StringVar(&runApprovalMode, "approve", "ask", "approval mode: ask, auto-edit, full-auto")
	runCmd.Flags().IntVar(&runTimeout, "timeout", 0, "timeout in seconds (0=no limit)")
	runCmd.Flags().BoolVar(&runSaveSession, "save-session", false, "save session after execution")
	runCmd.Flags().IntVar(&runMaxTurns, "max-turns", 10, "maximum agent turns")
}

func truncatePrompt(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
