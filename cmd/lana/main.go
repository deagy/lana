// Package main is the entry point for the Lana CLI — a Codex CLI clone.
package main

import (
	"os"

	"github.com/deagy/lana/cmd/lana/root"
)

func main() {
	cmd := root.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
