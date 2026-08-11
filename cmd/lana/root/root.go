// Package root provides the cobra root command for the Lana CLI.
package root

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	localagents "github.com/deagy/lana/internal/agents"
	"github.com/deagy/lana/internal/app"
	"github.com/deagy/lana/internal/cli"
	agentscmd "github.com/deagy/lana/internal/cmd/agents"
	"github.com/deagy/lana/internal/cmd/file"
	"github.com/deagy/lana/internal/cmd/knowledge"
	"github.com/deagy/lana/internal/cmd/sdcl"
	"github.com/deagy/lana/internal/cmd/system"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/tools"
	"github.com/deagy/lana/internal/tui"
	"github.com/deagy/lana/pkg/config"
	"github.com/deagy/lana/pkg/version"
)

// Options exposes only injectable conversational dependencies. In particular,
// command wiring does not know about provider credentials or SDKs.
type Options struct {
	// Executor accepts a fully assembled, provider-neutral turn executor.
	Executor      cli.TurnExecutor
	Provider      provider.Client
	Authorizer    tools.Authorizer
	ToolExecutor  tools.Executor
	MaxToolRounds int
	Sessions      cli.SessionStore
	Approvals     *cli.ApprovalBroker
	// AgentExecutor is optional. When supplied it processes structured local
	// agent tasks; it is never a shell-command executor.
	AgentExecutor localagents.Executor
	AgentStore    func(*cobra.Command) (localagents.Store, error)
	IsTerminal    func(io.Reader, io.Writer) bool
}

// NewRootCommand creates the root cobra command with all subcommands.
func NewRootCommand() *cobra.Command { return NewRootCommandWithOptions(Options{}) }

// NewRootCommandWithOptions is useful to embedding applications and tests that
// supply an agent turn kernel or a deterministic fake.
func NewRootCommandWithOptions(options Options) *cobra.Command {
	var workspace, configPath, verbosity string
	var quiet, jsonOutput bool

	cmd := &cobra.Command{
		Use:   "lana",
		Short: "Lana — a local coding-agent CLI",
		Long: `Lana is a local coding-agent CLI.

It provides an interactive terminal conversation, a noninteractive prompt
surface, local workspace tools mediated by the agent runtime, and read-only
inspection of Agentic SDLC run records.

Subcommands:
  agents    Manage local agent roles and structured work items
  exec      Run one noninteractive agent prompt
  file      File operations (read, write, delete, copy, move, search)
  knowledge Read local knowledge records
  sdlc      Inspect Agentic SDLC run records
  system    System commands (version, health, schema)

Examples:
  lana "Explain this repository"
  lana exec "Summarize the test failure" --jsonl
  lana file read src/main.go
  lana sdlc status
`,
		Version:       version.Info(),
		SilenceErrors: false,
		SilenceUsage:  false,
		Args:          cobra.ArbitraryArgs,
	}
	cmd.RunE = func(command *cobra.Command, args []string) error {
		runtime, err := runtimeFor(command, options)
		if err != nil {
			return err
		}
		if err := runtime.Ready(); err != nil {
			return err
		}
		terminal := options.IsTerminal
		if terminal == nil {
			terminal = defaultTerminal
		}
		if terminal(command.InOrStdin(), command.OutOrStdout()) {
			return tui.Run(command.Context(), tui.Options{Runtime: runtime, Approvals: options.Approvals, Color: colorEnabled(), Input: command.InOrStdin(), Output: command.OutOrStdout(), InitialPrompt: strings.Join(args, " ")})
		}
		prompt, err := cli.PromptFromArgsOrInput(args, command.InOrStdin())
		if err != nil {
			return err
		}
		return cli.RunPlain(command.Context(), runtime, prompt, command.OutOrStdout(), command.ErrOrStderr())
	}

	// --config selects a configuration file; --workspace selects where commands
	// operate. They must never share a backing variable.
	cmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", "", "Workspace path")
	// Keep the documented long form. A persistent -c would collide with
	// subcommand-local -c flags while Cobra generates completion scripts.
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "Project config file path")
	// Keep --verbose long-only so future subcommands can reserve their own
	// short flags without colliding with persistent completion output.
	cmd.PersistentFlags().StringVar(&verbosity, "verbose", "", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")
	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Use JSON diagnostic logging (exec event output uses --jsonl)")

	// Resolve configuration once after parsing. The resulting immutable app is
	// attached to the command context for future commands to consume.
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		flags := config.FlagOverrides{}
		persistent := cmd.Root().PersistentFlags()
		if persistent.Lookup("workspace").Changed {
			flags.Workspace = &workspace
		}
		if persistent.Lookup("verbose").Changed {
			flags.LogLevel = &verbosity
		}
		if persistent.Lookup("json").Changed {
			format := "text"
			if jsonOutput {
				format = "json"
			}
			flags.LogFormat = &format
		}
		application, err := app.New(app.Options{Config: config.ResolveOptions{
			ConfigPath: configPath,
			Flags:      flags,
		}})
		if err != nil {
			return err
		}
		cmd.SetContext(app.WithContext(cmd.Context(), application))

		if quiet {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
		}

		return nil
	}

	// Register the local-v1 command surface. Legacy compatibility commands that
	// can spawn arbitrary work, alter lifecycle state, persist uncontained data,
	// activate extensions, or reach Git/MCP remotes are deliberately absent.
	execCommand := cli.NewExecCommand(func(command *cobra.Command) (*cli.Runtime, error) { return runtimeFor(command, options) })
	wireExecSessionFlag(execCommand)
	cmd.AddCommand(execCommand)
	cmd.AddCommand(agentscmd.NewCommand(agentscmd.Options{Executor: options.AgentExecutor, Store: options.AgentStore}))
	cmd.AddCommand(file.NewCommand())
	cmd.AddCommand(knowledge.NewCommand())
	cmd.AddCommand(sdcl.NewCommand())
	cmd.AddCommand(system.NewCommand())

	// Completion commands
	cmd.AddCommand(completionCmd())

	return cmd
}

