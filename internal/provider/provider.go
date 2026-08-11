// Package provider defines the provider-neutral streaming contract used by an
// agent turn. Provider implementations must translate their native protocol to
// these values and must not expose credentials in events or errors.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const EventSchemaVersion = 1

// EventType identifies a provider-neutral, streaming event.
type EventType string

const (
	EventMessageStart EventType = "message.start"
	EventTextDelta    EventType = "message.delta"
	EventToolCall     EventType = "tool.call"
	EventMessageEnd   EventType = "message.end"
	EventError        EventType = "error"
)

// Event is the versioned envelope exchanged by providers, turn orchestration,
// and persistence. Data is deliberately opaque so its schema can evolve by
// EventType without breaking the envelope contract.
type Event struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id,omitempty"`
	TurnID        string    `json:"turn_id,omitempty"`
	Type          EventType `json:"type"`
	// ErrorCode is required for error events. It is a stable, non-secret
	// machine-readable category; diagnostic detail belongs in Data.
	ErrorCode string          `json:"error_code,omitempty"`
	At        time.Time       `json:"at"`
	Data      json.RawMessage `json:"data,omitempty"`
}

func NewEvent(eventType EventType, data any) (Event, error) {
	return NewEventAt(time.Now().UTC(), eventType, data)
}

// NewEventAt creates an event with a caller-supplied timestamp. Adapters that
// receive a provider timestamp, and deterministic tests, should use this
// constructor instead of relying on the local clock.
func NewEventAt(at time.Time, eventType EventType, data any) (Event, error) {
	if eventType == "" {
		return Event{}, errors.New("provider event type is required")
	}
	if at.IsZero() {
		return Event{}, errors.New("provider event timestamp is required")
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal provider event data: %w", err)
	}
	event := Event{SchemaVersion: EventSchemaVersion, Type: eventType, At: at.UTC(), Data: encoded}
	if eventType == EventError {
		event.ErrorCode = "provider_error"
	}
	return SanitizeEvent(event), nil
}

func (e Event) Validate() error {
	if e.SchemaVersion != EventSchemaVersion {
		return fmt.Errorf("unsupported provider event schema version %d", e.SchemaVersion)
	}
	if e.Type == "" {
		return errors.New("provider event type is required")
	}
	if e.At.IsZero() {
		return errors.New("provider event timestamp is required")
	}
	if len(e.Data) > 0 && !json.Valid(e.Data) {
		return errors.New("provider event data is not valid JSON")
	}
	if e.Type == EventError && e.ErrorCode == "" {
		return errors.New("provider error event code is required")
	}
	return nil
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role            `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// ToolDefinition mirrors the portable portion of JSON Schema and is kept here
// so a provider request need not depend on the tool execution package.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type Request struct {
	Model    string            `json:"model,omitempty"`
	Messages []Message         `json:"messages"`
	Tools    []ToolDefinition  `json:"tools,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Stream is pull based, making callers naturally backpressure providers and
// allowing cancellation to interrupt a blocked receive.
type Stream interface {
	Recv(context.Context) (Event, error)
	Close() error
}

// Client starts one model response. It may return a stream before any event is
// available. Implementations should honor ctx at both Stream and Recv time.
type Client interface {
	Stream(context.Context, Request) (Stream, error)
}

// SliceStream is a deterministic in-memory stream useful in tests and local
// adapters.
type SliceStream struct {
	events []Event
	next   int
	closed bool
}

func NewSliceStream(events ...Event) *SliceStream { return &SliceStream{events: events} }

func (s *SliceStream) Recv(ctx context.Context) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	if s.closed || s.next >= len(s.events) {
		return Event{}, io.EOF
	}
	event := s.events[s.next]
	s.next++
	return event, nil
}

func (s *SliceStream) Close() error { s.closed = true; return nil }

// StaticClient serves preconstructed streams. It is intentionally small and
// supports fakes without binding tests to a provider SDK.
type StaticClient struct {
	Streams  []Stream
	Requests []Request
}

func (c *StaticClient) Stream(ctx context.Context, request Request) (Stream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.Requests = append(c.Requests, request)
	if len(c.Streams) == 0 {
		return nil, io.EOF
	}
	stream := c.Streams[0]
	c.Streams = c.Streams[1:]
	return stream, nil
}

