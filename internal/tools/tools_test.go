package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRegistryValidatesAndDispatches(t *testing.T) {
	called := false
	registry := NewRegistry(map[string]Executor{"echo": ExecutorFunc(func(_ context.Context, call Call) (Result, error) {
		called = true
		return JSONResult(call, map[string]string{"ok": "yes"})
	})})
	result, err := registry.Execute(context.Background(), Call{ID: "1", Name: "echo", Arguments: json.RawMessage(`{}`)})
	if err != nil || !called || result.CallID != "1" {
		t.Fatalf("result=%#v err=%v called=%v", result, err, called)
	}
}

func TestRegistryRejectsResultWithoutExplicitTimestamp(t *testing.T) {
	registry := NewRegistry(map[string]Executor{"echo": ExecutorFunc(func(context.Context, Call) (Result, error) {
		return Result{Content: json.RawMessage(`{"token":"secret"}`)}, nil
	})})
	_, err := registry.Execute(context.Background(), Call{ID: "1", Name: "echo", Arguments: json.RawMessage(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("result without timestamp must fail closed, err=%v", err)
	}
}

func TestResultExplicitTimestampAndErrorSemantics(t *testing.T) {
	at := time.Date(2026, 8, 10, 2, 3, 4, 0, time.UTC)
	result, err := JSONResultAt(at, Call{ID: "call", Name: "echo"}, map[string]string{"api_token": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.At.Equal(at) || string(result.Content) == `{"api_token":"secret"}` || result.Validate() != nil {
		t.Fatalf("result=%#v", result)
	}
	result.ErrorCode = "unexpected"
	if err := result.Validate(); err == nil {
		t.Fatal("successful result with an error code must be rejected")
	}
	failure := ErrorResultAt(at, Call{ID: "call", Name: "echo"}, "bad code!", errString("Bearer super-secret"))
	if failure.ErrorCode != "tool_failed" || failure.Validate() != nil || string(failure.Content) == `"Bearer super-secret"` {
		t.Fatalf("failure=%#v", failure)
	}
	if ErrorResultAt(time.Time{}, Call{ID: "call", Name: "echo"}, "failed", nil).Validate() == nil {
		t.Fatal("zero timestamp must not acquire an implicit timestamp")
	}
}

func TestErrorResultRedacts(t *testing.T) {
	result := ErrorResult(Call{ID: "1", Name: "echo"}, "failed", errString("api_key=secret"))
	if string(result.Content) == `"api_key=secret"` {
		t.Fatalf("secret leaked: %s", result.Content)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
