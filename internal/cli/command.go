package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// NewExecCommand provides `lana exec [PROMPT]`, a noninteractive
// conversation runner. JSONL is suitable for scripts; default output is a
// plain text stream with no terminal decoration on stdout.
func NewExecCommand(runtime func(*cobra.Command) (*Runtime, error)) *cobra.Command {
	var model, sessionID string
	var jsonl bool
	cmd := &cobra.Command{
		Use:   "exec [PROMPT]",
		Short: "Run one noninteractive agent prompt",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := runtime(cmd)
			if err != nil {
				return err
			}
			if err := r.Ready(); err != nil {
				return err
			}
			if model != "" {
				r.Model = model
			}
			if sessionID != "" {
				if err := r.Resume(cmd.Context(), sessionID); err != nil {
					return fmt.Errorf("resume session: %w", err)
				}
			}
			prompt, err := PromptFromArgsOrInput(args, cmd.InOrStdin())
			if err != nil {
				return err
			}
			var sink EventSink
			if jsonl {
				sink = JSONLRenderer{Writer: cmd.OutOrStdout()}
			} else {
				sink = PlainRenderer{Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()}
			}
			_, err = r.Send(cmd.Context(), prompt, sink)
			if !jsonl {
				_, _ = io.WriteString(cmd.OutOrStdout(), "\n")
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&jsonl, "jsonl", false, "Write provider events as JSON Lines")
	cmd.Flags().StringVarP(&model, "model", "m", "", "Model identifier")
	cmd.Flags().StringVar(&sessionID, "resume", "", "Resume a stored session")
	return cmd
}

// PromptFromArgsOrInput reads a prompt from command arguments, or from stdin
// when no argument was supplied. It is shared with the root non-TTY fallback.
func PromptFromArgsOrInput(args []string, input io.Reader) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	if input == nil {
		return "", fmt.Errorf("prompt is required")
	}
	data, err := io.ReadAll(bufio.NewReader(input))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", fmt.Errorf("prompt is required")
	}
	return string(data), nil
}

// RunPlain is used by the top-level non-TTY fallback.
func RunPlain(ctx context.Context, runtime *Runtime, prompt string, stdout, stderr io.Writer) error {
	_, err := runtime.Send(ctx, prompt, PlainRenderer{Stdout: stdout, Stderr: stderr})
	if err == nil {
		_, _ = io.WriteString(stdout, "\n")
	}
	return err
}
