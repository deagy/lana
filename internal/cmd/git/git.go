// Package git provides git integration subcommands.
package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// NewCommand creates the git command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Git operations",
		Long: `Git operations wrapper. Delegates to system git.

Subcommands:
  status      Show working tree status
  diff        Show diff
  log         Show log
  branch      Show current branch
  commit      Commit changes
  push        Push changes
  pull        Pull changes
  remote      Manage remotes
  fetch       Fetch from remote
  stash       Stash changes
  merge       Merge branches
  pr-create   Create a draft PR
  pr-list     List open PRs
  pr-diff     Show PR diff
`,
	}
	cmd.AddCommand(gitStatusCommand())
	cmd.AddCommand(gitDiffCommand())
	cmd.AddCommand(gitLogCommand())
	cmd.AddCommand(gitBranchCommand())
	cmd.AddCommand(gitCommitCommand())
	cmd.AddCommand(gitPushCommand())
	cmd.AddCommand(gitPullCommand())
	cmd.AddCommand(gitRemoteCommand())
	cmd.AddCommand(gitFetchCommand())
	cmd.AddCommand(gitStashCommand())
	cmd.AddCommand(gitMergeCommand())
	cmd.AddCommand(gitPRCreateCommand())
	cmd.AddCommand(gitPRListCommand())
	cmd.AddCommand(gitPRDiffCommand())
	return cmd
}

func gitStatusCommand() *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show working tree status",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"status"}
			if short {
				gitArgs = append(gitArgs, "--short")
			}
			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git status: %w", err)
			}
			if len(strings.TrimSpace(string(out))) == 0 {
				fmt.Println("Working tree clean.")
				return nil
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&short, "short", "s", false, "Short format")
	return cmd
}

func gitDiffCommand() *cobra.Command {
	var cached bool
	var statFlag bool

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show diff",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"diff"}
			if cached {
				gitArgs = append(gitArgs, "--cached")
			}
			if statFlag {
				gitArgs = append(gitArgs, "--stat")
			}
			gitArgs = append(gitArgs, args...)

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git diff: %w", err)
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&cached, "cached", "c", false, "Show cached changes")
	cmd.Flags().BoolVarP(&statFlag, "stat", "S", false, "Show stat summary")
	return cmd
}

func gitLogCommand() *cobra.Command {
	var n, maxCount int
	var oneline bool

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show log",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"log"}
			if oneline {
				gitArgs = append(gitArgs, "--oneline")
			}
			if maxCount > 0 {
				gitArgs = append(gitArgs, "-n", fmt.Sprintf("%d", maxCount))
			}
			if n > 0 {
				gitArgs = append(gitArgs, "-n", fmt.Sprintf("%d", n))
			}
			gitArgs = append(gitArgs, args...)

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git log: %w", err)
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().IntVarP(&n, "n", "n", 0, "Number of commits")
	cmd.Flags().IntVarP(&maxCount, "max-count", "N", 10, "Maximum commits")
	cmd.Flags().BoolVarP(&oneline, "oneline", "1", false, "One line per commit")
	return cmd
}

func gitBranchCommand() *cobra.Command {
	var all bool
	var current bool

	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Show current branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"branch"}
			if all {
				gitArgs = append(gitArgs, "-a")
			}
			if current {
				gitArgs = []string{"branch", "--show-current"}
			}

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git branch: %w", err)
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Show all branches including remote")
	cmd.Flags().BoolVarP(&current, "current", "c", false, "Show only current branch")
	return cmd
}

func gitCommitCommand() *cobra.Command {
	var message string
	var amend bool

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" {
				return fmt.Errorf("--message is required")
			}

			gitArgs := []string{"commit", "-m", message}
			if amend {
				gitArgs = append(gitArgs, "--amend")
			}

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git commit: %w\n%s", err, string(out))
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (required)")
	cmd.Flags().BoolVarP(&amend, "amend", "a", false, "Amend last commit")
	return cmd
}

func gitPushCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"push"}
			if force {
				gitArgs = append(gitArgs, "--force-with-lease")
			}
			gitArgs = append(gitArgs, args...)

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git push: %w\n%s", err, string(out))
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force push (with lease)")
	return cmd
}

func gitPullCommand() *cobra.Command {
	var rebase bool

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"pull"}
			if rebase {
				gitArgs = append(gitArgs, "--rebase")
			}
			gitArgs = append(gitArgs, args...)

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git pull: %w\n%s", err, string(out))
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&rebase, "rebase", "r", false, "Rebase on pull")
	return cmd
}

func gitRemoteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manage remotes",
		Long: `Manage git remotes.

Subcommands:
  list      List remotes
  add       Add a remote
  remove    Remove a remote
  set-url   Set remote URL
`,
	}
	cmd.AddCommand(gitRemoteListCommand())
	cmd.AddCommand(gitRemoteAddCommand())
	cmd.AddCommand(gitRemoteRemoveCommand())
	cmd.AddCommand(gitRemoteSetURLCommand())

	return cmd
}

func gitRemoteListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List remotes",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := exec.Command("git", "remote", "-v").CombinedOutput()
			if err != nil {
				return fmt.Errorf("git remote list: %w", err)
			}
			fmt.Print(string(out))
			return nil
		},
	}
}

func gitRemoteAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add a remote",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := exec.Command("git", "remote", "add", args[0], args[1]).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git remote add: %w\n%s", err, string(out))
			}
			fmt.Printf("Remote %q added: %s\n", args[0], args[1])
			return nil
		},
	}
	return cmd
}

func gitRemoteRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := exec.Command("git", "remote", "remove", args[0]).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git remote remove: %w\n%s", err, string(out))
			}
			fmt.Printf("Remote %q removed\n", args[0])
			return nil
		},
	}
}

func gitRemoteSetURLCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-url <name> <url>",
		Short: "Set remote URL",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := exec.Command("git", "remote", "set-url", args[0], args[1]).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git remote set-url: %w\n%s", err, string(out))
			}
			fmt.Printf("Remote %q URL set to: %s\n", args[0], args[1])
			return nil
		},
	}
}

func gitFetchCommand() *cobra.Command {
	var all bool
	var prune bool

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch from remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"fetch"}
			if all {
				gitArgs = append(gitArgs, "--all")
			}
			if prune {
				gitArgs = append(gitArgs, "--prune")
			}
			gitArgs = append(gitArgs, args...)

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git fetch: %w\n%s", err, string(out))
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Fetch all remotes")
	cmd.Flags().BoolVarP(&prune, "prune", "p", false, "Prune remote-tracking refs")
	return cmd
}

func gitStashCommand() *cobra.Command {
	var save, pop bool
	var message string

	cmd := &cobra.Command{
		Use:   "stash",
		Short: "Stash changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if save {
				gitArgs := []string{"stash", "save"}
				if message != "" {
					gitArgs = append(gitArgs, message)
				}
				out, err := exec.Command("git", gitArgs...).CombinedOutput()
				if err != nil {
					return fmt.Errorf("git stash save: %w\n%s", err, string(out))
				}
				fmt.Print(string(out))
			} else if pop {
				out, err := exec.Command("git", "stash", "pop").CombinedOutput()
				if err != nil {
					return fmt.Errorf("git stash pop: %w\n%s", err, string(out))
				}
				fmt.Print(string(out))
			} else {
				out, err := exec.Command("git", "stash", "list").CombinedOutput()
				if err != nil {
					return fmt.Errorf("git stash list: %w\n%s", err, string(out))
				}
				fmt.Print(string(out))
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&save, "save", "s", false, "Stash changes")
	cmd.Flags().BoolVarP(&pop, "pop", "p", false, "Pop stashed changes")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Stash message")
	return cmd
}

func gitMergeCommand() *cobra.Command {
	var noFF bool

	cmd := &cobra.Command{
		Use:   "merge <branch>",
		Short: "Merge branches",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"merge"}
			if noFF {
				gitArgs = append(gitArgs, "--no-ff")
			}
			gitArgs = append(gitArgs, args[0])

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git merge: %w\n%s", err, string(out))
			}
			fmt.Print(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&noFF, "no-ff", "n", false, "No fast-forward")
	return cmd
}

func gitPRCreateCommand() *cobra.Command {
	var title, body, base string

	cmd := &cobra.Command{
		Use:   "pr-create",
		Short: "Create a draft PR",
		Long: `Create a pull request through the configured platform (GitHub/GitLab).

Examples:
  lana git pr-create --title "Add feature X" --base main
  lana git pr-create --title "Fix bug" --body "Fixes #123" --base develop
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}

			fmt.Printf("Creating pull request: %q\n", title)
			if base != "" {
				fmt.Printf("  Base branch: %s\n", base)
			}
			if body != "" {
				fmt.Printf("  Body: %s\n", body)
			}

			// Try to detect platform
			remoteURL, _ := exec.Command("git", "remote", "get-url", "origin").CombinedOutput()
			if len(remoteURL) > 0 && strings.Contains(string(remoteURL), "gitlab") {
				fmt.Println("  Platform: GitLab (PR creation pending)")
			} else {
				fmt.Println("  Platform: GitHub (PR creation pending)")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "PR title (required)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "PR body")
	cmd.Flags().StringVarP(&base, "base", "B", "", "Base branch")
	return cmd
}

func gitPRListCommand() *cobra.Command {
	var state string

	cmd := &cobra.Command{
		Use:   "pr-list",
		Short: "List open PRs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if state == "" {
				state = "open"
			}

			fmt.Printf("Listing %s PRs...\n", state)
			fmt.Println("  (GitHub/GitLab PR listing integration pending)")
			return nil
		},
	}

	cmd.Flags().StringVarP(&state, "state", "s", "open", "PR state (open, closed)")
	return cmd
}

func gitPRDiffCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "pr-diff <number>",
		Short: "Show PR diff",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Fetching diff for PR #%s...\n", args[0])
			fmt.Println("  (GitHub/GitLab PR diff integration pending)")
			return nil
		},
	}

	return cmd
}
