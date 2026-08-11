// Package exec provides the shell command execution subcommand.
package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"strings"
	"sync"
	"time"

	"github.com/deagy/lana/internal/app"
	"github.com/deagy/lana/internal/policy"
	"github.com/deagy/lana/pkg/config"
	"github.com/spf13/cobra"
)

// DefaultOutputLimit caps command output retained or streamed by Lana.
const DefaultOutputLimit int64 = 1 << 20

var ErrOutputLimit = errors.New("command output exceeded limit")

// Request describes a process invocation. The command is intentionally passed
// to a shell to preserve the legacy CLI semantics.
type Request struct {
	Command     string
	Dir         string
	Env         []string
	OutputLimit int64
	Stream      bool
	Stdout      io.Writer
	Stderr      io.Writer
}

// Result is bounded command output and elapsed time.
type Result struct {
	Stdout    string
	Stderr    string
	Elapsed   time.Duration
	Truncated bool
}

// ProcessExecutor permits the agent kernel and tests to inject a process
// implementation without granting the command direct process authority.
type ProcessExecutor interface {
	Run(context.Context, Request) (Result, error)
}

// NewCommand creates the exec command group.
func NewCommand() *cobra.Command { return NewCommandWithExecutor(OSExecutor{}) }

// NewCommandWithExecutor builds the command using executor.
func NewCommandWithExecutor(executor ProcessExecutor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [flags] <command> [args...]",
		Short: "Execute commands with timeout and bounded output",
		Long: `Execute shell commands with timeout and bounded output.

workspace-* modes intentionally reject execution: a shell command cannot be
contained by a label-only filesystem policy. Use unrestricted only when the
caller has authority to run an unrestricted local process.
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExec(cmd, args, executor)
		},
	}
	cmd.Flags().StringP("workdir", "d", "", "Working directory (must be within workspace for restricted modes)")
	cmd.Flags().StringArrayP("env", "e", []string{}, "Environment variable (KEY=VALUE, repeatable)")
	cmd.Flags().StringP("timeout", "t", "60s", "Execution timeout (e.g., 30s, 5m, 1h)")
	cmd.Flags().StringP("sandbox", "s", "workspace-write", "Policy mode (unrestricted, workspace-write, workspace-read-only)")
	cmd.Flags().BoolP("stream", "S", false, "Stream bounded output in real-time")
	cmd.Flags().Int64("max-output", DefaultOutputLimit, "Maximum combined stdout/stderr bytes")
	return cmd
}

func runExec(cmd *cobra.Command, args []string, executor ProcessExecutor) error {
	command := strings.Join(args, " ")
	workdir, _ := cmd.Flags().GetString("workdir")
	envFlags, _ := cmd.Flags().GetStringArray("env")
	settings, err := resolveExecutionSettings(cmd)
	if err != nil {
		return err
	}
	stream, _ := cmd.Flags().GetBool("stream")
	maxOutput, _ := cmd.Flags().GetInt64("max-output")
	if maxOutput <= 0 {
		return fmt.Errorf("max-output must be greater than zero")
	}
	p, err := policy.New(policy.Options{Mode: policy.Mode(settings.sandbox), Workspace: settings.workspace})
	if err != nil {
		return fmt.Errorf("create execution policy: %w", err)
	}
	if workdir == "" {
		workdir = settings.workspace
	}
	workdirEvaluation, err := p.Enforce(policy.OperationExecute, workdir, false)
	if err != nil {
		return fmt.Errorf("execution policy: %w", err)
	}
	if workdirEvaluation.CanonicalPath != "" {
		workdir = workdirEvaluation.CanonicalPath
	}

	env, err := validatedEnvironment(envFlags, settings.allowedEnvPrefixes)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), settings.timeout)
	defer cancel()
	result, err := executor.Run(ctx, Request{
		Command: command, Dir: workdir, Env: env, OutputLimit: maxOutput,
		Stream: stream, Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(),
	})
	if result.Stdout != "" && !stream {
		fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	}
	if result.Stderr != "" && !stream {
		fmt.Fprint(cmd.ErrOrStderr(), result.Stderr)
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("command timed out after %s", settings.timeout)
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return fmt.Errorf("command cancelled: %w", ctx.Err())
		}
		if errors.Is(err, ErrOutputLimit) {
			return fmt.Errorf("%w (%d bytes)", ErrOutputLimit, maxOutput)
		}
		return fmt.Errorf("command failed after %s: %w", result.Elapsed.Round(time.Millisecond), err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "\n[Command completed in %s]\n", result.Elapsed.Round(time.Millisecond))
	return nil
}

type executionSettings struct {
	workspace          string
	sandbox            string
	timeout            time.Duration
	allowedEnvPrefixes []string
}

func resolveExecutionSettings(cmd *cobra.Command) (executionSettings, error) {
	defaults := config.DefaultConfig()
	settings := executionSettings{
		sandbox:            defaults.Exec.Sandbox,
		timeout:            defaults.Exec.Timeout,
		allowedEnvPrefixes: append([]string(nil), defaults.Exec.AllowedEnvPrefixes...),
	}
	if application, ok := app.FromContext(cmd.Context()); ok {
		resolved := application.Config()
		cfg := resolved.Config()
		settings.workspace = resolved.Workspace()
		settings.sandbox = cfg.Exec.Sandbox
		settings.timeout = cfg.Exec.Timeout
		settings.allowedEnvPrefixes = append([]string(nil), cfg.Exec.AllowedEnvPrefixes...)
	}
	if settings.workspace == "" {
		var err error
		settings.workspace, err = os.Getwd()
		if err != nil {
			return executionSettings{}, fmt.Errorf("get workspace: %w", err)
		}
	}
	if flag := cmd.Flags().Lookup("sandbox"); flag != nil && flag.Changed {
		settings.sandbox, _ = cmd.Flags().GetString("sandbox")
	}
	if flag := cmd.Flags().Lookup("timeout"); flag != nil && flag.Changed {
		timeoutText, _ := cmd.Flags().GetString("timeout")
		timeout, err := time.ParseDuration(timeoutText)
		if err != nil || timeout <= 0 {
			if err == nil {
				err = errors.New("must be greater than zero")
			}
			return executionSettings{}, fmt.Errorf("invalid timeout: %w", err)
		}
		settings.timeout = timeout
	}
	if settings.timeout <= 0 {
		return executionSettings{}, fmt.Errorf("invalid configured execution timeout %s", settings.timeout)
	}
	return settings, nil
}

func validatedEnvironment(values, allowedPrefixes []string) ([]string, error) {
	prefixes, err := normalizeAllowedPrefixes(allowedPrefixes)
	if err != nil {
		return nil, err
	}
	env := make([]string, 0, len(os.Environ())+len(values))
	for _, value := range os.Environ() {
		key, _, ok := strings.Cut(value, "=")
		if ok && allowedEnvironmentKey(key, prefixes) {
			env = append(env, value)
		}
	}
	for _, value := range values {
		key, _, ok := strings.Cut(value, "=")
		if !ok || key == "" || strings.ContainsAny(key, "\x00=") {
			return nil, fmt.Errorf("invalid environment assignment %q", value)
		}
		if !allowedEnvironmentKey(key, prefixes) {
			return nil, fmt.Errorf("environment key %q is not allowed by exec.allowed_env_prefixes", key)
		}
		env = append(env, value)
	}
	return env, nil
}

func normalizeAllowedPrefixes(prefixes []string) ([]string, error) {
	result := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" || strings.Contains(prefix, "=") || strings.ContainsRune(prefix, '\x00') {
			return nil, fmt.Errorf("invalid exec.allowed_env_prefixes entry %q", prefix)
		}
		result = append(result, prefix)
	}
	return result, nil
}

func allowedEnvironmentKey(key string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// OSExecutor runs local processes. It is not an OS sandbox.
type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, request Request) (Result, error) {
	if request.OutputLimit <= 0 {
		request.OutputLimit = DefaultOutputLimit
	}
	process := osexec.Command("sh", "-c", request.Command)
	process.Dir = request.Dir
	process.Env = request.Env
	configureProcessGroup(process)
	limiter := newOutputLimiter(request.OutputLimit, request.Stream, request.Stdout, request.Stderr)
	process.Stdout = limiter.stdoutWriter()
	process.Stderr = limiter.stderrWriter()

	start := time.Now()
	if err := process.Start(); err != nil {
		return Result{Elapsed: time.Since(start)}, err
	}
	wait := make(chan error, 1)
	go func() { wait <- process.Wait() }()
	select {
	case err := <-wait:
		result := limiter.result(time.Since(start))
		if limiter.truncated() {
			return result, ErrOutputLimit
		}
		return result, err
	case <-ctx.Done():
		terminateProcessGroup(process)
		err := <-wait
		_ = err
		return limiter.result(time.Since(start)), ctx.Err()
	}
}

type outputLimiter struct {
	mu        sync.Mutex
	remaining int64
	stdout    strings.Builder
	stderr    strings.Builder
	stream    bool
	stdoutOut io.Writer
	stderrOut io.Writer
	cut       bool
}

func newOutputLimiter(limit int64, stream bool, stdout, stderr io.Writer) *outputLimiter {
	return &outputLimiter{remaining: limit, stream: stream, stdoutOut: stdout, stderrOut: stderr}
}

func (l *outputLimiter) stdoutWriter() io.Writer { return limitedWriter{limiter: l, stderr: false} }
func (l *outputLimiter) stderrWriter() io.Writer { return limitedWriter{limiter: l, stderr: true} }

type limitedWriter struct {
	limiter *outputLimiter
	stderr  bool
}

func (w limitedWriter) Write(data []byte) (int, error) {
	l := w.limiter
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cut || int64(len(data)) > l.remaining {
		allowed := l.remaining
		if allowed > 0 {
			l.append(w.stderr, data[:allowed])
			l.remaining = 0
		}
		l.cut = true
		return len(data), ErrOutputLimit
	}
	l.append(w.stderr, data)
	l.remaining -= int64(len(data))
	return len(data), nil
}

func (l *outputLimiter) append(stderr bool, data []byte) {
	if stderr {
		l.stderr.Write(data)
		if l.stream && l.stderrOut != nil {
			_, _ = l.stderrOut.Write(data)
		}
		return
	}
	l.stdout.Write(data)
	if l.stream && l.stdoutOut != nil {
		_, _ = l.stdoutOut.Write(data)
	}
}

func (l *outputLimiter) truncated() bool { l.mu.Lock(); defer l.mu.Unlock(); return l.cut }
func (l *outputLimiter) result(elapsed time.Duration) Result {
	l.mu.Lock()
	defer l.mu.Unlock()
	return Result{Stdout: l.stdout.String(), Stderr: l.stderr.String(), Elapsed: elapsed, Truncated: l.cut}
}

func isSecretKey(key string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range []string{"key", "token", "secret", "password", "credential", "api_key", "apikey"} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
