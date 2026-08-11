// Package dispatch provides agent dispatch subcommands.
package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
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
in .lana/dispatch-state.json for tracking.

Examples:
  lana dispatch run agents-go-service-implementer --task "Implement the core CLI"
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
	var envVars []string

	cmd := &cobra.Command{
		Use:   "run [role]",
		Short: "Run an agent dispatch",
		Long: `Dispatch an agent task for execution. Spawns a subprocess
with the specified role as a command and captures output.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				role = args[0]
			}
			if task == "" {
				return fmt.Errorf("--task is required")
			}
			if role == "" {
				role = "unknown-agent"
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

			dispatchDir := ".lana"
			if err := os.MkdirAll(dispatchDir, 0755); err != nil {
				return fmt.Errorf("create dispatch directory: %w", err)
			}

			statePath := filepath.Join(dispatchDir, dispatchStateFile)
			state := &DispatchState{Version: 1, Agents: []AgentState{}}
			data, err := os.ReadFile(statePath)
			if err == nil {
				if err := json.Unmarshal(data, state); err != nil {
					return fmt.Errorf("parse state: %w", err)
				}
			}

			startTime := time.Now().UTC()
			state.TaskID = role
			state.Status = "running"
			state.StartedAt = startTime

			agentID := fmt.Sprintf("agent-%03d", len(state.Agents)+1)
			agent := AgentState{
				ID:        agentID,
				Role:      role,
				Task:      task,
				Status:    "running",
				StartedAt: startTime,
			}

			t, err := time.ParseDuration(timeoutStr)
			if err != nil {
				return fmt.Errorf("invalid timeout: %w", err)
			}

			var cmdEnv []string
			cmdEnv = append(cmdEnv, os.Environ()...)
			for _, ev := range envVars {
				parts := strings.SplitN(ev, "=", 2)
				if len(parts) == 2 {
					key := parts[0]
					if isSecretKey(key) {
						cmdEnv = append(cmdEnv, key+"=***REDACTED***")
					} else {
						cmdEnv = append(cmdEnv, ev)
					}
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), t)
			defer cancel()

			execCmd := exec.CommandContext(ctx, "sh", "-c", task)
			execCmd.Dir = workdir
			execCmd.Env = cmdEnv

			var stdout, stderr bytes.Buffer
			execCmd.Stdout = &stdout
			execCmd.Stderr = &stderr

			start := time.Now()
			err = execCmd.Run()
			elapsed := time.Since(start)

			output := stdout.String()
			if stderr.Len() > 0 {
				output += stderr.String()
			}

			agent.CompletedAt = time.Now().UTC()
			agent.Output = output
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					agent.Status = "timeout"
					agent.Error = fmt.Sprintf("timed out after %s", elapsed)
					agent.ExitCode = 124
				} else {
					agent.Status = "failed"
					agent.Error = err.Error()
					if exitErr, ok := err.(*exec.ExitError); ok {
						agent.ExitCode = exitErr.ExitCode()
					}
				}
			} else {
				agent.Status = "completed"
				agent.ExitCode = 0
			}

			state.Agents = append(state.Agents, agent)

			if state.Status == "running" && agent.Status == "completed" && len(state.Agents) == 1 {
				state.Status = "completed"
				state.CompletedAt = agent.CompletedAt
			}

			if err := saveState(statePath, state); err != nil {
				return fmt.Errorf("save state: %w", err)
			}

			fmt.Printf("Dispatched: %s (role=%s, task=%q)\n", agentID, role, task)
			fmt.Printf("  Status:    %s\n", agent.Status)
			fmt.Printf("  Started:   %s\n", agent.StartedAt.Format(time.RFC3339))
			fmt.Printf("  Elapsed:   %s\n", elapsed.Round(time.Millisecond))
			if agent.Status == "completed" {
				fmt.Printf("  Exit code: 0\n")
			} else {
				fmt.Printf("  Error:     %s\n", agent.Error)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&task, "task", "t", "", "Task description or command (required)")
	cmd.Flags().StringVarP(&role, "role", "r", "", "Agent role name")
	cmd.Flags().StringVarP(&timeoutStr, "timeout", "T", "10m", "Execution timeout")
	cmd.Flags().StringVarP(&workdir, "workdir", "d", "", "Working directory")
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", []string{}, "Environment variable (KEY=VALUE)")
	cmd.MarkFlagRequired("task")
	return cmd
}

func dispatchStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show dispatch status",
		RunE: func(cmd *cobra.Command, args []string) error {
			statePath := filepath.Join(".lana", dispatchStateFile)
			state, err := loadState(statePath)
			if err != nil {
				fmt.Println("No dispatch state found.")
				return nil
			}

			fmt.Println("Dispatch State:")
			fmt.Printf("  Task ID:    %s\n", state.TaskID)
			fmt.Printf("  Status:     %s\n", state.Status)
			fmt.Printf("  Started:    %s\n", state.StartedAt.Format(time.RFC3339))
			if !state.CompletedAt.IsZero() {
				fmt.Printf("  Completed:  %s\n", state.CompletedAt.Format(time.RFC3339))
			}
			fmt.Printf("  Agents:     %d\n", len(state.Agents))
			fmt.Println()

			for _, a := range state.Agents {
				fmt.Printf("  [%s] %s\n", a.Status, a.ID)
				fmt.Printf("    Role: %s\n", a.Role)
				fmt.Printf("    Task: %s\n", a.Task)
				fmt.Printf("    Started:   %s\n", a.StartedAt.Format(time.RFC3339))
				if !a.CompletedAt.IsZero() {
					fmt.Printf("    Completed: %s\n", a.CompletedAt.Format(time.RFC3339))
				}
				if a.ExitCode != 0 {
					fmt.Printf("    Exit Code: %d\n", a.ExitCode)
				}
				if a.Error != "" {
					fmt.Printf("    Error:     %s\n", a.Error)
				}
				fmt.Println()
			}
			return nil
		},
	}
}

func dispatchHistoryCommand() *cobra.Command {
	var limit int
	var format string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show dispatch history",
		RunE: func(cmd *cobra.Command, args []string) error {
			statePath := filepath.Join(".lana", dispatchStateFile)
			state, err := loadState(statePath)
			if err != nil {
				fmt.Println("No dispatch history found.")
				return nil
			}

			if limit <= 0 {
				limit = 10
			}

			agents := state.Agents
			if len(agents) > limit {
				agents = agents[len(agents)-limit:]
			}

			if format == "json" {
				data, _ := json.MarshalIndent(DispatchState{Version: 1, Agents: agents}, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(agents) == 0 {
				fmt.Println("No dispatch history.")
				return nil
			}

			fmt.Printf("Dispatch History (last %d):\n\n", len(agents))
			for _, a := range agents {
				fmt.Printf("  %s [%s] role=%s task=%q\n", a.ID, a.Status, a.Role, a.Task)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "Number of entries to show")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text, json)")
	return cmd
}

func loadState(path string) (*DispatchState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no state found")
	}

	var state DispatchState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &state, nil
}

func saveState(path string, state *DispatchState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func isSecretKey(key string) bool {
	lower := strings.ToLower(key)
	secretPatterns := []string{"key", "token", "secret", "password", "credential", "api_key", "apikey"}
	for _, pat := range secretPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
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
	var mu sync.Mutex
	sem := make(chan struct{}, r.MaxParallel)
	var wg sync.WaitGroup

	for _, t := range tasks {
		wg.Add(1)
		sem <- struct{}{}

		go func(task AgentTask) {
			defer wg.Done()
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			execCmd := exec.CommandContext(ctx, "sh", "-c", task.Task)
			execCmd.Dir = task.Dir
			execCmd.Env = append(os.Environ(), task.Env...)

			var stdout, stderr bytes.Buffer
			execCmd.Stdout = &stdout
			execCmd.Stderr = &stderr

			start := time.Now()
			err := execCmd.Run()
			elapsed := time.Since(start)

			var state AgentState
			state.ID = task.ID
			state.Role = task.Role
			state.Task = task.Task
			state.StartedAt = start.UTC()

			output := stdout.String()
			if stderr.Len() > 0 {
				output += stderr.String()
			}
			state.Output = output

			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					state.Status = "timeout"
					state.Error = fmt.Sprintf("timed out after %s", elapsed)
					state.ExitCode = 124
				} else {
					state.Status = "failed"
					state.Error = err.Error()
					if exitErr, ok := err.(*exec.ExitError); ok {
						state.ExitCode = exitErr.ExitCode()
					}
				}
			} else {
				state.Status = "completed"
				state.ExitCode = 0
			}

			state.CompletedAt = time.Now().UTC()
			mu.Lock()
			results = append(results, state)
			mu.Unlock()
		}(t)
	}

	wg.Wait()
	return results, nil
}
