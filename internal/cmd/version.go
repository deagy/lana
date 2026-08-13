package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version and commit are set via ldflags during build
var (
	Version = "dev"
	Commit  = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Lana %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Go: %s\n", runtime.Version())
		fmt.Printf("OS: %s\n", runtime.GOOS)
		fmt.Printf("Arch: %s\n", runtime.GOARCH)
		return nil
	},
}