// Redact removes credential-shaped diagnostic text. When the text has a
// secret-shaped form, it deliberately returns a generic message instead of
// risking preservation of an unrecognised secret suffix.
func Redact(value string) string {
	lower := normalizeSecretText(value)
	compact := strings.Join(strings.Fields(lower), "")
	spaced := strings.Join(strings.Fields(lower), " ")
	// Provider errors can include a full Authorization header (where Bearer and
	// its credential are separated by whitespace). Once a credential-shaped
	// field is present, returning a generic diagnostic is safer than attempting
	// to preserve a possibly secret suffix.
	for _, key := range secretMarkers {
		if strings.Contains(lower, key+"=") || strings.Contains(lower, key+":") ||
			strings.Contains(compact, key+"=") || strings.Contains(compact, key+":") {
			return "[REDACTED provider diagnostic]"
		}
	}
	if strings.Contains(spaced, "bearer ") || strings.Contains(spaced, "basic ") {
		return "[REDACTED provider diagnostic]"
	}
	if containsURIUserinfo(value) {
		return "[REDACTED provider diagnostic]"
	}
	return value
}

var secretMarkers = []string{
	"api_key", "apikey", "authorization", "cookie", "token", "secret",
	"password", "credential", "access_key", "private_key", "refresh",
}

var uriPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s'"<>]+`)

// normalizeSecretText normalizes separator variants used by HTTP headers,
// query parameters, and metadata keys before checking them against the same
// marker set used for structured payloads.
func normalizeSecretText(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "-", "_"))
}

// containsURIUserinfo detects embedded URI credentials in diagnostics and
// free-form tool arguments. Parsing each URI, rather than matching an '@'
// alone, avoids redacting ordinary email addresses in text.
func containsURIUserinfo(value string) bool {
	for _, candidate := range uriPattern.FindAllString(value, -1) {
		parsed, err := url.Parse(candidate)
		if err == nil && parsed.User != nil {
			return true
		}
	}
	return false
}

// RedactMetadata returns a copy suitable for crossing a renderer or durable
// storage boundary. Sensitive keys are redacted even when their values do not
// have a recognisable shape.
func RedactMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	redacted := make(map[string]string, len(metadata))
	for key, value := range metadata {
		if secretKey(key) {
			redacted[key] = "[REDACTED]"
			continue
		}
		redacted[key] = Redact(value)
	}
	return redacted
}

// RedactJSON structurally redacts arbitrary JSON before it crosses a durable
// or rendering boundary. Invalid JSON is replaced with a generic JSON string:
// callers must never persist an opaque payload they could not inspect.
func RedactJSON(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return json.RawMessage(`"[REDACTED invalid payload]"`)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return json.RawMessage(`"[REDACTED invalid payload]"`)
	}
	encoded, err := json.Marshal(redactValue(value))
	if err != nil {
		return json.RawMessage(`"[REDACTED payload]"`)
	}
	return encoded
}

// SanitizeEvent canonicalizes timestamps and redacts all event payloads.
// Error events get a deliberately narrow envelope so generic provider metadata
// cannot be accidentally rendered or persisted as diagnostics.
func SanitizeEvent(event Event) Event {
	if !event.At.IsZero() {
		event.At = event.At.UTC()
	}
	if event.Type != EventError {
		event.Data = RedactJSON(event.Data)
		return event
	}
	code := stableErrorCode(event.ErrorCode)
	message := "provider error"
	var payload map[string]json.RawMessage
	if json.Unmarshal(event.Data, &payload) == nil {
		if rawCode, ok := payload["code"]; ok && json.Unmarshal(rawCode, &code) == nil {
			code = stableErrorCode(code)
		}
		if rawMessage, ok := payload["message"]; ok {
			var supplied string
			if json.Unmarshal(rawMessage, &supplied) == nil && strings.TrimSpace(supplied) != "" {
				message = Redact(supplied)
			}
		}
	}
	event.ErrorCode = code
	event.Data, _ = json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
	return event
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if secretKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = redactValue(child)
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redactValue(child)
		}
		return result
	case string:
		return Redact(typed)
	default:
		return value
	}
}

func secretKey(key string) bool {
	lower := normalizeSecretText(key)
	for _, marker := range secretMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func stableErrorCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" || len(code) > 64 {
		return "provider_error"
	}
	for _, char := range code {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-') {
			return "provider_error"
		}
	}
	return code
}
