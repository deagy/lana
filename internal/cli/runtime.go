// Package cli provides provider-neutral conversation execution and renderers
// shared by the interactive terminal and the noninteractive command surface.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/deagy/lana/internal/agent"
	"github.com/deagy/lana/internal/policy"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/tools"
)

// EventSink receives provider events while a turn is in progress.
type EventSink interface {
	Emit(context.Context, provider.Event) error
}

type EventSinkFunc func(context.Context, provider.Event) error

func (f EventSinkFunc) Emit(ctx context.Context, event provider.Event) error { return f(ctx, event) }

// TurnExecutor is the narrow dependency required by the presentation layer.
// Provider credentials, authorization policy, and tool implementations stay
// outside the CLI and are injected by the embedding application.
type TurnExecutor interface {
	Run(context.Context, provider.Request, EventSink) (agent.TurnResult, error)
}

// ReadyExecutor optionally validates configuration before a terminal is opened
// or a session file is created.
type ReadyExecutor interface {
	TurnExecutor
	Ready() error
}

type TurnExecutorFunc func(context.Context, provider.Request, EventSink) (agent.TurnResult, error)

func (f TurnExecutorFunc) Run(ctx context.Context, request provider.Request, sink EventSink) (agent.TurnResult, error) {
	return f(ctx, request, sink)
}

// ApprovalRequest represents one tool authorization decision. Presentation
// code chooses Allow or Deny; no policy or credential is embedded in the UI.
type ApprovalRequest struct {
	Call     tools.Call
	response chan error
}

// ApprovalPreview is the safe, human-readable representation of a pending
// call. It deliberately omits write/patch bodies while retaining a bounded
// change summary, so an approval screen does not become a second secret or
// source-content disclosure channel.
type ApprovalPreview struct {
	Tool        string
	Arguments   string
	Scope       string
	DiffSummary string
}

// Preview returns redacted, bounded call details appropriate for an approval
// surface. It never returns the raw tool arguments.
func (r *ApprovalRequest) Preview() ApprovalPreview {
	if r == nil {
		return ApprovalPreview{Tool: "unknown", Arguments: `"[REDACTED missing call]"`, Scope: "unspecified", DiffSummary: "no change summary available"}
	}
	return previewCall(r.Call)
}

func previewCall(call tools.Call) ApprovalPreview {
	tool := strings.TrimSpace(call.Name)
	if tool == "" {
		tool = "unknown"
	}
	preview := ApprovalPreview{Tool: tool, Scope: "unspecified", DiffSummary: "no textual change supplied"}
	redacted := provider.RedactJSON(call.Arguments)
	var arguments map[string]any
	if json.Unmarshal(redacted, &arguments) != nil {
		preview.Arguments = `"[REDACTED invalid payload]"`
		return preview
	}
	preview.Scope = approvalScope(arguments)
	preview.DiffSummary = approvalDiffSummary(arguments)
	encoded, _ := json.MarshalIndent(compactApprovalArguments(arguments), "", "  ")
	preview.Arguments = string(encoded)
	if preview.Arguments == "" {
		preview.Arguments = "{}"
	}
	return preview
}

func compactApprovalArguments(arguments map[string]any) map[string]any {
	result := make(map[string]any, len(arguments))
	for key, value := range arguments {
		result[key] = compactApprovalValue(key, value)
	}
	return result
}

func compactApprovalValue(key string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return compactApprovalArguments(typed)
	case []any:
		result := make([]any, len(typed))
		for i, child := range typed {
			result[i] = compactApprovalValue(key, child)
		}
		return result
	case string:
		value := redactApprovalText(typed)
		if key == "content" || key == "patch" || key == "diff" {
			return fmt.Sprintf("[omitted: %d bytes]", len(value))
		}
		if len(value) > 512 {
			return fmt.Sprintf("[truncated: %d bytes]", len(value))
		}
		return value
	default:
		return value
	}
}

// redactApprovalText adds common command-line secret forms to the structured
// provider redaction. Tool calls can carry a shell command, where a secret is
// often separated from its flag instead of encoded as key=value JSON.
func redactApprovalText(value string) string {
	redacted := provider.Redact(value)
	if redacted != value {
		return redacted
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"--token", "--api-key", "--apikey", "--password", "--secret", "--credential", "authorization "} {
		if strings.Contains(lower, marker) {
			return "[REDACTED]"
		}
	}
	return value
}

