package exec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/deagy/lana/internal/app"
	"github.com/deagy/lana/pkg/config"
)

type fakeExecutor struct {
	request  Request
	result   Result
	err      error
	deadline time.Time
}

func (f *fakeExecutor) Run(ctx context.Context, request Request) (Result, error) {
	f.request = request
	f.deadline, _ = ctx.Deadline()
	return f.result, f.err
}

func TestIsSecretKey(t *testing.T) {
	tests := []struct {
		key    string
		expect bool
	}{
		{"API_KEY", true},
		{"TOKEN", true},
		{"SECRET", true},
		{"PASSWORD", true},
		{"credential", true},
		{"apikey", true},
		{"HOME", false},
		{"PATH", false},
		{"LANG", false},
	}

	for _, tt := range tests {
		if got := isSecretKey(tt.key); got != tt.expect {
			t.Errorf("isSecretKey(%q) = %v, want %v", tt.key, got, tt.expect)
		}
	}
}

func TestIsSecretKey_CaseInsensitive(t *testing.T) {
	if !isSecretKey("api_key") {
		t.Error("expected 'api_key' to be detected as secret")
	}
	if !isSecretKey("MyToken") {
		t.Error("expected 'MyToken' to be detected as secret")
	}
}

func TestExecCommand_NotFound(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"/nonexistent/command/that/does/not/exist"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestExecCommand_Valid(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"echo", "test", "--sandbox", "unrestricted"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecCommand_Timeout(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"sleep", "0.1", "&&", "echo", "done", "--timeout", "5s", "--sandbox", "unrestricted"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecCommand_Workdir(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644)

	cmd := NewCommand()
	cmd.SetArgs([]string{"cat", "test.txt", "--workdir", tmpDir, "--sandbox", "unrestricted"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecCommand_ShortTimeout(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"sleep", "1", "--timeout", "0.1s", "--sandbox", "unrestricted"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout in error message, got: %v", err)
	}
}

func TestExecCommand_BadTimeout(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"echo", "test", "--timeout", "not-a-duration"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestExecCommandRejectsDisallowedEnv(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"echo", "secret", "--env", "API_KEY=hidden", "--sandbox", "unrestricted"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected allowlist rejection, got: %v", err)
	}
}

func TestValidatedEnvironmentFiltersInheritedAndRejectsExplicit(t *testing.T) {
	t.Setenv("SAFE_INHERITED", "yes")
	t.Setenv("BLOCKED_INHERITED", "no")
	env, err := validatedEnvironment([]string{"SAFE_EXPLICIT=ok"}, []string{"SAFE_"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(env, "SAFE_INHERITED=yes") || !slices.Contains(env, "SAFE_EXPLICIT=ok") {
		t.Fatalf("allowed keys missing from %q", env)
	}
	for _, value := range env {
		if strings.HasPrefix(value, "BLOCKED_INHERITED=") {
			t.Fatalf("blocked inherited key leaked: %q", value)
		}
	}
	if _, err := validatedEnvironment([]string{"BLOCKED_EXPLICIT=no"}, []string{"SAFE_"}); err == nil {
		t.Fatal("expected explicit disallowed key rejection")
	}
}

func TestExecUsesAppConfigurationUnlessFlagsOverride(t *testing.T) {
	workspace := t.TempDir()
	application := testApplication(t, workspace, "exec:\n  sandbox: unrestricted\n  timeout: 3s\n  allowed_env_prefixes: [SAFE_]\n")
	fake := &fakeExecutor{result: Result{}}
	cmd := NewCommandWithExecutor(fake)
	cmd.SetContext(app.WithContext(context.Background(), application))
	cmd.SetArgs([]string{"echo", "test", "--env", "SAFE_VALUE=ok"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if fake.request.Dir != workspace || !slices.Contains(fake.request.Env, "SAFE_VALUE=ok") {
		t.Fatalf("request did not use app config: %+v", fake.request)
	}
	if remaining := time.Until(fake.deadline); remaining < 2*time.Second || remaining > 4*time.Second {
		t.Fatalf("configured timeout not used; remaining=%s", remaining)
	}

	cmd = NewCommandWithExecutor(fake)
	cmd.SetContext(app.WithContext(context.Background(), application))
	cmd.SetArgs([]string{"echo", "test", "--sandbox", "workspace-write", "--timeout", "100ms"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be enforced") {
		t.Fatalf("flag sandbox should override config, got %v", err)
	}
}

func testApplication(t *testing.T, workspace, contents string) *app.Application {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	application, err := app.New(app.Options{Config: config.ResolveOptions{Workspace: workspace, WorkingDirectory: workspace, ConfigPath: configPath, UserConfigPath: filepath.Join(t.TempDir(), "missing.yaml")}})
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func TestExecCommandRestrictedModeIsExplicitlyUnenforceable(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"echo", "test"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be enforced") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecCommandInjectsExecutorAndBoundsRequest(t *testing.T) {
	fake := &fakeExecutor{result: Result{Stdout: "ok"}}
	cmd := NewCommandWithExecutor(fake)
	cmd.SetArgs([]string{"echo", "test", "--sandbox", "unrestricted", "--max-output", "123"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if fake.request.Command != "echo test" || fake.request.OutputLimit != 123 {
		t.Fatalf("request = %+v", fake.request)
	}
}

func TestOSExecutorOutputLimit(t *testing.T) {
	_, err := (OSExecutor{}).Run(context.Background(), Request{Command: "printf 123456", Dir: t.TempDir(), Env: os.Environ(), OutputLimit: 3})
	if !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("error = %v, want output limit", err)
	}
}
