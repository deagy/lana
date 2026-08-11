// Package file provides policy-enforced file operation subcommands.
package file

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/deagy/lana/internal/app"
	"github.com/deagy/lana/internal/policy"
	"github.com/spf13/cobra"
)

const DefaultReadLimit int64 = 1 << 20

var ErrReadLimit = errors.New("file exceeds read limit")

// NewCommand creates the file command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "file", Short: "Policy-enforced file operations"}
	cmd.PersistentFlags().String("sandbox", "", "Policy mode override (unrestricted, workspace-write, workspace-read-only)")
	cmd.AddCommand(fileReadCommand())
	cmd.AddCommand(fileWriteCommand())
	cmd.AddCommand(fileDeleteCommand())
	cmd.AddCommand(fileCopyCommand())
	cmd.AddCommand(fileMoveCommand())
	cmd.AddCommand(fileSearchCommand())
	cmd.AddCommand(fileInfoCommand())
	return cmd
}

func fileReadCommand() *cobra.Command {
	var lineRange string
	var headN int
	var showCount bool
	var maxBytes int64
	cmd := &cobra.Command{Use: "read [flags] <path>", Short: "Read bounded file contents", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := commandPolicy(cmd, policy.ModeWorkspaceReadOnly)
		if err != nil {
			return err
		}
		evaluation, err := p.Enforce(policy.OperationRead, args[0], false)
		if err != nil {
			return fmt.Errorf("read policy: %w", err)
		}
		data, err := readBounded(evaluation.CanonicalPath, maxBytes)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("file not found: %s", args[0])
			}
			return fmt.Errorf("read file: %w", err)
		}
		content := selectLines(string(data), lineRange, headN)
		fmt.Fprint(cmd.OutOrStdout(), content)
		if showCount {
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d lines\n", lineCount(content))
		}
		return nil
	}}
	cmd.Flags().StringVarP(&lineRange, "lines", "l", "", "Line range (e.g., '1-10' or '50-')")
	cmd.Flags().IntVarP(&headN, "head", "n", 0, "Show first N lines")
	cmd.Flags().BoolVarP(&showCount, "count", "c", false, "Show line count")
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", DefaultReadLimit, "Maximum bytes to read")
	return cmd
}

func fileWriteCommand() *cobra.Command {
	var content string
	var backup, atomic bool
	var maxBytes int64
	cmd := &cobra.Command{Use: "write [flags] <path>", Short: "Write file contents", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := commandPolicy(cmd, policy.ModeWorkspaceWrite)
		if err != nil {
			return err
		}
		evaluation, err := p.Enforce(policy.OperationWrite, args[0], false)
		if err != nil {
			return fmt.Errorf("write policy: %w", err)
		}
		if content == "" {
			stat, _ := os.Stdin.Stat()
			if stat == nil || stat.Mode()&os.ModeCharDevice != 0 {
				return fmt.Errorf("provide content via --content flag or stdin")
			}
			data, err := readReaderBounded(os.Stdin, maxBytes)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			content = string(data)
		}
		path := evaluation.CanonicalPath
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
		if backup {
			backupPath := path + ".bak"
			if _, err := p.Enforce(policy.OperationWrite, backupPath, false); err != nil {
				return fmt.Errorf("backup policy: %w", err)
			}
			if data, err := readBounded(path, maxBytes); err == nil {
				if err := os.WriteFile(backupPath, data, 0644); err != nil {
					return fmt.Errorf("write backup: %w", err)
				}
			}
		}
		if atomic {
			tmp := path + ".tmp"
			if _, err := p.Enforce(policy.OperationWrite, tmp, false); err != nil {
				return fmt.Errorf("temporary file policy: %w", err)
			}
			if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
				return fmt.Errorf("write temporary file: %w", err)
			}
			if err := os.Rename(tmp, path); err != nil {
				_ = os.Remove(tmp)
				return fmt.Errorf("replace file: %w", err)
			}
		} else if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d bytes to %s\n", len(content), args[0])
		return nil
	}}
	cmd.Flags().StringVarP(&content, "content", "c", "", "Content to write")
	cmd.Flags().BoolVarP(&backup, "backup", "b", false, "Create backup before overwriting")
	cmd.Flags().BoolVarP(&atomic, "atomic", "a", false, "Atomic write")
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", DefaultReadLimit, "Maximum stdin or backup bytes")
	return cmd
}

