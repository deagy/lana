// Package sdcl provides Agentic SDLC management subcommands.
package sdcl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const runsDir = ".agentic-sdlc/runs"

// NewCommand creates the sdlc command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdlc",
		Short: "Inspect Agentic SDLC run records",
		Long: `Inspect Agentic SDLC run records.

The product CLI is read-only for lifecycle data. Creating runs, writing plans
or records, and changing lifecycle-gate state require the Agentic SDLC
workflow and its applicable authority; they are not product CLI operations.`,
	}
	cmd.AddCommand(sdclStatusCommand())
	cmd.AddCommand(sdclListRunsCommand())
	cmd.AddCommand(sdclShowRunCommand())
	cmd.AddCommand(sdclReadPlanCommand())
	cmd.AddCommand(sdclReadRecordCommand())
	return cmd
}

func sdclStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show lifecycle gate status",
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir := findRunDir()
			if runDir == "" {
				fmt.Println("No SDLC run found. Initialize with: lana sdlc init")
				return nil
			}

			recordPath := filepath.Join(runDir, "run-record.json")
			data, err := os.ReadFile(recordPath)
			if err != nil {
				return fmt.Errorf("read run record: %w", err)
			}

			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parse run record: %w", err)
			}

			phase, _ := record["current_lifecycle_phase"].(string)
			taskID, _ := record["task_id"].(string)
			fmt.Printf("SDLC Run: %s\n", taskID)
			fmt.Printf("SDLC Phase: %s\n\n", phase)

			if gates, ok := record["lifecycle_gates"].([]interface{}); ok {
				for _, gI := range gates {
					gate, ok := gI.(map[string]interface{})
					if !ok {
						continue
					}
					id, _ := gate["gate_id"].(string)
					status, _ := gate["status"].(string)
					name, _ := gate["name"].(string)
					fmt.Printf("  %s (%s) [%s]\n", id, name, status)
				}
			}
			return nil
		},
	}
}

func sdclListRunsCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list-runs",
		Short: "List all runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := os.ReadDir(runsDir)
			if err != nil {
				fmt.Println("No SDLC runs found.")
				return nil
			}

			type runInfo struct {
				Name string
				Time time.Time
			}
			var runs []runInfo

			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				recordPath := filepath.Join(runsDir, e.Name(), "run-record.json")
				info, err := os.Stat(recordPath)
				if err != nil {
					continue
				}
				runs = append(runs, runInfo{Name: e.Name(), Time: info.ModTime()})
			}

			sort.Slice(runs, func(i, j int) bool {
				return runs[i].Time.After(runs[j].Time)
			})

			if len(runs) == 0 {
				fmt.Println("No SDLC runs found.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(runs, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("SDLC Runs (%d):\n\n", len(runs))
			for _, r := range runs {
				fmt.Printf("  %s (modified: %s)\n", r.Name, r.Time.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func sdclShowRunCommand() *cobra.Command {
	var taskID string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show-run <task-id>",
		Short: "Show run details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID = args[0]
			recordPath := filepath.Join(runsDir, taskID, "run-record.json")

			data, err := os.ReadFile(recordPath)
			if err != nil {
				return fmt.Errorf("read run record: %w", err)
			}

			if jsonOutput {
				fmt.Println(string(data))
				return nil
			}

			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parse run record: %w", err)
			}

			taskIDVal, _ := record["task_id"].(string)
			phase, _ := record["current_lifecycle_phase"].(string)
			disposition, _ := record["disposition"].(string)

			fmt.Printf("Run: %s\n", taskIDVal)
			fmt.Printf("Phase:   %s\n", phase)
			fmt.Printf("Disposition: %s\n", disposition)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func sdclInitCommand() *cobra.Command {
	var taskID string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize SDLC for current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				taskID = fmt.Sprintf("run-%s", time.Now().Format("20060102-150405"))
			}

			runDir := filepath.Join(runsDir, taskID)
			if err := os.MkdirAll(runDir, 0755); err != nil {
				return fmt.Errorf("create run directory: %w", err)
			}

			record := map[string]interface{}{
				"version":                 2,
				"task_id":                 taskID,
				"recorded_at":             time.Now().UTC().Format(time.RFC3339),
				"current_lifecycle_phase": "intent",
				"disposition":             "pending",
				"lifecycle_gates": []map[string]interface{}{
					{"gate_id": "G1", "name": "Intent", "status": "pending"},
					{"gate_id": "G2", "name": "Requirements", "status": "pending"},
					{"gate_id": "G3", "name": "Architecture", "status": "pending"},
					{"gate_id": "G4", "name": "Governance", "status": "pending"},
					{"gate_id": "G5", "name": "Security", "status": "pending"},
					{"gate_id": "G6", "name": "Design Review", "status": "pending"},
					{"gate_id": "G7", "name": "Implementation", "status": "pending"},
					{"gate_id": "G8", "name": "Testing", "status": "pending"},
					{"gate_id": "G9", "name": "Release", "status": "pending"},
					{"gate_id": "G10", "name": "Operations", "status": "pending"},
				},
			}

			recordData, _ := json.MarshalIndent(record, "", "  ")
			if err := os.WriteFile(filepath.Join(runDir, "run-record.json"), recordData, 0644); err != nil {
				return fmt.Errorf("write run record: %w", err)
			}

			plan := map[string]interface{}{
				"schema_version": 5,
				"task_id":        taskID,
				"status":         "pending",
				"dispatch_plan":  []interface{}{},
			}
			planData, _ := json.MarshalIndent(plan, "", "  ")
			if err := os.WriteFile(filepath.Join(runDir, "dispatch-plan.json"), planData, 0644); err != nil {
				return fmt.Errorf("write dispatch plan: %w", err)
			}

			fmt.Printf("SDLC initialized: %s\n", taskID)
			fmt.Printf("Run directory: %s\n", runDir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&taskID, "task-id", "t", "", "Task ID")
	return cmd
}

func sdclReadPlanCommand() *cobra.Command {
	var taskID string

	cmd := &cobra.Command{
		Use:   "read-plan",
		Short: "Read dispatch plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				taskID = defaultTaskID()
			}
			planPath := filepath.Join(runsDir, taskID, "dispatch-plan.json")
			data, err := os.ReadFile(planPath)
			if err != nil {
				return fmt.Errorf("read dispatch plan: %w", err)
			}
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().StringVarP(&taskID, "task", "t", "", "Task ID")
	return cmd
}

func sdclWritePlanCommand() *cobra.Command {
	var taskID, planFile string

	cmd := &cobra.Command{
		Use:   "write-plan",
		Short: "Write dispatch plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				taskID = defaultTaskID()
			}
			if taskID == "" {
				return fmt.Errorf("no task ID specified and none found")
			}

			var data []byte
			if planFile == "-" || planFile == "" {
				dec := json.NewDecoder(os.Stdin)
				var plan map[string]interface{}
				if err := dec.Decode(&plan); err != nil {
					return fmt.Errorf("parse plan from stdin: %w", err)
				}
				data, _ = json.MarshalIndent(plan, "", "  ")
			} else {
				var err error
				data, err = os.ReadFile(planFile)
				if err != nil {
					return fmt.Errorf("read plan file: %w", err)
				}
			}

			var plan map[string]interface{}
			if err := json.Unmarshal(data, &plan); err != nil {
				return fmt.Errorf("parse dispatch plan: %w", err)
			}

			planPath := filepath.Join(runsDir, taskID)
			if err := os.MkdirAll(planPath, 0755); err != nil {
				return fmt.Errorf("create run directory: %w", err)
			}
			if err := os.WriteFile(filepath.Join(planPath, "dispatch-plan.json"), data, 0644); err != nil {
				return fmt.Errorf("write dispatch plan: %w", err)
			}
			fmt.Println("Dispatch plan written.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&taskID, "task", "t", "", "Task ID")
	cmd.Flags().StringVarP(&planFile, "file", "f", "", "Plan file path")
	return cmd
}

func sdclReadRecordCommand() *cobra.Command {
	var taskID string

	cmd := &cobra.Command{
		Use:   "read-record",
		Short: "Read run record",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				taskID = defaultTaskID()
			}
			recordPath := filepath.Join(runsDir, taskID, "run-record.json")
			data, err := os.ReadFile(recordPath)
			if err != nil {
				return fmt.Errorf("read run record: %w", err)
			}
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().StringVarP(&taskID, "task", "t", "", "Task ID")
	return cmd
}

func sdclWriteRecordCommand() *cobra.Command {
	var taskID string

	cmd := &cobra.Command{
		Use:   "write-record",
		Short: "Write run record",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				taskID = defaultTaskID()
			}
			if taskID == "" {
				return fmt.Errorf("no task ID specified and none found")
			}

			dec := json.NewDecoder(os.Stdin)
			var record map[string]interface{}
			if err := dec.Decode(&record); err != nil {
				return fmt.Errorf("parse run record from stdin: %w", err)
			}

			recordPath := filepath.Join(runsDir, taskID)
			recordData, _ := json.MarshalIndent(record, "", "  ")
			if err := os.MkdirAll(recordPath, 0755); err != nil {
				return fmt.Errorf("create run directory: %w", err)
			}
			if err := os.WriteFile(filepath.Join(recordPath, "run-record.json"), recordData, 0644); err != nil {
				return fmt.Errorf("write run record: %w", err)
			}
			fmt.Println("Run record written.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&taskID, "task", "t", "", "Task ID")
	return cmd
}

func sdclGateReviewCommand() *cobra.Command {
	var gate string

	cmd := &cobra.Command{
		Use:   "review-gate <gate-id>",
		Short: "Review a specific gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gate = args[0]
			runDir := findRunDir()
			if runDir == "" {
				return fmt.Errorf("no SDLC run found")
			}

			recordPath := filepath.Join(runDir, "run-record.json")
			data, err := os.ReadFile(recordPath)
			if err != nil {
				return fmt.Errorf("read run record: %w", err)
			}

			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parse run record: %w", err)
			}

			if gates, ok := record["lifecycle_gates"].([]interface{}); ok {
				for _, gI := range gates {
					gateData, ok := gI.(map[string]interface{})
					if !ok {
						continue
					}
					gateID, _ := gateData["gate_id"].(string)
					if gateID == gate {
						gateData["status"] = "reviewed"
						record["lifecycle_gates"] = gates
						recordData, _ := json.MarshalIndent(record, "", "  ")
						os.WriteFile(recordPath, recordData, 0644)
						fmt.Printf("Gate %s marked as reviewed.\n", gate)
						return nil
					}
				}
			}
			return fmt.Errorf("gate not found: %s", gate)
		},
	}
	return cmd
}

func sdclGateApproveCommand() *cobra.Command {
	var gate string

	cmd := &cobra.Command{
		Use:   "approve-gate <gate-id>",
		Short: "Approve a gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gate = args[0]
			runDir := findRunDir()
			if runDir == "" {
				return fmt.Errorf("no SDLC run found")
			}

			recordPath := filepath.Join(runDir, "run-record.json")
			data, err := os.ReadFile(recordPath)
			if err != nil {
				return fmt.Errorf("read run record: %w", err)
			}

			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parse run record: %w", err)
			}

			if gates, ok := record["lifecycle_gates"].([]interface{}); ok {
				for _, gI := range gates {
					gateData, ok := gI.(map[string]interface{})
					if !ok {
						continue
					}
					gateID, _ := gateData["gate_id"].(string)
					if gateID == gate {
						gateData["status"] = "approved"
						record["lifecycle_gates"] = gates
						recordData, _ := json.MarshalIndent(record, "", "  ")
						os.WriteFile(recordPath, recordData, 0644)
						fmt.Printf("Gate %s approved.\n", gate)
						return nil
					}
				}
			}
			return fmt.Errorf("gate not found: %s", gate)
		},
	}
	return cmd
}

func sdclGateRejectCommand() *cobra.Command {
	var gate, reason string

	cmd := &cobra.Command{
		Use:   "reject-gate <gate-id>",
		Short: "Reject a gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gate = args[0]
			runDir := findRunDir()
			if runDir == "" {
				return fmt.Errorf("no SDLC run found")
			}

			recordPath := filepath.Join(runDir, "run-record.json")
			data, err := os.ReadFile(recordPath)
			if err != nil {
				return fmt.Errorf("read run record: %w", err)
			}

			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parse run record: %w", err)
			}

			if gates, ok := record["lifecycle_gates"].([]interface{}); ok {
				for _, gI := range gates {
					gateData, ok := gI.(map[string]interface{})
					if !ok {
						continue
					}
					gateID, _ := gateData["gate_id"].(string)
					if gateID == gate {
						gateData["status"] = "rejected"
						if reason != "" {
							gateData["rejection_reason"] = reason
						}
						record["lifecycle_gates"] = gates
						recordData, _ := json.MarshalIndent(record, "", "  ")
						os.WriteFile(recordPath, recordData, 0644)
						fmt.Printf("Gate %s rejected", gate)
						if reason != "" {
							fmt.Printf(": %s", reason)
						}
						fmt.Println()
						return nil
					}
				}
			}
			return fmt.Errorf("gate not found: %s", gate)
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Rejection reason")
	return cmd
}

func sdclGateRequestChangesCommand() *cobra.Command {
	var gate, reason string

	cmd := &cobra.Command{
		Use:   "request-changes <gate-id>",
		Short: "Request changes for a gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gate = args[0]
			runDir := findRunDir()
			if runDir == "" {
				return fmt.Errorf("no SDLC run found")
			}

			recordPath := filepath.Join(runDir, "run-record.json")
			data, err := os.ReadFile(recordPath)
			if err != nil {
				return fmt.Errorf("read run record: %w", err)
			}

			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parse run record: %w", err)
			}

			if gates, ok := record["lifecycle_gates"].([]interface{}); ok {
				for _, gI := range gates {
					gateData, ok := gI.(map[string]interface{})
					if !ok {
						continue
					}
					gateID, _ := gateData["gate_id"].(string)
					if gateID == gate {
						gateData["status"] = "changes_requested"
						if reason != "" {
							gateData["change_request_reason"] = reason
						}
						record["lifecycle_gates"] = gates
						recordData, _ := json.MarshalIndent(record, "", "  ")
						os.WriteFile(recordPath, recordData, 0644)
						fmt.Printf("Changes requested for gate %s", gate)
						if reason != "" {
							fmt.Printf(": %s", reason)
						}
						fmt.Println()
						return nil
					}
				}
			}
			return fmt.Errorf("gate not found: %s", gate)
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Change request reason")
	return cmd
}

func sdclGateInvalidateCommand() *cobra.Command {
	var gate string

	cmd := &cobra.Command{
		Use:   "invalidate-gates <gate-id>",
		Short: "Invalidate a gate and dependent downstream gates",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gate = args[0]
			runDir := findRunDir()
			if runDir == "" {
				return fmt.Errorf("no SDLC run found")
			}

			recordPath := filepath.Join(runDir, "run-record.json")
			data, err := os.ReadFile(recordPath)
			if err != nil {
				return fmt.Errorf("read run record: %w", err)
			}

			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				return fmt.Errorf("parse run record: %w", err)
			}

			gateOrder := []string{"G1", "G2", "G3", "G4", "G5", "G6", "G7", "G8", "G9", "G10"}
			targetIdx := -1
			for idx, g := range gateOrder {
				if g == gate {
					targetIdx = idx
					break
				}
			}

			if targetIdx == -1 {
				return fmt.Errorf("gate not found: %s", gate)
			}

			if gates, ok := record["lifecycle_gates"].([]interface{}); ok {
				for idx := targetIdx; idx < len(gateOrder) && idx < len(gates); idx++ {
					gateData, ok := gates[idx].(map[string]interface{})
					if !ok {
						continue
					}
					gateData["status"] = "invalidated"
					if idx > targetIdx {
						gateData["invalidation_reason"] = "upstream gate invalidated"
					}
				}

				record["lifecycle_gates"] = gates
				recordData, _ := json.MarshalIndent(record, "", "  ")
				os.WriteFile(recordPath, recordData, 0644)

				fmt.Printf("Gate %s invalidated and %d downstream gates invalidated.\n",
					gate, len(gateOrder)-targetIdx-1)
				return nil
			}

			return fmt.Errorf("no lifecycle gates found")
		},
	}
	return cmd
}

func findRunDir() string {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return ""
	}

	type dirInfo struct {
		name string
		mod  time.Time
	}
	var dirs []dirInfo

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		recordPath := filepath.Join(runsDir, e.Name(), "run-record.json")
		info, err := os.Stat(recordPath)
		if err != nil {
			continue
		}
		dirs = append(dirs, dirInfo{name: e.Name(), mod: info.ModTime()})
	}

	if len(dirs) == 0 {
		return ""
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].mod.After(dirs[j].mod)
	})

	return filepath.Join(runsDir, dirs[0].name)
}

func defaultTaskID() string {
	dir := findRunDir()
	if dir == "" {
		return ""
	}
	return filepath.Base(dir)
}

// Ensure strings is used
var _ = strings.TrimSpace
