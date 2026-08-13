package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/deagy/lana/internal/approval"
	"github.com/deagy/lana/internal/execution"
	"github.com/deagy/lana/internal/output"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/storage"
	"github.com/deagy/lana/internal/tools"
)

// NonInteractiveRunner executes an agent prompt and streams structured output.
type NonInteractiveRunner struct {
	sessionID string
	store     *storage.FileStore
	client    provider.Client
	registry  *tools.Registry
	policy    approval.Policy
	formatter output.Formatter
	maxTurns  int
}

// NewNonInteractiveRunner creates a new runner.
func NewNonInteractiveRunner(
	sessionID string,
	store *storage.FileStore,
	client provider.Client,
	registry *tools.Registry,
	policy approval.Policy,
	formatter output.Formatter,
	maxTurns int,
) *NonInteractiveRunner {
	return &NonInteractiveRunner{
		sessionID: sessionID,
		store:     store,
		client:    client,
		registry:  registry,
		policy:    policy,
		formatter: formatter,
		maxTurns:  maxTurns,
	}
}

// Run executes the prompt and streams results.
func (r *NonInteractiveRunner) Run(ctx context.Context, prompt string) error {
	// Get current session
	sess, err := r.store.Get(ctx, r.sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	// Add user message
	sess.Transcript = append(sess.Transcript, session.Message{
		Role:      "user",
		Content:   prompt,
		Timestamp: time.Now(),
	})

	// Build request with tool schemas
	toolSchemas := make([]provider.Tool, 0)
	for _, tool := range r.registry.List() {
		toolSchemas = append(toolSchemas, provider.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}

	req := &provider.Request{
		Model:    sess.Model,
		Messages: toProviderMessages(sess.Transcript),
		Tools:    toolSchemas,
	}

	// Stream from provider
	stream, err := r.client.Chat(ctx, req)
	if err != nil {
		result := output.Result{
			Status:    "error",
			Error:     fmt.Sprintf("provider error: %v", err),
			Timestamp: time.Now().Unix(),
		}
		r.outputResult(result)
		return err
	}
	defer stream.Close()

	// Process events
	var assistantContent string
	var toolCalls []session.ToolCall
	messageStart := false

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("cancelled: %w", ctx.Err())
		default:
		}

		event, err := stream.NextEvent(ctx)
		if err != nil {
			// EOF is normal end of stream, not an error
			if err == io.EOF {
				break
			}
			result := output.Result{
				Status:    "error",
				Error:     fmt.Sprintf("stream error: %v", err),
				Timestamp: time.Now().Unix(),
			}
			r.outputResult(result)
			return err
		}

		if event == nil {
			break
		}

		// Handle message events
		switch e := event.(type) {
		case *provider.MessageStartEvent:
			messageStart = true
			result := output.Result{
				Status:    "message_start",
				Message:   "",
				Timestamp: time.Now().Unix(),
			}
			r.outputResult(result)

		case *provider.MessageDeltaEvent:
			assistantContent += e.Content
			result := output.Result{
				Status:    "message_delta",
				Message:   e.Content,
				Timestamp: time.Now().Unix(),
			}
			r.outputResult(result)

		case *provider.ToolCallEvent:
			// Execute tool
			result, err := r.executeTool(ctx, e)
			if err != nil {
				// Emit error but continue
				errResult := output.Result{
					Status:    "tool_error",
					ToolName:  e.Name,
					Error:     err.Error(),
					Timestamp: time.Now().Unix(),
				}
				r.outputResult(errResult)

				// Add failed tool call to transcript
				toolCalls = append(toolCalls, session.ToolCall{
					ID:     e.ID,
					Name:   e.Name,
					Input:  e.Input,
					Status: "failed",
					Error:  err.Error(),
				})
			} else {
				// Emit success
				toolCalls = append(toolCalls, session.ToolCall{
					ID:     e.ID,
					Name:   e.Name,
					Input:  e.Input,
					Result: result,
					Status: "completed",
				})

				resultOutput := output.Result{
					Status:     "tool_result",
					ToolName:   e.Name,
					ToolOutput: result,
					Timestamp:  time.Now().Unix(),
				}
				r.outputResult(resultOutput)
			}

		case *provider.MessageEndEvent:
			// Message complete
			result := output.Result{
				Status:    "message_end",
				Message:   assistantContent,
				Timestamp: time.Now().Unix(),
			}
			r.outputResult(result)

		case *provider.ErrorEvent:
			errResult := output.Result{
				Status:    "error",
				Error:     e.Err.Error(),
				Timestamp: time.Now().Unix(),
			}
			r.outputResult(errResult)
			return e.Err
		}
	}

	// Save message to session
	if messageStart {
		sess.Transcript = append(sess.Transcript, session.Message{
			Role:      "assistant",
			Content:   assistantContent,
			ToolCalls: toolCalls,
			Timestamp: time.Now(),
		})
		sess.UpdatedAt = time.Now()

		if err := r.store.Save(ctx, r.sessionID, sess); err != nil {
			return fmt.Errorf("save session: %w", err)
		}
	}

	return nil
}

// executeTool runs a tool with approval checks.
func (r *NonInteractiveRunner) executeTool(ctx context.Context, e *provider.ToolCallEvent) (string, error) {
	// Create executor (no broker for non-interactive; policy decides)
	executor := execution.NewExecutor(r.registry, r.policy, nil)

	// Execute (approval handled by policy/executor)
	result, err := executor.Execute(ctx, r.sessionID, e.Name, e.Input)
	if err != nil {
		return "", err
	}

	return result.Output, nil
}

// outputResult formats and prints a result.
func (r *NonInteractiveRunner) outputResult(result output.Result) error {
	text, err := r.formatter.FormatResult(result)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, text)
	return nil
}

// toProviderMessages converts session messages to provider format.
func toProviderMessages(msgs []session.Message) []provider.Message {
	var result []provider.Message
	for _, m := range msgs {
		result = append(result, provider.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return result
}
