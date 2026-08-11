// Package logger provides structured logging initialization using slog.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu    sync.Mutex
	level slog.Level
)

// Options configures the logger.
type Options struct {
	Level    string
	Format   string // "text" or "json"
	ToFile   bool
	FilePath string
}

// Init creates and returns an slog.Logger configured with the given options.
func Init(opts Options) *slog.Logger {
	mu.Lock()
	defer mu.Unlock()

	var l slog.Level
	switch opts.Level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	level = l

	var w io.Writer = os.Stdout

	if opts.ToFile && opts.FilePath != "" {
		f, err := openPrivateLogFile(opts.FilePath)
		if err == nil {
			w = io.MultiWriter(os.Stdout, f)
		}
	}

	var handler slog.Handler
	var opts2 slog.HandlerOptions
	opts2.Level = l

	switch opts.Format {
	case "json":
		handler = slog.NewJSONHandler(w, &opts2)
	default:
		handler = slog.NewTextHandler(w, &opts2)
	}

	lg := slog.New(handler)
	slog.SetDefault(lg)
	return lg
}

// openPrivateLogFile opens path for appending after ensuring the configured log
// directory and file are accessible only to their owner. Failures are returned
// to the caller so it can continue logging to stdout.
func openPrivateLogFile(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if err := ensurePrivateLogDir(dir); err != nil {
		return nil, err
	}

	if err := rejectSymlink(path); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// ensurePrivateLogDir creates the configured log directory with private
// permissions. Only the configured directory itself is chmodded when it
// already exists; its parents are left unchanged.
func ensurePrivateLogDir(dir string) error {
	// A filename without a directory component is intentionally written in the
	// caller's current directory. That directory is not a configured log
	// directory, so never alter its permissions.
	if dir == "." {
		return nil
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := rejectSymlinkComponents(absDir); err != nil {
		return err
	}
	if err := os.MkdirAll(absDir, 0700); err != nil {
		return err
	}
	if err := rejectSymlinkComponents(absDir); err != nil {
		return err
	}
	return os.Chmod(absDir, 0700)
}

// rejectSymlink ensures an existing file path is not redirected elsewhere.
func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlinked log file %q", path)
	}
	return nil
}

// rejectSymlinkComponents rejects paths that traverse a symlink. This avoids
// changing permissions on a directory outside the configured log target.
func rejectSymlinkComponents(path string) error {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	root := volume + string(filepath.Separator)
	rel, err := filepath.Rel(root, clean)
	if err != nil {
		return err
	}

	current := root
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing log path through symlink %q", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("log path component %q is not a directory", current)
		}
	}
	return nil
}

// Get returns the package-level default logger.
func Get() *slog.Logger {
	return slog.Default()
}

// SetLevel changes the minimum log level.
func SetLevel(lvl string) {
	switch lvl {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
}

// Level returns the current minimum log level.
func Level() string {
	switch level {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "info"
	}
}

// Debug logs a debug message with optional key-value pairs.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Info logs an info message with optional key-value pairs.
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn logs a warning message with optional key-value pairs.
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs an error message with optional key-value pairs.
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
