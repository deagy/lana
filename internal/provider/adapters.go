package provider

import (
	"context"
	"fmt"
)

// Backend is the only dependency an adapter needs. Concrete HTTP/SDK clients
// live outside the runtime kernel and can be injected without leaking their
// authentication configuration into logs or request events.
type Backend interface {
	Stream(context.Context, Request) (Stream, error)
}

type AdapterKind string

const (
	AdapterOpenAI           AdapterKind = "openai"
	AdapterAnthropic        AdapterKind = "anthropic"
	AdapterOpenAICompatible AdapterKind = "openai-compatible"
)

// Adapter marks protocol choice while preserving the same portable Client API.
// Endpoint and credentials intentionally do not appear in this type.
type Adapter struct {
	Kind    AdapterKind
	backend Backend
}

func NewOpenAIAdapter(backend Backend) *Adapter {
	return &Adapter{Kind: AdapterOpenAI, backend: backend}
}
func NewAnthropicAdapter(backend Backend) *Adapter {
	return &Adapter{Kind: AdapterAnthropic, backend: backend}
}
func NewOpenAICompatibleAdapter(backend Backend) *Adapter {
	return &Adapter{Kind: AdapterOpenAICompatible, backend: backend}
}

func (a *Adapter) Stream(ctx context.Context, request Request) (Stream, error) {
	if a == nil || a.backend == nil {
		return nil, fmt.Errorf("%s provider adapter is not configured", a.Kind)
	}
	stream, err := a.backend.Stream(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("%s provider stream: %s", a.Kind, Redact(err.Error()))
	}
	return &redactingStream{Stream: stream}, nil
}

type redactingStream struct{ Stream }

func (s *redactingStream) Recv(ctx context.Context) (Event, error) {
	event, err := s.Stream.Recv(ctx)
	if err != nil {
		return Event{}, fmt.Errorf("provider stream receive: %s", Redact(err.Error()))
	}
	return SanitizeEvent(event), nil
}
