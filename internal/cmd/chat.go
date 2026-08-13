package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/deagy/lana/internal/approval"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/providers"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/storage"
	"github.com/deagy/lana/internal/tui"
)

var (
	chatModel    string
	chatProvider string
	resumeID     string
)

var chatCmd = &cobra.Command{
	Use:   "chat [prompt]",
	Short: "Start or continue an interactive chat session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// Use overrides or config
		providerName := chatProvider
		if providerName == "" {
			providerName = cfg.Provider.Name
		}

		model := chatModel
		if model == "" {
			model = cfg.Provider.Model
		}

		// Create session store (use file-based for Phase 3)
		storeDir := cfg.Session.StorePath
		store, err := storage.NewFileStore(storeDir)
		if err != nil {
			return fmt.Errorf("create session store: %w", err)
		}
		defer store.Close()

		ctx := context.Background()

		// Create or resume session
		var sessionID string
		var sess *session.Session

		if resumeID != "" {
			sessionID = resumeID
			sess, err = store.Get(ctx, sessionID)
			if err != nil {
				return fmt.Errorf("resume session: %w", err)
			}
		} else {
			sessionID, err = store.Create(ctx, session.CreateOpts{
				Model:    model,
				Provider: providerName,
				Title:    "Chat Session",
			})
			if err != nil {
				return fmt.Errorf("create session: %w", err)
			}
			sess, _ = store.Get(ctx, sessionID)
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
			return fmt.Errorf("create provider: %w", err)
		}

		// Approval policy
		policy := approval.NewStaticPolicy(approval.Mode(cfg.Approval.Mode))

		// Determine mode: TUI or CLI
		useTUI := tui.IsInteractive() && len(args) == 0 && resumeID == ""

		// Process initial prompt if provided
		if len(args) > 0 {
			prompt := args[0]
			if useTUI && len(args) == 0 {
				// TUI mode with prompt
				return tui.RunWithPrompt(ctx, sessionID, store, providerClient, policy, prompt)
			}
			return runSingleTurn(ctx, sess, store, sessionID, providerClient, policy, prompt)
		}

		// Interactive mode
		if useTUI {
			return tui.Run(ctx, sessionID, store, providerClient, policy)
		}

		return runInteractiveChat(ctx, sess, store, sessionID, providerClient, policy)
	},
}

func runSingleTurn(ctx context.Context, sess *session.Session, store session.Store, sessionID string,
	client provider.Client, policy approval.Policy, prompt string) error {

	fmt.Printf("You: %s\n\n", prompt)

	// Add user message
	userMsg := &session.Message{
		Role:      "user",
		Content:   prompt,
		Timestamp: time.Now(),
	}
	if err := store.AppendMessage(ctx, sessionID, userMsg); err != nil {
		return err
	}

	// Reload session to get updated transcript
	sess, err := store.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	// Send to provider
	req := &provider.Request{
		Messages: toProviderMessages(sess.Transcript),
		Model:    sess.Model,
	}

	reader, err := client.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("chat: %w", err)
	}
	defer reader.Close()

	// Stream response
	var assistantContent string
	fmt.Print("Lana: ")

	for {
		event, err := reader.NextEvent(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read event: %w", err)
		}

		switch e := event.(type) {
		case *provider.MessageDeltaEvent:
			fmt.Print(e.Content)
			assistantContent += e.Content
		case *provider.MessageEndEvent:
			// Done
		case *provider.ToolCallEvent:
			fmt.Printf("\n[Tool Call: %s]\n", e.Name)
		case *provider.ErrorEvent:
			return fmt.Errorf("provider error: %s", e.Message)
		}
	}

	fmt.Println()

	// Add assistant message to transcript
	assistantMsg := &session.Message{
		Role:      "assistant",
		Content:   assistantContent,
		Timestamp: time.Now(),
	}
	if err := store.AppendMessage(ctx, sessionID, assistantMsg); err != nil {
		return err
	}

	fmt.Printf("\nSession: %s\n", sessionID[:8])
	return nil
}

func runInteractiveChat(ctx context.Context, sess *session.Session, store session.Store, sessionID string,
	client provider.Client, policy approval.Policy) error {

	fmt.Printf("Welcome to Lana Chat\nProvider: %s | Model: %s | Session: %s\n\n", client.Name(), client.Model(), sessionID[:8])
	fmt.Println("Type your message and press Enter. Type 'exit' to quit.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("You: ")
		prompt, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}

		prompt = prompt[:len(prompt)-1] // Remove newline
		if prompt == "" {
			continue
		}

		if prompt == "exit" {
			fmt.Println("Goodbye!")
			return nil
		}

		// Add user message
		userMsg := &session.Message{
			Role:      "user",
			Content:   prompt,
			Timestamp: time.Now(),
		}
		if err := store.AppendMessage(ctx, sessionID, userMsg); err != nil {
			fmt.Printf("Error saving message: %v\n", err)
			continue
		}

		// Refresh session from store
		sessRefresh, errRefresh := store.Get(ctx, sessionID)
		if errRefresh != nil {
			fmt.Printf("Error loading session: %v\n", errRefresh)
			continue
		}
		sess = sessRefresh

		// Send to provider
		req := &provider.Request{
			Messages: toProviderMessages(sess.Transcript),
			Model:    sess.Model,
		}

		chatReader, errChat := client.Chat(ctx, req)
		if errChat != nil {
			fmt.Printf("Error: %v\n", errChat)
			continue
		}

		// Stream response
		var assistantContent string
		fmt.Print("Lana: ")

		hasError := false
		for {
			event, err := chatReader.NextEvent(ctx)
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Printf("\nError: %v\n", err)
				hasError = true
				break
			}

			switch e := event.(type) {
			case *provider.MessageDeltaEvent:
				fmt.Print(e.Content)
				assistantContent += e.Content
			case *provider.MessageEndEvent:
				// Done
			case *provider.ToolCallEvent:
				fmt.Printf("\n[Tool Call: %s]\n", e.Name)
			case *provider.ErrorEvent:
				fmt.Printf("\nError: %s\n", e.Message)
				hasError = true
			}
		}

		chatReader.Close()

		if !hasError && assistantContent != "" {
			fmt.Println()

			// Add assistant message
			assistantMsg := &session.Message{
				Role:      "assistant",
				Content:   assistantContent,
				Timestamp: time.Now(),
			}
			if err := store.AppendMessage(ctx, sessionID, assistantMsg); err != nil {
				fmt.Printf("Error saving response: %v\n", err)
			}
		}

		fmt.Println()
	}
}

func toProviderMessages(transcript []session.Message) []provider.Message {
	msgs := make([]provider.Message, len(transcript))
	for i, msg := range transcript {
		msgs[i] = provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return msgs
}

func init() {
	chatCmd.Flags().StringVar(&chatModel, "model", "", "override default model")
	chatCmd.Flags().StringVar(&chatProvider, "provider", "", "override default provider")
	chatCmd.Flags().StringVar(&resumeID, "resume", "", "resume a previous session by ID")
}
