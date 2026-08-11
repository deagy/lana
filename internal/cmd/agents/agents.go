// Package agents exposes Lana's local agent registry and task queue.
package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	localagents "github.com/deagy/lana/internal/agents"
	"github.com/deagy/lana/internal/app"
)

// Options keeps command wiring independent of an agent implementation. A nil
// Executor makes the standalone CLI an inspection and task-recording surface;
// it never infers a shell, provider, or autonomous worker from task JSON.
type Options struct {
	Registry localagents.Registry
	Executor localagents.Executor
	Store    func(*cobra.Command) (localagents.Store, error)
	Now      func() time.Time
}

func NewCommand(options Options) *cobra.Command {
	registry := options.Registry
	if len(registry.List()) == 0 {
		registry = localagents.DefaultRegistry()
	}
	storeFor := options.Store
	if storeFor == nil {
		storeFor = defaultStore
	}
	queueFor := func(command *cobra.Command) (*localagents.Queue, error) {
		store, err := storeFor(command)
		if err != nil {
			return nil, err
		}
		return &localagents.Queue{Registry: registry, Store: store, Executor: options.Executor, Now: options.Now}, nil
	}

	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Manage local agent roles and structured work items",
		Long: `Manage Lana's local agent registry and structured task records.

Task input and results are JSON metadata. The standalone CLI records, lists,
and cancellation-requests tasks but does not execute them. Execution requires
an embedding application to provide an explicit local Executor; this command
never treats task input as shell text or configures providers or credentials.`,
	}
	cmd.AddCommand(rolesCommand(registry))
	cmd.AddCommand(describeCommand(registry))
	cmd.AddCommand(submitCommand(queueFor))
	cmd.AddCommand(listCommand(queueFor))
	cmd.AddCommand(showCommand(queueFor))
	cmd.AddCommand(cancelCommand(queueFor))
	cmd.AddCommand(workCommand(queueFor))
	return cmd
}

func rolesCommand(registry localagents.Registry) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "List registered local agent roles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			roles := registry.List()
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), roles)
			}
			for _, role := range roles {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", role.ID, role.Description)
			}
			return nil
		},
		Args: cobra.NoArgs,
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write JSON")
	return cmd
}

func describeCommand(registry localagents.Registry) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "describe ROLE",
		Short: "Describe a registered local agent role",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			role, ok := registry.Describe(args[0])
			if !ok {
				return fmt.Errorf("%w: %s", localagents.ErrRoleNotFound, args[0])
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), role)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n%s\n", role.Name, role.ID, role.Description)
			if len(role.Capabilities) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Capabilities: %s\n", strings.Join(role.Capabilities, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write JSON")
	return cmd
}

func submitCommand(queueFor func(*cobra.Command) (*localagents.Queue, error)) *cobra.Command {
	var id, role, input string
	var metadata, dependencies []string
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Register a structured local agent task",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			values, err := keyValues(metadata)
			if err != nil {
				return err
			}
			queue, err := queueFor(cmd)
			if err != nil {
				return err
			}
			task, err := queue.Register(cmd.Context(), localagents.TaskRequest{ID: id, Role: role, Input: json.RawMessage(input), Metadata: values, DependsOn: dependencies})
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), task)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Stable task ID (lowercase letters, digits, hyphens)")
	cmd.Flags().StringVar(&role, "role", "", "Registered role ID")
	cmd.Flags().StringVar(&input, "input", "", "Task input as a JSON value")
	cmd.Flags().StringArrayVar(&metadata, "metadata", nil, "Task metadata KEY=VALUE (repeatable)")
	cmd.Flags().StringSliceVar(&dependencies, "depends-on", nil, "Completed task IDs required before execution")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("role")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func listCommand(queueFor func(*cobra.Command) (*localagents.Queue, error)) *cobra.Command {
	var status string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local agent tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			queue, err := queueFor(cmd)
			if err != nil {
				return err
			}
			tasks, err := queue.Store.List(cmd.Context())
			if err != nil {
				return err
			}
			if status != "" {
				filtered := tasks[:0]
				for _, task := range tasks {
					if string(task.Status) == status {
						filtered = append(filtered, task)
					}
				}
				tasks = filtered
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), tasks)
			}
			for _, task := range tasks {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", task.ID, task.Status, task.Role, task.CreatedAt.Format(time.RFC3339))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by task status")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write JSON")
	return cmd
}

func showCommand(queueFor func(*cobra.Command) (*localagents.Queue, error)) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "show TASK_ID",
		Short: "Show one local agent task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			queue, err := queueFor(cmd)
			if err != nil {
				return err
			}
			task, err := queue.Store.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), task)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\nrole: %s\nstatus: %s\ninput: %s\n", task.ID, task.Role, task.Status, task.Input)
			if len(task.Result) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "result: %s\n", task.Result)
			}
			if task.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "error: %s\n", task.Error)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write JSON")
	return cmd
}

func cancelCommand(queueFor func(*cobra.Command) (*localagents.Queue, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel TASK_ID",
		Short: "Cancel a queued task or request cancellation of a running task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			queue, err := queueFor(cmd)
			if err != nil {
				return err
			}
			task, err := queue.Cancel(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), task)
		},
	}
}

func workCommand(queueFor func(*cobra.Command) (*localagents.Queue, error)) *cobra.Command {
	var maxConcurrency int
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Process tasks only through an explicitly configured local executor",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			queue, err := queueFor(cmd)
			if err != nil {
				return err
			}
			if queue.Executor == nil {
				return errors.New("agent work is unavailable in the standalone CLI: it only records and inspects tasks; embed Lana with an explicit local Executor to process work")
			}
			return queue.Run(cmd.Context(), maxConcurrency)
		},
	}
	cmd.Flags().IntVar(&maxConcurrency, "max-concurrency", 1, "Maximum concurrent local agent tasks")
	return cmd
}

func defaultStore(command *cobra.Command) (localagents.Store, error) {
	workspace, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	if application, ok := app.FromContext(command.Context()); ok {
		workspace = application.Config().Workspace()
	}
	return localagents.NewFileStore(filepath.Join(workspace, ".lana", "agents", "tasks.json"))
}

func keyValues(entries []string) (map[string]string, error) {
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("metadata must use KEY=VALUE: %q", entry)
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("duplicate metadata key %q", key)
		}
		values[key] = value
	}
	return values, nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
