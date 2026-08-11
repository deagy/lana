package goal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGoalCreateCommand(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	cmd := goalCreateCommand()
	cmd.SetArgs([]string{"--objective", "Test goal creation", "--with-budget", "--token-budget", "1000"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("goal create failed: %v", err)
	}

	// Verify file was created
	goalsPath := filepath.Join(tmpDir, ".lana", goalsFile)
	data, err := os.ReadFile(goalsPath)
	if err != nil {
		t.Fatalf("goals file not found: %v", err)
	}

	var gf goalFile
	if err := json.Unmarshal(data, &gf); err != nil {
		t.Fatalf("parse goals: %v", err)
	}

	if len(gf.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(gf.Goals))
	}

	if gf.Goals[0].Objective != "Test goal creation" {
		t.Errorf("expected objective 'Test goal creation', got %v", gf.Goals[0].Objective)
	}

	if gf.Goals[0].Status != StatusPending {
		t.Errorf("expected status '%s', got %v", StatusPending, gf.Goals[0].Status)
	}

	if gf.Goals[0].TokenBudget == nil || *gf.Goals[0].TokenBudget != 1000 {
		t.Errorf("expected budget 1000, got %v", gf.Goals[0].TokenBudget)
	}
}

func TestGoalListCommand(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	goalsPath := filepath.Join(tmpDir, ".lana", goalsFile)
	os.MkdirAll(filepath.Dir(goalsPath), 0755)
	gf := &goalFile{
		Version: 1,
		Goals: []Goal{
			{ID: "goal-0001", Objective: "Test 1", Status: StatusPending},
			{ID: "goal-0002", Objective: "Test 2", Status: StatusComplete},
		},
	}
	data, _ := json.MarshalIndent(gf, "", "  ")
	os.WriteFile(goalsPath, data, 0644)

	cmd := goalListCommand()
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("goal list failed: %v", err)
	}
}

func TestGoalShowCommand(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	goalsPath := filepath.Join(tmpDir, ".lana", goalsFile)
	os.MkdirAll(filepath.Dir(goalsPath), 0755)
	gf := &goalFile{
		Version: 1,
		Goals: []Goal{
			{ID: "goal-0001", Objective: "Test objective", Status: StatusPending},
		},
	}
	data, _ := json.MarshalIndent(gf, "", "  ")
	os.WriteFile(goalsPath, data, 0644)

	cmd := goalShowCommand()
	cmd.SetArgs([]string{"goal-0001"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("goal show failed: %v", err)
	}
}

func TestGoalShowNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	goalsPath := filepath.Join(tmpDir, ".lana", goalsFile)
	os.MkdirAll(filepath.Dir(goalsPath), 0755)
	gf := &goalFile{
		Version: 1,
		Goals:   []Goal{},
	}
	data, _ := json.MarshalIndent(gf, "", "  ")
	os.WriteFile(goalsPath, data, 0644)

	cmd := goalShowCommand()
	cmd.SetArgs([]string{"nonexistent"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for nonexistent goal")
	}
}

func TestGoalUpdateCommand(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	goalsPath := filepath.Join(tmpDir, ".lana", goalsFile)
	os.MkdirAll(filepath.Dir(goalsPath), 0755)
	gf := &goalFile{
		Version: 1,
		Goals: []Goal{
			{ID: "goal-0001", Objective: "Test", Status: StatusPending},
		},
	}
	data, _ := json.MarshalIndent(gf, "", "  ")
	os.WriteFile(goalsPath, data, 0644)

	cmd := goalUpdateCommand()
	cmd.SetArgs([]string{"goal-0001", "--status", StatusComplete, "--summary", "Done"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("goal update failed: %v", err)
	}

	// Verify status was updated
	data, _ = os.ReadFile(goalsPath)
	gf2 := &goalFile{Version: 1}
	json.Unmarshal(data, gf2)
	if gf2.Goals[0].Status != StatusComplete {
		t.Errorf("expected status '%s', got %v", StatusComplete, gf2.Goals[0].Status)
	}
	if gf2.Goals[0].LastSummary != "Done" {
		t.Errorf("expected summary 'Done', got %v", gf2.Goals[0].LastSummary)
	}
}