func fileDeleteCommand() *cobra.Command {
	var force, approve bool
	cmd := &cobra.Command{Use: "delete <path>", Short: "Delete a file", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := commandPolicy(cmd, policy.ModeWorkspaceWrite)
		if err != nil {
			return err
		}
		_, err = p.Enforce(policy.OperationDelete, args[0], approve || force)
		if err != nil {
			return fmt.Errorf("delete policy: %w", err)
		}
		info, err := os.Lstat(lexicalPath(p, args[0]))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("file not found: %s", args[0])
			}
			return fmt.Errorf("stat file: %w", err)
		}
		if info.IsDir() && !force {
			return fmt.Errorf("path is a directory; use --force to delete directories")
		}
		if err := os.Remove(lexicalPath(p, args[0])); err != nil {
			return fmt.Errorf("delete file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted: %s\n", args[0])
		return nil
	}}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Explicitly approve deletion (required for directories)")
	cmd.Flags().BoolVar(&approve, "approve", false, "Explicitly approve this high-risk deletion")
	return cmd
}

func fileCopyCommand() *cobra.Command { return transferCommand("copy", false) }
func fileMoveCommand() *cobra.Command { return transferCommand("move", true) }

func transferCommand(name string, move bool) *cobra.Command {
	var maxBytes int64
	var approve bool
	cmd := &cobra.Command{Use: name + " <source> <destination>", Short: strings.Title(name) + " a file", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := commandPolicy(cmd, policy.ModeWorkspaceWrite)
		if err != nil {
			return err
		}
		source, err := p.Enforce(policy.OperationRead, args[0], false)
		if err != nil {
			return fmt.Errorf("source policy: %w", err)
		}
		destination, err := p.Enforce(policy.OperationWrite, args[1], false)
		if err != nil {
			return fmt.Errorf("destination policy: %w", err)
		}
		if move {
			if _, err := p.Enforce(policy.OperationDelete, args[0], approve); err != nil {
				return fmt.Errorf("source delete policy: %w", err)
			}
		}
		data, err := readBounded(source.CanonicalPath, maxBytes)
		if err != nil {
			return fmt.Errorf("read source: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(destination.CanonicalPath), 0755); err != nil {
			return fmt.Errorf("create destination directory: %w", err)
		}
		if err := os.WriteFile(destination.CanonicalPath, data, 0644); err != nil {
			return fmt.Errorf("write destination: %w", err)
		}
		if move {
			if err := os.Remove(lexicalPath(p, args[0])); err != nil {
				_ = os.Remove(destination.CanonicalPath)
				return fmt.Errorf("delete source: %w", err)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s -> %s\n", strings.Title(name), args[0], args[1])
		return nil
	}}
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", DefaultReadLimit, "Maximum source bytes")
	if move {
		cmd.Flags().BoolVar(&approve, "approve", false, "Explicitly approve deletion of the source")
	}
	return cmd
}

func fileSearchCommand() *cobra.Command {
	var maxResults int
	var includeExt string
	var caseSensitive bool
	cmd := &cobra.Command{Use: "search [flags] <pattern>", Short: "Search workspace files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := commandPolicy(cmd, policy.ModeWorkspaceReadOnly)
		if err != nil {
			return err
		}
		if _, err := p.Enforce(policy.OperationSearch, p.Workspace(), false); err != nil {
			return fmt.Errorf("search policy: %w", err)
		}
		if maxResults <= 0 {
			maxResults = 100
		}
		pattern := args[0]
		if !caseSensitive {
			pattern = strings.ToLower(pattern)
		}
		matches := make([]string, 0)
		err = filepath.Walk(p.Workspace(), func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info.IsDir() {
				return nil
			}
			name := info.Name()
			if !caseSensitive {
				name = strings.ToLower(name)
			}
			matched, matchErr := filepath.Match(pattern, name)
			if matchErr != nil || !matched {
				return nil
			}
			if includeExt != "" && !strings.HasSuffix(info.Name(), includeExt) {
				return nil
			}
			rel, relErr := filepath.Rel(p.Workspace(), path)
			if relErr == nil {
				matches = append(matches, rel)
			}
			if len(matches) >= maxResults {
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
		if len(matches) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No files found.")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Found %d files matching %q:\n", len(matches), args[0])
		for _, match := range matches {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", match)
		}
		return nil
	}}
	cmd.Flags().IntVarP(&maxResults, "max-results", "m", 100, "Maximum results")
	cmd.Flags().StringVarP(&includeExt, "include", "i", "", "File extension filter")
	cmd.Flags().BoolVarP(&caseSensitive, "case-sensitive", "C", false, "Case-sensitive matching")
	return cmd
}

func fileInfoCommand() *cobra.Command {
	return &cobra.Command{Use: "info <path>", Short: "Show file information", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, err := commandPolicy(cmd, policy.ModeWorkspaceReadOnly)
		if err != nil {
			return err
		}
		evaluation, err := p.Enforce(policy.OperationInfo, args[0], false)
		if err != nil {
			return fmt.Errorf("info policy: %w", err)
		}
		info, err := os.Stat(evaluation.CanonicalPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("file not found: %s", args[0])
			}
			return fmt.Errorf("stat file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Name:       %s\nSize:       %d bytes\nPermissions: %s\nModified:   %s\nIs Dir:     %v\n", info.Name(), info.Size(), info.Mode(), info.ModTime().Format("2006-01-02 15:04:05"), info.IsDir())
		return nil
	}}
}

func commandPolicy(cmd *cobra.Command, mode policy.Mode) (*policy.Policy, error) {
	workspace := ""
	if application, ok := app.FromContext(cmd.Context()); ok {
		resolved := application.Config()
		workspace = resolved.Workspace()
		mode = policy.Mode(resolved.Config().Exec.Sandbox)
	}
	if workspace == "" {
		workspace, _ = cmd.Flags().GetString("workspace")
	}
	if workspace == "" && cmd.Root() != nil {
		workspace, _ = cmd.Root().PersistentFlags().GetString("workspace")
	}
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get workspace: %w", err)
		}
	}
	if flag := cmd.Flags().Lookup("sandbox"); flag != nil && flag.Changed {
		configuredMode, _ := cmd.Flags().GetString("sandbox")
		mode = policy.Mode(configuredMode)
	}
	p, err := policy.New(policy.Options{Mode: mode, Workspace: workspace})
	if err != nil {
		return nil, fmt.Errorf("create file policy: %w", err)
	}
	return p, nil
}

