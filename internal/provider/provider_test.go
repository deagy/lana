package provider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type errorBackend struct{}

func (errorBackend) Stream(context.Context, Request) (Stream, error) {
	return nil, errors.New("Authorization=Bearer highly-secret")
}

type credentialErrorBackend struct{ message string }

func (b credentialErrorBackend) Stream(context.Context, Request) (Stream, error) {
	return nil, errors.New(b.message)
}

type eventBackend struct{ stream Stream }

func (b eventBackend) Stream(context.Context, Request) (Stream, error) { return b.stream, nil }

func TestAdaptersRedactCredentialValues(t *testing.T) {
	adapter := NewOpenAIAdapter(errorBackend{})
	_, err := adapter.Stream(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" || contains(got, "highly-secret") {
		t.Fatalf("credential leaked: %q", got)
	}
}

func TestAdapterRedactsHeaderAndURIErrorContext(t *testing.T) {
	const secret = "do-not-leak"
	for _, message := range []string{
		"X-API-Key: " + secret,
		"Cookie: session=" + secret,
		"request to https://user:" + secret + "@api.example.test/?api-key=" + secret,
	} {
		adapter := NewOpenAIAdapter(credentialErrorBackend{message: message})
		_, err := adapter.Stream(context.Background(), Request{})
		if err == nil || contains(err.Error(), secret) {
			t.Fatalf("unsafe provider context %q: %v", message, err)
		}
	}
}

func TestSliceStreamHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewSliceStream().Recv(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestAdapterRedactsStructuredErrorEvent(t *testing.T) {
	at := time.Date(2026, 8, 10, 1, 2, 3, 0, time.FixedZone("local", -7*60*60))
	raw := Event{SchemaVersion: EventSchemaVersion, Type: EventError, At: at, Data: json.RawMessage(`{"code":"upstream_unavailable","message":"Authorization: Bearer top-secret","metadata":{"refresh_token":"also-secret"}}`)}
	adapter := NewOpenAIAdapter(eventBackend{stream: NewSliceStream(raw)})
	stream, err := adapter.Stream(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	event, err := stream.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if event.ErrorCode != "upstream_unavailable" || event.At.Location() != time.UTC || contains(string(event.Data), "top-secret") || contains(string(event.Data), "also-secret") {
		t.Fatalf("unsafe event: %#v", event)
	}
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNewEventAtAndInvalidPayloadAreDeterministic(t *testing.T) {
	at := time.Date(2026, 8, 10, 2, 3, 4, 0, time.UTC)
	event, err := NewEventAt(at, EventTextDelta, map[string]any{"metadata": map[string]string{"api_token": "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	if !event.At.Equal(at) || contains(string(event.Data), "secret") {
		t.Fatalf("event=%#v", event)
	}
	if got := string(RedactJSON(json.RawMessage(`{"token":"secret"} {}`))); got != `"[REDACTED invalid payload]"` {
		t.Fatalf("invalid payload = %s", got)
	}
}

func TestRedactionCoversHTTPHeadersAndURISecrets(t *testing.T) {
	const secret = "do-not-leak"
	for _, payload := range []string{
		"X-API-Key: " + secret,
		"X-API-Key \t: " + secret,
		"Cookie: session=" + secret,
		"https://user:" + secret + "@api.example.test/v1/messages",
		"https://api.example.test/v1/messages?api-key=" + secret,
		"https://api.example.test/v1/messages?refresh-token=" + secret,
	} {
		if got := Redact(payload); contains(got, secret) {
			t.Fatalf("credential leaked from %q: %q", payload, got)
		}
	}
}

func TestRedactJSONAndMetadataCoverHyphenatedHeaderKeys(t *testing.T) {
	const secret = "do-not-leak"
	payload := json.RawMessage(`{
		"command":"curl -H 'X-API-Key: do-not-leak' https://user:do-not-leak@example.test/?access-token=do-not-leak",
		"headers":{"X-API-Key":"do-not-leak","Authorization":"Bearer do-not-leak","Cookie":"session=do-not-leak"},
		"metadata":{"refresh-token":"do-not-leak"}
	}`)
	if got := string(RedactJSON(payload)); contains(got, secret) {
		t.Fatalf("credential leaked from JSON: %s", got)
	}
	metadata := RedactMetadata(map[string]string{
		"x-api-key":     secret,
		"refresh-token": secret,
		"request-url":   "https://user:" + secret + "@example.test/?api-key=" + secret,
	})
	for key, value := range metadata {
		if contains(value, secret) {
			t.Fatalf("credential leaked from metadata %q: %q", key, value)
		}
	}
}

func TestSanitizeEventRedactsCommandHeaderAndMetadataPayloads(t *testing.T) {
	const secret = "do-not-leak"
	event := SanitizeEvent(Event{
		SchemaVersion: EventSchemaVersion,
		Type:          EventToolCall,
		At:            time.Now(),
		Data: json.RawMessage(`{
			"command":"curl -H 'Cookie: session=do-not-leak' https://api.example.test/?api-key=do-not-leak",
			"headers":{"X-API-Key":"do-not-leak","Authorization":"Bearer do-not-leak"},
			"metadata":{"access-token":"do-not-leak"}
		}`),
	})
	if got := string(event.Data); contains(got, secret) {
		t.Fatalf("credential leaked from event: %s", got)
	}
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
