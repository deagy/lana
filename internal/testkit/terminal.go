// Package testkit contains deterministic terminal-contract fixtures shared by
// CLI and TUI tests. It intentionally has no provider, credential, process,
// or terminal side effects.
package testkit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/deagy/lana/internal/agent"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/tools"
)

// Script records requests and replays events in their declared order. Tests
// adapt Emit to the presentation package's EventSink, keeping this package
// independent of the CLI implementation it helps test.
type Script struct {
	Events []provider.Event
	Result agent.TurnResult
	Err    error

	mu       sync.Mutex
	requests []provider.Request
}

// Run records request, emits the scripted events in order, and honours
// cancellation before each event.
func (s *Script) Run(ctx context.Context, request provider.Request, emit func(context.Context, provider.Event) error) (agent.TurnResult, error) {
	s.mu.Lock()
	s.requests = append(s.requests, cloneRequest(request))
	events := append([]provider.Event(nil), s.Events...)
	result, runErr := s.Result, s.Err
	s.mu.Unlock()
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return agent.TurnResult{}, err
		}
		if emit != nil {
			if err := emit(ctx, event); err != nil {
				return agent.TurnResult{}, err
			}
		}
	}
	return result, runErr
}

// Requests returns a snapshot, so tests can make assertions without racing a
// running terminal loop.
func (s *Script) Requests() []provider.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	requests := make([]provider.Request, len(s.requests))
	for i, request := range s.requests {
		requests[i] = cloneRequest(request)
	}
	return requests
}

func cloneRequest(request provider.Request) provider.Request {
	request.Messages = append([]provider.Message(nil), request.Messages...)
	request.Tools = append([]provider.ToolDefinition(nil), request.Tools...)
	if request.Metadata != nil {
		request.Metadata = make(map[string]string, len(request.Metadata))
		for key, value := range request.Metadata {
			request.Metadata[key] = value
		}
	}
	return request
}

// FixturePath finds repository testdata relative to this source file instead
// of relying on the current package's go test working directory.
func FixturePath(parts ...string) string {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		panic("locate testkit source")
	}
	path := []string{filepath.Dir(source), "..", "..", "testdata"}
	return filepath.Join(append(path, parts...)...)
}

// LoadEvents decodes a checked-in, provider-neutral event fixture.
func LoadEvents(parts ...string) ([]provider.Event, error) {
	path := FixturePath(parts...)
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read terminal fixture %q: %w", path, err)
	}
	var events []provider.Event
	if err := json.Unmarshal(contents, &events); err != nil {
		return nil, fmt.Errorf("decode terminal fixture %q: %w", path, err)
	}
	return events, nil
}

// ToolScript is a deterministic authorizer/executor pair for turn tests. A
// configured authorization error prevents Execute from being called, matching
// the production authorization boundary.
type ToolScript struct {
	AuthorizeErr error
	ExecuteErr   error
	Result       tools.Result

	mu    sync.Mutex
	Calls []tools.Call
}

func (s *ToolScript) Authorize(_ context.Context, call tools.Call) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls = append(s.Calls, call)
	return s.AuthorizeErr
}

func (s *ToolScript) Execute(_ context.Context, call tools.Call) (tools.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls = append(s.Calls, call)
	return s.Result, s.ExecuteErr
}

func (s *ToolScript) RecordedCalls() []tools.Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]tools.Call(nil), s.Calls...)
}
