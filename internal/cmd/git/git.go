// Package git provides git integration subcommands.
package git

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	ghclient "github.com/deagy/lana/internal/github"
	glclient "github.com/deagy/lana/internal/gitlab"
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
  pr-create   Create a PR/MR
  pr-list     List open PRs/MRs
  pr-diff     Show PR/MR diff
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
	var cached, statFlag bool
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
	cmd.Flags().BoolVarP(&oneline, "oneline", "1", false, "One line per commit")
	cmd.Flags().IntVarP(&maxCount, "max-count", "n", 0, "Maximum number of commits")
	cmd.Flags().IntVarP(&n, "num", "N", 0, "Number of commits")
	return cmd
}

func gitBranchCommand() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Show current branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"branch"}
			if all {
				gitArgs = append(gitArgs, "-a")
			}
			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git branch: %w", err)
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Show all branches")
	return cmd
}

func gitCommitCommand() *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" {
				return fmt.Errorf("--message is required")
			}
			gitArgs := []string{"commit", "-m", message}
			gitArgs = append(gitArgs, args...)
			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git commit: %w\n%s", err, string(out))
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (required)")
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
				gitArgs = append(gitArgs, "--force")
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
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force push")
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
	var verbose bool
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manage remotes",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"remote"}
			if verbose {
				gitArgs = append(gitArgs, "-v")
			}
			if len(args) > 0 {
				gitArgs = append(gitArgs, args...)
			}
			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git remote: %w", err)
			}
			fmt.Print(string(out))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	return cmd
}

func gitFetchCommand() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch from remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			gitArgs := []string{"fetch"}
			if all {
				gitArgs = append(gitArgs, "--all")
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
				gitArgs := []string{"stash", "push"}
				if message != "" {
					gitArgs = append(gitArgs, "-m", message)
				}
				out, err := exec.Command("git", gitArgs...).CombinedOutput()
				if err != nil {
					return fmt.Errorf("git stash push: %w\n%s", err, string(out))
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

// detectPlatform returns "github" or "gitlab" based on the remote URL.
func detectPlatform() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get remote URL: %w", err)
	}
	remoteURL := strings.TrimSpace(string(out))
	if strings.Contains(remoteURL, "gitlab") {
		return "gitlab", nil
	}
	return "github", nil
}