func approvalScope(arguments map[string]any) string {
	values := make([]string, 0, 4)
	for _, key := range []string{"workspace", "workdir", "cwd", "path", "source", "destination", "target", "file", "uri"} {
		value, ok := arguments[key]
		if !ok {
			continue
		}
		encoded, err := json.Marshal(compactApprovalValue(key, value))
		if err == nil {
			values = append(values, key+"="+string(encoded))
		}
	}
	if len(values) == 0 {
		return "unspecified"
	}
	return strings.Join(values, ", ")
}

func approvalDiffSummary(arguments map[string]any) string {
	for _, key := range []string{"patch", "diff"} {
		if value, ok := arguments[key].(string); ok {
			added, removed := 0, 0
			for _, line := range strings.Split(value, "\n") {
				if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
					continue
				}
				if strings.HasPrefix(line, "+") {
					added++
				} else if strings.HasPrefix(line, "-") {
					removed++
				}
			}
			return fmt.Sprintf("%s supplied: %d addition(s), %d removal(s), %d bytes", key, added, removed, len(value))
		}
	}
	if value, ok := arguments["content"].(string); ok {
		return fmt.Sprintf("replacement content supplied: %d bytes (full diff unavailable)", len(value))
	}
	return "no textual change supplied"
}

func (r *ApprovalRequest) Allow() { r.respond(nil) }
func (r *ApprovalRequest) Deny()  { r.respond(errors.New("tool call denied by user")) }
func (r *ApprovalRequest) respond(err error) {
	select {
	case r.response <- err:
	default:
	}
}

// ApprovalBroker is a tools.Authorizer which pauses a turn until an injected
// presentation layer decides whether to permit the requested tool call.
type ApprovalBroker struct{ requests chan *ApprovalRequest }

