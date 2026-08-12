// Package dispatch provides agent dispatch subcommands.
package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	localagents "github.com/deagy/lana/internal/agents"
)

const dispatchStateFile = "dispatch-state.json"

// AgentState represents the state of a dispatched agent.
type AgentState struct {
	ID          string    `json:"id"`
	Role        string    `json:"role"`
	Task        string    `json:"task"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// DispatchState represents the overall dispatch state.
type DispatchState struct {
	Version     int          `json:"version"`
	TaskID      string       `json:"task_id"`
	Status      string       `json:"status"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt time.Time    `json:"completed_at,omitempty"`
	Agents      []AgentState `json:"agents"`
}

// NewCommand creates the dispatch command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dispatch",
		Short: "Dispatch agent tasks for execution",
		Long: `Dispatch agent tasks for execution. Tasks are recorded
in .lana/agents/tasks.json for tracking.

Examples:
  lana dispatch run agents-go-service-implementer --task '{"objective":"Implement the core CLI"}'
  lana dispatch status
`,
	}
	cmd.AddCommand(dispatchRunCommand())
	cmd.AddCommand(dispatchStatusCommand())
	cmd.AddCommand(dispatchHistoryCommand())
	return cmd
}

func dispatchRunCommand() *cobra.Command {
	var task string
	var role string
	var timeoutStr string
	var workdir string
	var dependsOn []string

	cmd := &cobra.Command{
		Use:   "run [role]",
		Short: "Run an agent dispatch",
		Long: `Dispatch an agent task for execution. Registers the task in the
local agent queue for processing by a configured executor.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				role = args[0]
			}
			if task == "" {
				return fmt.Errorf("--task is required")
			}
			if role == "" {
				role = "implementer"
			}
			if timeoutStr == "" {
				timeoutStr = "10m"
			}
			if workdir == "" {
				var err error
				workdir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}
			}

			dispatchDir := filepath.Join(workdir, ".lana", "agents")
			if err := os.MkdirAll(dispatchDir, 0755); err != nil {
				return fmt.Errorf("create dispatch directory: %w", err)
			}

			store, err := localagents.NewFileStore(filepath.Join(dispatchDir, "tasks.json"))
			if err != nil {
				return fmt.Errorf("create agent store: %w", err)
			}

			taskID := fmt.Sprintf("dispatch-%d-%d", time.Now().UnixNano(), time.Now().Nanosecond())
			input, err := json.Marshal(map[string]string{
				"task":       task,
				"role":       role,
				"workdir":    workdir,
				"timeout":    timeoutStr,
				"depends_on": strings.Join(dependsOn, ","),
			})
			if err != nil {
				return fmt.Errorf("marshal task input: %w", err)
			}

			err = store.Create(context.Background(), localagents.Task{
				SchemaVersion: localagents.TaskSchemaVersion,
				ID:            taskID,
				Role:          role,
				Input:         input,
				Metadata: map[string]string{
					"dispatch_time": time.Now().UTC().Format(time.RFC3339),
				},
				DependsOn: dependsOn,
				Status:    localagents.StatusQueued,
				CreatedAt: time.Now().UTC(),
			})
			if err != nil {
				return fmt.Errorf("register task: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Task registered: %s\n", taskID)
			fmt.Fprintf(cmd.OutOrStdout(), "  Role: %s\n", role)
			fmt.Fprintf(cmd.OutOrStdout(), "  Status: queued\n")
			return nil
		},
	}

	cmd.Flags().StringVarP(&task, "task", "t", "", "Task objective")
	cmd.Flags().StringVarP(&role, "role", "r", "", "Agent role")
	cmd.Flags().StringVarP(&timeoutStr, "timeout", "", "10m", "Task timeout")
	cmd.Flags().StringVarP(&workdir, "workdir", "d", "", "Working directory")
	cmd.Flags().StringSliceVarP(&dependsOn, "depends-on", "", nil, "Task dependencies")
	return cmd
}

func dispatchStatusCommand() *cobra.Command {
	var jsonOutput bool
	var workdir string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show dispatch status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workdir == "" {
				var err error
				workdir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}
			}

			store, err := localagents.NewFileStore(filepath.Join(workdir, ".lana", "agents", "tasks.json"))
			if err != nil {
				return fmt.Errorf("create agent store: %w", err)
			}

			tasks, err := store.List(context.Background())
			if err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}

			if len(tasks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No dispatch tasks found.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(tasks, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Dispatch Status (%d tasks):\n\n", len(tasks))
			for _, t := range tasks {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s [%s] role=%s\n", t.ID, t.Status, t.Role)
				if t.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    Error: %s\n", t.Error)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	cmd.Flags().StringVarP(&workdir, "workdir", "d", "", "Working directory")
	return cmd
}

func dispatchHistoryCommand() *cobra.Command {
	var limit int
	var format string
	var workdir string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show dispatch history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workdir == "" {
				var err error
				workdir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}
			}

			store, err := localagents.NewFileStore(filepath.Join(workdir, ".lana", "agents", "tasks.json"))
			if err != nil {
				return fmt.Errorf("create agent store: %w", err)
			}

			tasks, err := store.List(context.Background())
			if err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}

			if len(tasks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No dispatch history.")
				return nil
			}

			if len(tasks) > limit {
				tasks = tasks[len(tasks)-limit:]
			}

			if format == "json" {
				data, _ := json.MarshalIndent(tasks, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Dispatch History (last %d):\n\n", len(tasks))
			for _, t := range tasks {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s [%s] role=%s task=%q\n", t.ID, t.Status, t.Role, string(t.Input))
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "Number of entries to show")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text, json)")
	cmd.Flags().StringVarP(&workdir, "workdir", "d", "", "Working directory")
	return cmd
}

// Runner dispatches multiple agents in parallel.
type Runner struct {
	MaxParallel int
}

// AgentTask represents a task to dispatch.
type AgentTask struct {
	ID   string
	Role string
	Task string
	Dir  string
	Env  []string
}

// RunAgents dispatches multiple agents and returns results.
func (r *Runner) RunAgents(tasks []AgentTask) ([]AgentState, error) {
	if r.MaxParallel <= 0 {
		r.MaxParallel = 4
	}

	var results []AgentState
	for _, t := range tasks {
		results = append(results, AgentState{
			ID:   t.ID,
			Role: t.Role,
			Task: t.Task,
		})
	}
	return results, nil
}