func gitPRCreateCommand() *cobra.Command {
	var title, body, base string
	cmd := &cobra.Command{
		Use:   "pr-create",
		Short: "Create a pull/merge request",
		Long: `Create a pull request (GitHub) or merge request (GitLab).

Examples:
  lana git pr-create --title "Add feature X" --base main
  lana git pr-create --title "Fix bug" --body "Fixes #123" --base develop
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			platform, err := detectPlatform()
			if err != nil {
				return err
			}
			currentBranch, err := getCurrentBranch()
			if err != nil {
				return fmt.Errorf("get current branch: %w", err)
			}
			if base == "" {
				base = "main"
			}

			switch platform {
			case "github":
				return createGitHubPR(cmd, title, body, currentBranch, base)
			case "gitlab":
				return createGitLabMR(cmd, title, body, currentBranch, base)
			default:
				return fmt.Errorf("unsupported platform: %s", platform)
			}
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "PR/MR title (required)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "PR/MR body/description")
	cmd.Flags().StringVarP(&base, "base", "B", "", "Base branch (default: main)")
	return cmd
}

func createGitHubPR(cmd *cobra.Command, title, body, head, base string) error {
	owner, repo, err := ghclient.DetectOwnerRepo()
	if err != nil {
		return fmt.Errorf("detect GitHub repo: %w", err)
	}
	client := ghclient.FromEnv()
	pr, err := client.CreatePR(cmd.Context(), owner, repo, ghclient.CreatePROptions{
		Title: title,
		Body:  body,
		Head:  head,
		Base:  base,
		Draft: true,
	})
	if err != nil {
		return fmt.Errorf("create GitHub PR: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created GitHub PR #%d: %s\n", pr.Number, pr.Title)
	fmt.Fprintf(cmd.OutOrStdout(), "  URL: %s\n", pr.HTMLURL)
	fmt.Fprintf(cmd.OutOrStdout(), "  Head: %s -> Base: %s\n", pr.HeadRef, pr.BaseRef)
	return nil
}

func createGitLabMR(cmd *cobra.Command, title, body, sourceBranch, targetBranch string) error {
	projectPath, err := glclient.DetectProjectPath()
	if err != nil {
		return fmt.Errorf("detect GitLab project: %w", err)
	}
	client := glclient.FromEnv()
	mr, err := client.CreateMR(cmd.Context(), projectPath, glclient.MROptions{
		Title:        title,
		Description:  body,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		Draft:        true,
	})
	if err != nil {
		return fmt.Errorf("create GitLab MR: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created GitLab MR !%d: %s\n", mr.IID, mr.Title)
	fmt.Fprintf(cmd.OutOrStdout(), "  URL: %s\n", mr.WebURL)
	fmt.Fprintf(cmd.OutOrStdout(), "  Source: %s -> Target: %s\n", mr.SourceBranch, mr.TargetBranch)
	return nil
}

func gitPRListCommand() *cobra.Command {
	var state string
	cmd := &cobra.Command{
		Use:   "pr-list",
		Short: "List open PRs/MRs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if state == "" {
				state = "open"
			}
			platform, err := detectPlatform()
			if err != nil {
				return err
			}
			switch platform {
			case "github":
				return listGitHubPRs(cmd, state)
			case "gitlab":
				return listGitLabMRs(cmd, state)
			default:
				return fmt.Errorf("unsupported platform: %s", platform)
			}
		},
	}
	cmd.Flags().StringVarP(&state, "state", "s", "open", "PR/MR state (open, closed, merged)")
	return cmd
}

func listGitHubPRs(cmd *cobra.Command, state string) error {
	owner, repo, err := ghclient.DetectOwnerRepo()
	if err != nil {
		return fmt.Errorf("detect GitHub repo: %w", err)
	}
	client := ghclient.FromEnv()
	prs, err := client.ListPRs(cmd.Context(), owner, repo, state, "", "")
	if err != nil {
		return fmt.Errorf("list GitHub PRs: %w", err)
	}
	if len(prs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No open PRs found.")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "GitHub PRs (%d):\n\n", len(prs))
	for _, pr := range prs {
		draft := ""
		if pr.Draft {
			draft = " [draft]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  #%d %s%s\n", pr.Number, pr.Title, draft)
		fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", pr.HTMLURL)
	}
	return nil
}

func listGitLabMRs(cmd *cobra.Command, state string) error {
	projectPath, err := glclient.DetectProjectPath()
	if err != nil {
		return fmt.Errorf("detect GitLab project: %w", err)
	}
	client := glclient.FromEnv()
	var mrState glclient.MROpenState
	switch state {
	case "open":
		mrState = glclient.MRStateOpened
	case "closed":
		mrState = glclient.MRStateClosed
	case "merged":
		mrState = glclient.MRStateMerged
	default:
		mrState = glclient.MRStateOpened
	}
	mrs, err := client.ListMRs(cmd.Context(), projectPath, mrState)
	if err != nil {
		return fmt.Errorf("list GitLab MRs: %w", err)
	}
	if len(mrs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No open MRs found.")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "GitLab MRs (%d):\n\n", len(mrs))
	for _, mr := range mrs {
		draft := ""
		if mr.Draft {
			draft = " [draft]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  !%d %s%s\n", mr.IID, mr.Title, draft)
		fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", mr.WebURL)
	}
	return nil
}

func gitPRDiffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr-diff <number>",
		Short: "Show PR/MR diff",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			platform, err := detectPlatform()
			if err != nil {
				return err
			}
			var number int
			if _, err := fmt.Sscanf(args[0], "%d", &number); err != nil {
				return fmt.Errorf("invalid PR number: %s", args[0])
			}
			switch platform {
			case "github":
				return showGitHubPRDiff(cmd, number)
			case "gitlab":
				return showGitLabMRDiff(cmd, number)
			default:
				return fmt.Errorf("unsupported platform: %s", platform)
			}
		},
	}
	return cmd
}

func showGitHubPRDiff(cmd *cobra.Command, number int) error {
	owner, repo, err := ghclient.DetectOwnerRepo()
	if err != nil {
		return fmt.Errorf("detect GitHub repo: %w", err)
	}
	client := ghclient.FromEnv()
	diff, err := client.GetPRDiff(cmd.Context(), owner, repo, number)
	if err != nil {
		return fmt.Errorf("get GitHub PR diff: %w", err)
	}
	fmt.Print(diff)
	return nil
}

func showGitLabMRDiff(cmd *cobra.Command, number int) error {
	projectPath, err := glclient.DetectProjectPath()
	if err != nil {
		return fmt.Errorf("detect GitLab project: %w", err)
	}
	client := glclient.FromEnv()
	diff, err := client.GetMRDiff(cmd.Context(), projectPath, number)
	if err != nil {
		return fmt.Errorf("get GitLab MR diff: %w", err)
	}
	fmt.Print(diff)
	return nil
}

func getCurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Suppress unused import warnings.
var _ = json.Marshal