func NewApprovalBroker() *ApprovalBroker {
	return &ApprovalBroker{requests: make(chan *ApprovalRequest)}
}
func (b *ApprovalBroker) Requests() <-chan *ApprovalRequest { return b.requests }
func (b *ApprovalBroker) Wait(ctx context.Context) (*ApprovalRequest, error) {
	if b == nil {
		return nil, errors.New("approval broker is required")
	}
	select {
	case request := <-b.requests:
		return request, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (b *ApprovalBroker) Authorize(ctx context.Context, call tools.Call) error {
	if b == nil {
		return errors.New("approval broker is required")
	}
	request := &ApprovalRequest{Call: call, response: make(chan error, 1)}
	select {
	case b.requests <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.response:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ApprovalBroker returns b so a composed runtime can prove that the broker
// displayed by the TUI is the same object making authorization decisions.
func (b *ApprovalBroker) ApprovalBroker() *ApprovalBroker { return b }

// AuthorizationController is an injected, mode-aware authorization policy.
// SetMode must affect future authorization decisions; a UI must not treat it
// as an independent preference store.
type AuthorizationController interface {
	policy.ModeController
}

// AuthorizationControllerProvider is implemented by an executor that can
// expose its actual authorization control path to presentation code.
type AuthorizationControllerProvider interface {
	AuthorizationController() AuthorizationController
}

// ApprovalBrokerProvider is a deliberately narrow identity seam for hosts
// which use the interactive broker as their actual tools.Authorizer.
type ApprovalBrokerProvider interface {
	ApprovalBroker() *ApprovalBroker
}

// Kernel adapts the agent turn kernel to TurnExecutor without exposing any
// provider-specific configuration to commands or the terminal UI.
type Kernel struct {
	Provider      provider.Client
	Authorizer    tools.Authorizer
	Executor      tools.Executor
	MaxToolRounds int
}

func (k Kernel) Ready() error {
	if k.Provider == nil {
		return errors.New("conversational provider is not configured")
	}
	if k.Authorizer == nil {
		return errors.New("conversational tool authorizer is not configured")
	}
	if k.Executor == nil {
		return errors.New("conversational tool executor is not configured")
	}
	return nil
}

func (k Kernel) Run(ctx context.Context, request provider.Request, sink EventSink) (agent.TurnResult, error) {
	runner := agent.TurnRunner{
		Provider: k.Provider, Authorizer: k.Authorizer, Executor: k.Executor,
		MaxToolRounds: k.MaxToolRounds,
	}
	if sink != nil {
		runner.Sink = agent.EventSinkFunc(sink.Emit)
	}
	return runner.Run(ctx, request)
}

// AuthorizationController exposes an opt-in controller implemented by the
// injected authorizer. This makes mode selection part of the authorization
// path, not CLI metadata.
func (k Kernel) AuthorizationController() AuthorizationController {
	controller, _ := k.Authorizer.(AuthorizationController)
	return controller
}

// ApprovalBroker exposes the concrete interactive authorization path when the
// injected authorizer is an ApprovalBroker.
func (k Kernel) ApprovalBroker() *ApprovalBroker {
	broker, _ := k.Authorizer.(*ApprovalBroker)
	return broker
}

// UnavailableExecutor lets a binary retain all existing one-shot commands
// when no conversational provider was configured by its host.
type UnavailableExecutor struct{}

func (UnavailableExecutor) Ready() error {
	return errors.New("no conversational runtime is configured; configure a provider, tool authorizer, and tool executor, or inject cli.TurnExecutor")
}
func (UnavailableExecutor) Run(context.Context, provider.Request, EventSink) (agent.TurnResult, error) {
	return agent.TurnResult{}, UnavailableExecutor{}.Ready()
}

// SessionStore is satisfied by session.Store and makes conversations easy to
// test without binding a presenter to filesystem storage.
type SessionStore interface {
	Create(context.Context, string, map[string]string) (session.Session, error)
	Append(context.Context, string, string, any) (session.Record, error)
	Load(context.Context, string) (session.Session, error)
	Recover(context.Context, string) (session.Session, error)
	List(context.Context) ([]session.Summary, error)
	Fork(context.Context, string, string) (session.Session, error)
}

// Runtime keeps the in-memory transcript for one active conversation.
// Persisted records are append-only and use the session package's versioned
// envelope rather than a second TUI-specific file format.
type Runtime struct {
	Executor    TurnExecutor
	Sessions    SessionStore
	SessionID   string
	Model       string
	Permissions string
	Tools       []provider.ToolDefinition

	messages []provider.Message
	newID    func() string
	mu       sync.Mutex
}

type Options struct {
	Executor     TurnExecutor
	Sessions     SessionStore
	SessionID    string
	Model        string
	Permissions  string
	Tools        []provider.ToolDefinition
	NewSessionID func() string
}

func NewRuntime(opts Options) *Runtime {
	if opts.Executor == nil {
		opts.Executor = UnavailableExecutor{}
	}
	if opts.NewSessionID == nil {
		opts.NewSessionID = func() string { return fmt.Sprintf("session-%d", time.Now().UTC().UnixNano()) }
	}
	permissions := ""
	if mode, err := policy.ParseMode(opts.Permissions); err == nil {
		permissions = string(mode)
	}
	runtime := &Runtime{Executor: opts.Executor, Sessions: opts.Sessions, SessionID: opts.SessionID, Model: opts.Model, Permissions: permissions, Tools: append([]provider.ToolDefinition(nil), opts.Tools...), newID: opts.NewSessionID}
	if controller, ok := runtime.AuthorizationController(); ok {
		runtime.Permissions = string(controller.Mode())
	}
	return runtime
}

// AuthorizationController returns the controller from the executor that will
// make tool authorization decisions. A missing controller means permissions
// cannot safely be changed from the terminal UI.
func (r *Runtime) AuthorizationController() (AuthorizationController, bool) {
	if r == nil || r.Executor == nil {
		return nil, false
	}
	provider, ok := r.Executor.(AuthorizationControllerProvider)
	if !ok || provider.AuthorizationController() == nil {
		return nil, false
	}
	return provider.AuthorizationController(), true
}

// SetPermissionMode applies a validated policy mode to the injected
// authorization controller, then records the effective mode for session
// metadata and rendering.
func (r *Runtime) SetPermissionMode(mode policy.Mode) error {
	mode, err := policy.ParseMode(string(mode))
	if err != nil {
		return err
	}
	controller, ok := r.AuthorizationController()
	if !ok {
		return errors.New("authorization policy mode cannot be changed: no mode-aware authorizer is configured")
	}
	if err := controller.SetMode(mode); err != nil {
		return fmt.Errorf("set authorization policy mode: %w", err)
	}
	effective := controller.Mode()
	if _, err := policy.ParseMode(string(effective)); err != nil {
		return fmt.Errorf("authorization controller returned invalid policy mode %q", effective)
	}
	r.mu.Lock()
	r.Permissions = string(effective)
	r.mu.Unlock()
	return nil
}

// ApprovalBroker returns the broker used by the runtime's authorization path.
// The boolean distinguishes an unsupported executor from a nil broker.
func (r *Runtime) ApprovalBroker() (*ApprovalBroker, bool) {
	if r == nil || r.Executor == nil {
		return nil, false
	}
	provider, ok := r.Executor.(ApprovalBrokerProvider)
	if !ok {
		return nil, false
	}
	broker := provider.ApprovalBroker()
	return broker, broker != nil
}

// UsesApprovalBroker reports whether broker is the exact authorization object
// used by the injected executor. It gives embedding applications a testable
// composition check without exposing provider or credential details.
func (r *Runtime) UsesApprovalBroker(broker *ApprovalBroker) bool {
	configured, ok := r.ApprovalBroker()
	return ok && configured == broker
}

// Ready makes configuration failures actionable before a UI starts.
func (r *Runtime) Ready() error {
	if r == nil || r.Executor == nil {
		return UnavailableExecutor{}.Ready()
	}
	if executor, ok := r.Executor.(ReadyExecutor); ok {
		return executor.Ready()
	}
	return nil
}

// Start creates a durable session on first use. Supplying no store is valid:
// the conversation remains in-memory, useful for automation and test fakes.
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startLocked(ctx)
}

func (r *Runtime) startLocked(ctx context.Context) error {
	if r.SessionID == "" {
		r.SessionID = r.newID()
	}
	if r.Sessions == nil {
		return nil
	}
	loaded, err := r.Sessions.Load(ctx, r.SessionID)
	if err == nil {
		if loaded.Recovered {
			_, err = r.Sessions.Recover(ctx, r.SessionID)
		}
		return err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err = r.Sessions.Create(ctx, r.SessionID, map[string]string{"model": r.Model, "permissions": r.Permissions})
	return err
}

// Recover explicitly discards an incomplete final JSONL record. It is safe to
// call on a clean session and is the prerequisite used by Resume and Fork.
func (r *Runtime) Recover(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Sessions == nil {
		return errors.New("session storage is not configured")
	}
	_, err := r.Sessions.Recover(ctx, id)
	return err
}

// Fork creates a durable child after explicitly recovering the parent. It
// does not switch the active runtime; callers can Resume the child when ready.
func (r *Runtime) Fork(ctx context.Context, parentID, childID string) (session.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Sessions == nil {
		return session.Session{}, errors.New("session storage is not configured")
	}
	if _, err := r.Sessions.Recover(ctx, parentID); err != nil {
		return session.Session{}, err
	}
	return r.Sessions.Fork(ctx, parentID, childID)
}

// Resume explicitly recovers a possible torn final record, then reconstructs
// portable user, assistant, and tool-result history for a continued turn.
func (r *Runtime) Resume(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Sessions == nil {
		return errors.New("session storage is not configured")
	}
	loaded, err := r.Sessions.Recover(ctx, id)
	if err != nil {
		return err
	}
	r.SessionID = loaded.ID
	r.messages = r.messages[:0]
	for _, record := range loaded.Records {
		switch record.Kind {
		case "message.user", "message.assistant":
			var message provider.Message
			if json.Unmarshal(record.Data, &message) == nil && message.Role != "" {
				r.messages = append(r.messages, message)
			}
		case "tool.result":
			var result tools.Result
			if json.Unmarshal(record.Data, &result) == nil {
				result = tools.SanitizeResult(result)
				if result.Validate() == nil {
					data, _ := json.Marshal(result)
					r.messages = append(r.messages, provider.Message{Role: provider.RoleTool, Name: result.Name, ToolCallID: result.CallID, Data: data})
				}
			}
		}
	}
	return nil
}

func (r *Runtime) New(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.SessionID = ""
	r.messages = nil
	return r.startLocked(ctx)
}

// Send streams one prompt and appends the user message, provider events, and
// assembled assistant text to session storage. It returns on cancellation.
func (r *Runtime) Send(ctx context.Context, prompt string, sink EventSink) (agent.TurnResult, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return agent.TurnResult{}, errors.New("prompt is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.startLocked(ctx); err != nil {
		return agent.TurnResult{}, err
	}
	user := provider.Message{Role: provider.RoleUser, Content: prompt}
	r.messages = append(r.messages, user)
	request := provider.Request{Model: r.Model, Messages: append([]provider.Message(nil), r.messages...), Tools: append([]provider.ToolDefinition(nil), r.Tools...)}
	sessionID := r.SessionID
	persistCtx := context.WithoutCancel(ctx)
	if r.Sessions != nil {
		if _, err := r.Sessions.Append(ctx, sessionID, "message.user", user); err != nil {
			return agent.TurnResult{}, err
		}
	}

	var assistant strings.Builder
	wrapped := EventSinkFunc(func(eventCtx context.Context, event provider.Event) error {
		event = provider.SanitizeEvent(event)
		if err := event.Validate(); err != nil {
			return err
		}
		if r.Sessions != nil {
			if _, err := r.Sessions.Append(eventCtx, sessionID, "provider.event", event); err != nil {
				return err
			}
		}
		if event.Type == provider.EventTextDelta {
			assistant.WriteString(EventText(event))
		}
		if sink != nil {
			return sink.Emit(eventCtx, event)
		}
		return nil
	})
	result, err := r.Executor.Run(ctx, request, wrapped)
	if r.Sessions != nil {
		for _, toolResult := range result.ToolResults {
			toolResult = tools.SanitizeResult(toolResult)
			if validateErr := toolResult.Validate(); validateErr != nil {
				if err == nil {
					err = fmt.Errorf("invalid tool result for persistence: %w", validateErr)
				}
				continue
			}
			if _, appendErr := r.Sessions.Append(persistCtx, sessionID, "tool.result", toolResult); err == nil && appendErr != nil {
				err = appendErr
			}
		}
	}
	if text := assistant.String(); text != "" {
		message := provider.Message{Role: provider.RoleAssistant, Content: text}
		r.messages = append(r.messages, message)
		if r.Sessions != nil {
			_, appendErr := r.Sessions.Append(persistCtx, sessionID, "message.assistant", message)
			if err == nil && appendErr != nil {
				err = appendErr
			}
		}
	}
	if r.Sessions != nil {
		outcome := TurnOutcome{Status: turnStatus(ctx, err), Rounds: result.Rounds, Completed: result.Completed}
		if err != nil {
			outcome.Error = provider.Redact(err.Error())
		}
		if _, appendErr := r.Sessions.Append(persistCtx, sessionID, "turn.outcome", outcome); err == nil && appendErr != nil {
			err = appendErr
		}
	}
	return result, err
}

// TurnOutcome is stored for every turn, including failed or cancelled turns,
// so resumed sessions do not silently lose the terminal state of a request.
type TurnOutcome struct {
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	Rounds    int    `json:"rounds"`
	Completed bool   `json:"completed"`
}

func turnStatus(ctx context.Context, err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return "cancelled"
	}
	if err != nil {
		return "failed"
	}
	return "completed"
}

// EventText extracts the user-visible text field from the portable event
// envelope. It deliberately accepts common provider adapter shapes.
func EventText(event provider.Event) string {
	if len(event.Data) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(event.Data, &text) == nil {
		return text
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(event.Data, &fields) != nil {
		return ""
	}
	for _, key := range []string{"text", "content", "delta"} {
		if json.Unmarshal(fields[key], &text) == nil {
			return text
		}
	}
	return ""
}

// PlainRenderer keeps stdout protocol-safe: assistant text only, never color,
// status lines, or tool decorations. Tool activity is sent to stderr.
type PlainRenderer struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (p PlainRenderer) Emit(_ context.Context, event provider.Event) error {
	if p.Stdout == nil {
		p.Stdout = io.Discard
	}
	if p.Stderr == nil {
		p.Stderr = io.Discard
	}
	event = provider.SanitizeEvent(event)
	switch event.Type {
	case provider.EventTextDelta:
		_, err := io.WriteString(p.Stdout, EventText(event))
		return err
	case provider.EventToolCall:
		var call tools.Call
		if json.Unmarshal(event.Data, &call) == nil {
			_, err := fmt.Fprintf(p.Stderr, "tool: %s\n", call.Name)
			return err
		}
	}
	return nil
}

// JSONLRenderer writes one exact provider Event envelope per line to stdout.
// It intentionally does not write headers, status, or ANSI sequences.
type JSONLRenderer struct{ Writer io.Writer }

func (j JSONLRenderer) Emit(_ context.Context, event provider.Event) error {
	if j.Writer == nil {
		return nil
	}
	event = provider.SanitizeEvent(event)
	if err := event.Validate(); err != nil {
		return err
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(j.Writer, "%s\n", encoded)
	return err
}
