// Package plan provides plan management subcommands.
package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

const plansFile = "plans.json"

// Valid step statuses
const (
	StepPending    = "pending"
	StepInProgress = "in_progress"
	StepCompleted  = "completed"
)

// Plan represents a managed plan.
type Plan struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Steps     []Step `json:"steps"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Status    string `json:"status"`
}

// Step represents a plan step.
type Step struct {
	Index       int    `json:"index"`
	Text        string `json:"text"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Assignee    string `json:"assignee,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// planFile represents the JSON file format for plans.
type planFile struct {
	Version int    `json:"version"`
	Plans   []Plan `json:"plans"`
}

// NewCommand creates the plan command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan management",
	}
	cmd.AddCommand(planCreateCommand())
	cmd.AddCommand(planListCommand())
	cmd.AddCommand(planShowCommand())
	cmd.AddCommand(planUpdateCommand())
	return cmd
}

func planCreateCommand() *cobra.Command {
	var steps []string
	var title string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			plansDir := ".lana"
			if err := os.MkdirAll(plansDir, 0755); err != nil {
				return fmt.Errorf("create plans directory: %w", err)
			}

			plansPath := filepath.Join(plansDir, plansFile)

			// Load or create empty plans
			pf := &planFile{Version: 1}
			data, err := os.ReadFile(plansPath)
			if err == nil {
				if err := json.Unmarshal(data, pf); err != nil {
					return fmt.Errorf("parse plans: %w", err)
				}
			}

			id := fmt.Sprintf("plan-%04d", len(pf.Plans)+1)
			now := time.Now().UTC().Format(time.RFC3339)

			planSteps := make([]Step, 0, len(steps))
			for i, stepText := range steps {
				status := StepPending
				if i == 0 {
					status = StepInProgress
				}
				step := Step{
					Index:  i + 1,
					Text:   stepText,
					Status: status,
				}
				if status == StepInProgress {
					step.StartedAt = now
				}
				planSteps = append(planSteps, step)
			}

			plan := Plan{
				ID:        id,
				Title:     title,
				Steps:     planSteps,
				CreatedAt: now,
				UpdatedAt: now,
				Status:    StepPending,
			}
			if len(planSteps) > 0 && planSteps[0].Status == StepInProgress {
				plan.Status = StepInProgress
			}

			pf.Plans = append(pf.Plans, plan)
			if err := savePlans(plansPath, pf); err != nil {
				return fmt.Errorf("save plans: %w", err)
			}

			fmt.Printf("Plan created: %s\n", id)
			if title != "" {
				fmt.Printf("  Title:   %s\n", title)
			}
			fmt.Printf("  Steps:   %d\n", len(planSteps))
			fmt.Printf("  Status:  %s\n", plan.Status)
			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&steps, "step", "s", []string{}, "Plan step text (repeatable)")
	cmd.Flags().StringVarP(&title, "title", "t", "", "Plan title")
	return cmd
}

func planListCommand() *cobra.Command {
	var status string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			plansPath := filepath.Join(".lana", plansFile)
			data, err := os.ReadFile(plansPath)
			if err != nil {
				fmt.Println("No plans found.")
				return nil
			}

			pf := &planFile{Version: 1}
			if err := json.Unmarshal(data, pf); err != nil {
				fmt.Println("No plans found.")
				return nil
			}

			plans := pf.Plans
			if status != "" {
				filtered := make([]Plan, 0)
				for _, p := range plans {
					if p.Status == status {
						filtered = append(filtered, p)
					}
				}
				plans = filtered
			}

			if len(plans) == 0 {
				fmt.Println("No plans found.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(plans, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Plans (%d total):\n", len(plans))
			for _, p := range plans {
				completed := 0
				total := len(p.Steps)
				for _, s := range p.Steps {
					if s.Status == StepCompleted {
						completed++
					}
				}
				fmt.Printf("  %s [%s] %s (%d/%d steps)\n", p.ID, p.Status, p.Title, completed, total)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func planShowCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show plan details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plansPath := filepath.Join(".lana", plansFile)
			data, err := os.ReadFile(plansPath)
			if err != nil {
				return fmt.Errorf("no plans found")
			}

			pf := &planFile{Version: 1}
			if err := json.Unmarshal(data, pf); err != nil {
				return fmt.Errorf("parse plans: %w", err)
			}

			targetID := args[0]
			for _, p := range pf.Plans {
				if p.ID == targetID {
					if jsonOutput {
						data, _ := json.MarshalIndent(p, "", "  ")
						fmt.Println(string(data))
						return nil
					}

					completed := 0
					total := len(p.Steps)
					for _, s := range p.Steps {
						if s.Status == StepCompleted {
							completed++
						}
					}

					fmt.Printf("Plan:    %s\n", p.ID)
					if p.Title != "" {
						fmt.Printf("Title:   %s\n", p.Title)
					}
					fmt.Printf("Status:  %s (%d/%d steps)\n", p.Status, completed, total)
					if total > 0 {
						pct := float64(completed) / float64(total) * 100
						fmt.Printf("Progress: %.1f%%\n", pct)
					}
					fmt.Printf("Created: %s\n", p.CreatedAt)
					fmt.Printf("Updated: %s\n", p.UpdatedAt)
					fmt.Printf("\nSteps (%d):\n", len(p.Steps))
					for _, s := range p.Steps {
						symbol := "o"
						switch s.Status {
						case StepCompleted:
							symbol = "x"
						case StepInProgress:
							symbol = "/"
						}
						fmt.Printf("  %s [%s] %s\n", symbol, s.Status, s.Text)
						if s.Assignee != "" {
							fmt.Printf("      Assignee: %s\n", s.Assignee)
						}
						if s.Notes != "" {
							fmt.Printf("      Notes: %s\n", s.Notes)
						}
					}
					return nil
				}
			}
			return fmt.Errorf("plan not found: %s", targetID)
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func planUpdateCommand() *cobra.Command {
	var status string
	var assignee string
	var notes string

	cmd := &cobra.Command{
		Use:   "update <plan-id> <step-index>",
		Short: "Update plan step status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if status == "" && assignee == "" && notes == "" {
				return fmt.Errorf("at least one flag is required: --status, --assignee, or --notes")
			}

			plansPath := filepath.Join(".lana", plansFile)
			data, err := os.ReadFile(plansPath)
			if err != nil {
				return fmt.Errorf("no plans found")
			}

			pf := &planFile{Version: 1}
			if err := json.Unmarshal(data, pf); err != nil {
				return fmt.Errorf("parse plans: %w", err)
			}

			targetID := args[0]
			stepIndex, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid step index: %s", args[1])
			}
			stepIndex--

			now := time.Now().UTC().Format(time.RFC3339)
			found := false
			for i, p := range pf.Plans {
				if p.ID == targetID {
					found = true
					if stepIndex < 0 || stepIndex >= len(p.Steps) {
						return fmt.Errorf("step index %d out of range", stepIndex)
					}

					if status != "" {
						if status == StepInProgress {
							for j := range pf.Plans[i].Steps {
								if pf.Plans[i].Steps[j].Status == StepInProgress && j != stepIndex {
									pf.Plans[i].Steps[j].Status = StepPending
								}
							}
							pf.Plans[i].Steps[stepIndex].StartedAt = now
						}
						if status == StepCompleted {
							pf.Plans[i].Steps[stepIndex].CompletedAt = now
						}
						pf.Plans[i].Steps[stepIndex].Status = status
					}
					if assignee != "" {
						pf.Plans[i].Steps[stepIndex].Assignee = assignee
					}
					if notes != "" {
						pf.Plans[i].Steps[stepIndex].Notes = notes
					}

					pf.Plans[i].UpdatedAt = now

					allCompleted := true
					for _, s := range pf.Plans[i].Steps {
						if s.Status != StepCompleted {
							allCompleted = false
							break
						}
					}
					if allCompleted {
						pf.Plans[i].Status = StepCompleted
					} else {
						hasInProgress := false
						for _, s := range pf.Plans[i].Steps {
							if s.Status == StepInProgress {
								hasInProgress = true
								break
							}
						}
						if hasInProgress {
							pf.Plans[i].Status = StepInProgress
						} else {
							pf.Plans[i].Status = StepPending
						}
					}

					if err := savePlans(plansPath, pf); err != nil {
						return fmt.Errorf("save plans: %w", err)
					}

					fmt.Printf("Plan %s, step %d updated\n", targetID, stepIndex+1)
					if status != "" {
						fmt.Printf("  Status: %s\n", status)
					}
					if assignee != "" {
						fmt.Printf("  Assignee: %s\n", assignee)
					}
					if notes != "" {
						fmt.Printf("  Notes: %s\n", notes)
					}
					return nil
				}
			}

			if !found {
				return fmt.Errorf("plan not found: %s", targetID)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Step status: pending, in_progress, completed")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Step assignee")
	cmd.Flags().StringVarP(&notes, "notes", "n", "", "Step notes")
	return cmd
}

func savePlans(path string, pf *planFile) error {
	pf.Version = 1
	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
