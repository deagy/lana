// Package goal provides goal management subcommands.
package goal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

const goalsFile = "goals.json"

// Valid goal statuses
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusComplete   = "complete"
)

// Goal represents a managed goal.
type Goal struct {
	ID           string            `json:"id"`
	Objective    string            `json:"objective"`
	Status       string            `json:"status"`
	TokenBudget  *int              `json:"token_budget,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	LastSummary  string            `json:"last_summary,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// goalFile represents the JSON file format for goals.
type goalFile struct {
	Version int    `json:"version"`
	Goals   []Goal `json:"goals"`
}

// NewCommand creates the goal command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goal",
		Short: "Goal management",
		Long: `Goal management with lifecycle tracking.

Subcommands:
  create    Create a new goal
  list      List goals
  show      Show goal details
  update    Update goal status
  delete    Delete a goal
`,
	}
	cmd.AddCommand(goalCreateCommand())
	cmd.AddCommand(goalListCommand())
	cmd.AddCommand(goalShowCommand())
	cmd.AddCommand(goalUpdateCommand())
	cmd.AddCommand(goalDeleteCommand())
	return cmd
}

func goalCreateCommand() *cobra.Command {
	var objective string
	var tokenBudget int
	var setBudget bool
	var deps []string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new goal",
		RunE: func(cmd *cobra.Command, args []string) error {
			if objective == "" {
				return fmt.Errorf("--objective is required")
			}
			if setBudget && tokenBudget <= 0 {
				return fmt.Errorf("--token-budget must be > 0")
			}

			goalsDir := ".lana"
			if err := os.MkdirAll(goalsDir, 0755); err != nil {
				return fmt.Errorf("create goals directory: %w", err)
			}

			goalsPath := filepath.Join(goalsDir, goalsFile)

			// Load or create empty goals
			gf := &goalFile{Version: 1}
			data, err := os.ReadFile(goalsPath)
			if err == nil {
				if err := json.Unmarshal(data, gf); err != nil {
					return fmt.Errorf("parse goals: %w", err)
				}
			}

			id := fmt.Sprintf("goal-%04d", len(gf.Goals)+1)
			now := time.Now().UTC()

			goal := Goal{
				ID:        id,
				Objective: objective,
				Status:    StatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if setBudget {
				goal.TokenBudget = &tokenBudget
			}
			if len(deps) > 0 {
				goal.Dependencies = deps
			}

			gf.Goals = append(gf.Goals, goal)
			if err := saveGoals(goalsPath, gf); err != nil {
				return fmt.Errorf("save goals: %w", err)
			}

			fmt.Printf("Goal created: %s\n", id)
			fmt.Printf("  Objective: %s\n", objective)
			fmt.Printf("  Status:    %s\n", StatusPending)
			if setBudget {
				fmt.Printf("  Budget:    %d tokens\n", tokenBudget)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&objective, "objective", "o", "", "Goal objective text (required)")
	cmd.Flags().IntVar(&tokenBudget, "token-budget", 0, "Optional token budget")
	cmd.Flags().BoolVar(&setBudget, "with-budget", false, "Set token budget")
	cmd.Flags().StringArrayVarP(&deps, "depends", "d", []string{}, "Dependency goal IDs (repeatable)")
	cmd.MarkFlagRequired("objective")
	return cmd
}

func goalListCommand() *cobra.Command {
	var status string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List goals",
		RunE: func(cmd *cobra.Command, args []string) error {
			goalsPath := filepath.Join(".lana", goalsFile)
			data, err := os.ReadFile(goalsPath)
			if err != nil {
				fmt.Println("No goals found.")
				return nil
			}

			gf := &goalFile{Version: 1}
			if err := json.Unmarshal(data, gf); err != nil {
				fmt.Println("No goals found.")
				return nil
			}

			goals := gf.Goals
			if status != "" {
				filtered := make([]Goal, 0)
				for _, g := range goals {
					if g.Status == status {
						filtered = append(filtered, g)
					}
				}
				goals = filtered
			}

			if len(goals) == 0 {
				fmt.Println("No goals found.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(goals, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Goals (%d total):\n", len(goals))
			for _, g := range goals {
				fmt.Printf("  %s [%s] %s", g.ID, g.Status, g.Objective)
				if g.TokenBudget != nil {
					fmt.Printf(" (budget: %d)", *g.TokenBudget)
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func goalShowCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show goal details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			goalsPath := filepath.Join(".lana", goalsFile)
			data, err := os.ReadFile(goalsPath)
			if err != nil {
				return fmt.Errorf("no goals found")
			}

			gf := &goalFile{Version: 1}
			if err := json.Unmarshal(data, gf); err != nil {
				return fmt.Errorf("parse goals: %w", err)
			}

			targetID := args[0]
			for _, g := range gf.Goals {
				if g.ID == targetID {
					if jsonOutput {
						data, _ := json.MarshalIndent(g, "", "  ")
						fmt.Println(string(data))
						return nil
					}

					fmt.Printf("ID:          %s\n", g.ID)
					fmt.Printf("Objective:   %s\n", g.Objective)
					fmt.Printf("Status:      %s\n", g.Status)
					if g.TokenBudget != nil {
						fmt.Printf("Budget:      %d tokens\n", *g.TokenBudget)
					}
					if len(g.Dependencies) > 0 {
						fmt.Printf("Dependencies: %s\n", g.Dependencies)
					}
					if g.Metadata != nil {
						fmt.Printf("Metadata:\n")
						for k, v := range g.Metadata {
							fmt.Printf("  %s: %s\n", k, v)
						}
					}
					fmt.Printf("Created:     %s\n", g.CreatedAt.Format(time.RFC3339))
					fmt.Printf("Updated:     %s\n", g.UpdatedAt.Format(time.RFC3339))
					if g.LastSummary != "" {
						fmt.Printf("Summary:     %s\n", g.LastSummary)
					}
					return nil
				}
			}
			return fmt.Errorf("goal not found: %s", targetID)
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func goalUpdateCommand() *cobra.Command {
	var newStatus string
	var summary string
	var budget int
	var setBudget bool
	var addDep, removeDep []string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update goal status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if newStatus == "" && !setBudget && len(addDep) == 0 && len(removeDep) == 0 {
				return fmt.Errorf("at least one flag is required")
			}

			goalsPath := filepath.Join(".lana", goalsFile)
			data, err := os.ReadFile(goalsPath)
			if err != nil {
				return fmt.Errorf("no goals found")
			}

			gf := &goalFile{Version: 1}
			if err := json.Unmarshal(data, gf); err != nil {
				return fmt.Errorf("parse goals: %w", err)
			}

			targetID := args[0]
			targetIdx := -1
			for i, g := range gf.Goals {
				if g.ID == targetID {
					targetIdx = i
					break
				}
			}

			if targetIdx == -1 {
				return fmt.Errorf("goal not found: %s", targetID)
			}

			if newStatus != "" {
				gf.Goals[targetIdx].Status = newStatus
			}
			if summary != "" {
				gf.Goals[targetIdx].LastSummary = summary
			}
			if setBudget {
				if budget <= 0 {
					return fmt.Errorf("--token-budget must be > 0")
				}
				gf.Goals[targetIdx].TokenBudget = &budget
			}

			// Handle dependencies
			for _, dep := range addDep {
				found := false
				for _, d := range gf.Goals[targetIdx].Dependencies {
					if d == dep {
						found = true
						break
					}
				}
				if !found {
					gf.Goals[targetIdx].Dependencies = append(gf.Goals[targetIdx].Dependencies, dep)
				}
			}
			for _, dep := range removeDep {
				newDeps := make([]string, 0)
				for _, d := range gf.Goals[targetIdx].Dependencies {
					if d != dep {
						newDeps = append(newDeps, d)
					}
				}
				gf.Goals[targetIdx].Dependencies = newDeps
			}

			gf.Goals[targetIdx].UpdatedAt = time.Now().UTC()

			if err := saveGoals(goalsPath, gf); err != nil {
				return fmt.Errorf("save goals: %w", err)
			}

			fmt.Printf("Goal %s updated\n", targetID)
			if newStatus != "" {
				fmt.Printf("  Status: %s\n", newStatus)
			}
			if summary != "" {
				fmt.Printf("  Summary: %s\n", summary)
			}
			if setBudget {
				fmt.Printf("  Budget: %d tokens\n", budget)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&newStatus, "status", "s", "", "New status")
	cmd.Flags().StringVarP(&summary, "summary", "S", "", "Summary text")
	cmd.Flags().IntVar(&budget, "token-budget", 0, "New token budget")
	cmd.Flags().BoolVar(&setBudget, "with-budget", false, "Set token budget")
	cmd.Flags().StringArrayVarP(&addDep, "add-dep", "a", []string{}, "Add dependency goal ID")
	cmd.Flags().StringArrayVarP(&removeDep, "remove-dep", "r", []string{}, "Remove dependency goal ID")
	return cmd
}

func goalDeleteCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a goal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID := args[0]

			goalsPath := filepath.Join(".lana", goalsFile)
			data, err := os.ReadFile(goalsPath)
			if err != nil {
				return fmt.Errorf("no goals found")
			}

			gf := &goalFile{Version: 1}
			if err := json.Unmarshal(data, gf); err != nil {
				return fmt.Errorf("parse goals: %w", err)
			}

			found := false
			newGoals := make([]Goal, 0)
			for _, g := range gf.Goals {
				if g.ID == targetID {
					found = true
					if !force {
						return fmt.Errorf("goal %s found. Use --force to delete", targetID)
					}
				} else {
					newGoals = append(newGoals, g)
				}
			}

			if !found {
				return fmt.Errorf("goal not found: %s", targetID)
			}

			gf.Goals = newGoals
			if err := saveGoals(goalsPath, gf); err != nil {
				return fmt.Errorf("save goals: %w", err)
			}

			fmt.Printf("Deleted goal: %s\n", targetID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func saveGoals(path string, gf *goalFile) error {
	gf.Version = 1
	data, err := json.MarshalIndent(gf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
