package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deagy/lana/internal/approval"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/tools"
)

// Run starts the interactive TUI.
func Run(ctx context.Context, sessionID string, store session.Store, client provider.Client, policy approval.Policy, registry *tools.Registry) error {
	// Create the model
	model, err := New(sessionID, store, client, policy, registry)
	if err != nil {
		return fmt.Errorf("create model: %w", err)
	}

	// Run the Bubble Tea program
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}

	return nil
}

// RunWithPrompt starts the TUI with an initial prompt.
func RunWithPrompt(ctx context.Context, sessionID string, store session.Store, client provider.Client, policy approval.Policy, registry *tools.Registry, prompt string) error {
	// Create the model
	model, err := New(sessionID, store, client, policy, registry)
	if err != nil {
		return fmt.Errorf("create model: %w", err)
	}

	// Pre-populate the composer with the prompt
	model.composer.textarea.SetValue(prompt)

	// Run the Bubble Tea program
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}

	return nil
}
