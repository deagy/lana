package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/deagy/lana/internal/session"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		store := session.NewMemoryStore()
		defer store.Close()

		sessions, err := store.List(context.Background())
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found")
			return nil
		}

		fmt.Println("Sessions:")
		for _, s := range sessions {
			fmt.Printf("  %s: %s (%s @ %s) - %d messages\n",
				s.ID[:8], s.Title, s.Model, s.Provider, s.Messages)
		}
		return nil
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]
		store := session.NewMemoryStore()
		defer store.Close()

		if err := store.Delete(context.Background(), sessionID); err != nil {
			return err
		}

		fmt.Printf("Deleted session %s\n", sessionID[:8])
		return nil
	},
}

func init() {
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
}
