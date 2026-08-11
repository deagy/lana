// Package version provides build-time version information.
package version

import (
	"fmt"
	"runtime"
)

// Build-time variables set via -ldflags.
// Example: go build -ldflags="-X github.com/deagy/lana/pkg/version.Version=1.0.0 -X github.com/deagy/lana/pkg/version.Commit=abc123 -X github.com/deagy/lana/pkg/version.BuildDate=2026-08-11"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Info returns a structured version string.
func Info() string {
	return fmt.Sprintf("lana version %s (commit: %s, built: %s, %s)",
		Version, Commit, BuildDate, runtime.Version())
}

// Format returns version info for display.
func Format() string {
	return Info()
}

// Details returns version details as a map.
func Details() map[string]string {
	return map[string]string{
		"version": Version,
		"commit":  Commit,
		"built":   BuildDate,
		"go":      runtime.Version(),
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}
}
