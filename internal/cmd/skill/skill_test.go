package skill

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillListEmpty(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No skills installed") {
		t.Fatalf("expected 'No skills installed', got: %q", out.String())
	}
}

func TestSkillInstallAndList(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	sourceDir := filepath.Join(t.TempDir(), "test-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("test skill"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install failed: %v (%s)", err, errOut.String())
	}

	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "test-skill") {
		t.Fatalf("expected 'test-skill' in output, got: %q", output)
	}
	if !strings.Contains(output, "v0.0.1") {
		t.Fatalf("expected 'v0.0.1' in output, got: %q", output)
	}
}

func TestSkillInstallJSON(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	sourceDir := filepath.Join(t.TempDir(), "json-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", "--name", "my-skill", "--version", "2.0.0", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var skills []SkillManifest
	if err := json.Unmarshal([]byte(out.String()), &skills); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Fatalf("expected name 'my-skill', got %q", skills[0].Name)
	}
	if skills[0].Version != "2.0.0" {
		t.Fatalf("expected version '2.0.0', got %q", skills[0].Version)
	}
}

func TestSkillInfo(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	sourceDir := filepath.Join(t.TempDir(), "info-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"info", "info-skill"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "Name:        info-skill") {
		t.Fatalf("expected skill name in output, got: %q", output)
	}
	if !strings.Contains(output, "Version:     0.0.1") {
		t.Fatalf("expected version in output, got: %q", output)
	}
}

func TestSkillInfoNotFound(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"info", "nonexistent"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
	if !strings.Contains(errOut.String(), "no skills found") {
		t.Fatalf("expected 'no skills found' error, got: %q", errOut.String())
	}
}

func TestSkillEnableDisable(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	sourceDir := filepath.Join(t.TempDir(), "toggle-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Disable
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"disable", "toggle-skill"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "disabled") {
		t.Fatalf("expected 'disabled' in output, got: %q", out.String())
	}

	// Enable
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"enable", "toggle-skill"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "enabled") {
		t.Fatalf("expected 'enabled' in output, got: %q", out.String())
	}
}

func TestSkillRemove(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	sourceDir := filepath.Join(t.TempDir(), "remove-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Remove with force
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"remove", "--force", "remove-skill"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Skill removed") {
		t.Fatalf("expected 'Skill removed' in output, got: %q", out.String())
	}

	// Verify it's gone
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "remove-skill") {
		t.Fatalf("skill should be removed, but found in output: %q", out.String())
	}
}

func TestSkillRemoveWithoutForce(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"remove", "some-skill"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error without --force")
	}
	if !strings.Contains(errOut.String(), "no skills found") {
		t.Fatalf("expected 'no skills found' error, got: %q", errOut.String())
	}
}

func TestSkillInstallRequiresArg(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestSkillInstallSourceMustBeDirectory(t *testing.T) {
	dir := t.TempDir()
	originalSkillsDir := skillsDir
	skillsDir = dir
	defer func() { skillsDir = originalSkillsDir }()

	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", file})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for file source")
	}
	if !strings.Contains(errOut.String(), "must be a directory") {
		t.Fatalf("expected 'must be a directory' error, got: %q", errOut.String())
	}
}