// wireExecSessionFlag presents the published --session spelling while keeping
// --resume as a hidden compatibility alias for embedded callers. The exec
// command owns the session runtime; root only normalizes its public flags.
func wireExecSessionFlag(cmd *cobra.Command) {
	var sessionID string
	cmd.Flags().StringVar(&sessionID, "session", "", "Resume a stored session")
	if resume := cmd.Flags().Lookup("resume"); resume != nil {
		resume.Hidden = true
	}
	previousPreRunE := cmd.PreRunE
	cmd.PreRunE = func(command *cobra.Command, args []string) error {
		if command.Flags().Changed("session") {
			if command.Flags().Changed("resume") {
				return fmt.Errorf("--session and --resume cannot be used together")
			}
			if err := command.Flags().Set("resume", sessionID); err != nil {
				return fmt.Errorf("set session: %w", err)
			}
		}
		if previousPreRunE != nil {
			return previousPreRunE(command, args)
		}
		return nil
	}
}

func runtimeFor(cmd *cobra.Command, options Options) (*cli.Runtime, error) {
	var sessions cli.SessionStore = options.Sessions
	if sessions == nil {
		workspace, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get workspace: %w", err)
		}
		if application, ok := app.FromContext(cmd.Context()); ok {
			workspace = application.Config().Workspace()
		}
		store, err := session.NewStore(filepath.Join(workspace, ".lana", "sessions"))
		if err != nil {
			return nil, err
		}
		sessions = store
	}
	definitions := make([]provider.ToolDefinition, 0, len(tools.Builtins))
	for _, definition := range tools.Builtins {
		definitions = append(definitions, definition.ProviderDefinition())
	}
	executor := options.Executor
	if executor == nil && (options.Provider != nil || options.Authorizer != nil || options.ToolExecutor != nil) {
		executor = cli.Kernel{Provider: options.Provider, Authorizer: options.Authorizer, Executor: options.ToolExecutor, MaxToolRounds: options.MaxToolRounds}
	}
	return cli.NewRuntime(cli.Options{Executor: executor, Sessions: sessions, Tools: definitions, Permissions: "ask"}), nil
}

func defaultTerminal(input io.Reader, output io.Writer) bool {
	inputFile, inputOK := input.(*os.File)
	outputFile, outputOK := output.(*os.File)
	if !inputOK || !outputOK {
		return false
	}
	inputInfo, inputErr := inputFile.Stat()
	outputInfo, outputErr := outputFile.Stat()
	return inputErr == nil && outputErr == nil && inputInfo.Mode()&os.ModeCharDevice != 0 && outputInfo.Mode()&os.ModeCharDevice != 0
}

func colorEnabled() bool { return os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" }

func completionCmd() *cobra.Command {
	var shell string
	cmd := &cobra.Command{
		Use:   "completion <shell>",
		Short: "Generate autocompletion script",
		Long: `Generate autocompletion script for the specified shell.

Supported shells: bash, zsh, fish, powershell.

Examples:
  lana completion bash > /etc/bash_completion.d/lana
  lana completion zsh > ~/.zsh/completion/_lana
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell = strings.ToLower(args[0])
			switch shell {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletion(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", shell)
			}
		},
	}
	cmd.Flags().StringVarP(&shell, "shell", "s", "", "Shell type (bash, zsh, fish, powershell)")
	return cmd
}