// lexicalPath returns the requested path anchored to the workspace. It is used
// for unlinking so `file delete link` removes the link rather than its target.
// Policy validation happens first and resolves symlinks for containment.
func lexicalPath(p *policy.Policy, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(p.Workspace(), path)
}

func readBounded(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readReaderBounded(f, max)
}

func readReaderBounded(reader io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return nil, fmt.Errorf("max-bytes must be greater than zero")
	}
	data, err := io.ReadAll(io.LimitReader(reader, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrReadLimit
	}
	return data, nil
}

func selectLines(content, lineRange string, head int) string {
	if lineRange != "" {
		return applyLineRange(content, lineRange)
	}
	if head > 0 {
		lines := strings.Split(content, "\n")
		if head < len(lines) {
			lines = lines[:head]
		}
		return strings.Join(lines, "\n")
	}
	return content
}

func applyLineRange(content, rangeStr string) string {
	lines := strings.Split(content, "\n")
	start, end := 0, len(lines)
	if before, after, found := strings.Cut(rangeStr, "-"); found {
		if before != "" {
			value, err := strconv.Atoi(before)
			if err != nil || value < 1 {
				return ""
			}
			start = value - 1
		}
		if after != "" {
			value, err := strconv.Atoi(after)
			if err != nil || value < 1 {
				return ""
			}
			end = value
		}
	} else {
		value, err := strconv.Atoi(rangeStr)
		if err != nil || value < 1 || value > len(lines) {
			return content
		}
		return lines[value-1]
	}
	if start >= len(lines) || start >= end {
		return ""
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func lineCount(content string) int {
	if content == "" {
		return 0
	}
	return len(strings.Split(content, "\n"))
}
