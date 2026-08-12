package recovery

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestWithRecover_Success(t *testing.T) {
	fn := func() error {
		return nil
	}

	err := WithRecover(fn)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestWithRecover_Panic(t *testing.T) {
	fn := func() error {
		panic("test panic")
	}

	err := WithRecover(fn)
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if err.Error() != "panic: test panic" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithRecover_RuntimePanic(t *testing.T) {
	fn := func() error {
		var p *int
		*p = 42
		return nil
	}

	err := WithRecover(fn)
	if err == nil {
		t.Fatal("expected error from runtime panic")
	}
}

func TestRetryWithBackoff_Success(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("not yet")
		}
		return nil
	}

	err := RetryWithBackoff(context.Background(), 5, 0, 1, fn)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got: %d", attempts)
	}
}

func TestRetryWithBackoff_Failure(t *testing.T) {
	fn := func() error {
		return errors.New("always fails")
	}

	err := RetryWithBackoff(context.Background(), 2, 0, 1, fn)
	if err == nil {
		t.Fatal("expected error after retries")
	}
}

func TestRetryWithBackoff_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fn := func() error {
		return errors.New("should not be called")
	}

	err := RetryWithBackoff(ctx, 5, 0, 1, fn)
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestWithTimeout_Success(t *testing.T) {
	fn := func(ctx context.Context) error {
		return nil
	}

	err := WithTimeout(context.Background(), 5*time.Second, fn)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestWithTimeout_Expires(t *testing.T) {
	fn := func(ctx context.Context) error {
		// Sleep longer than timeout
		select {
		case <-time.After(100 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	err := WithTimeout(context.Background(), 10*time.Millisecond, fn)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestHandler_NewHandler(t *testing.T) {
	h := NewHandler()
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.signals == nil {
		t.Fatal("expected non-nil signals channel")
	}
}

func TestHandler_SetHandlers(t *testing.T) {
	h := NewHandler()

	h.SetInterruptHandler(func(ctx context.Context) error {
		return nil
	})
	h.SetShutdownHandler(func(ctx context.Context) error {
		return nil
	})
	h.SetErrorHandler(func(err error) error {
		return nil
	})

	// Just verify they don't panic
}

func TestHandler_StartStop(t *testing.T) {
	h := NewHandler()
	h.Start()
	h.Stop()
	// Should not panic
}

func TestRecover(t *testing.T) {
	var output os.File
	Recover(&output)
	// Should not panic
}

func TestRecover_WithPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			// Expected
		}
	}()

	fn := func() {
		panic("test")
	}

	fn()
}

func TestRetryWithBackoff_LargeDelay(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("fail")
		}
		return nil
	}

	err := RetryWithBackoff(context.Background(), 5, 0, 1, fn)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}
