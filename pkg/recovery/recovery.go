// Package recovery provides error recovery and graceful shutdown capabilities.
package recovery

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sync"
)

// Handler manages error recovery and graceful shutdown.
type Handler struct {
	mu          sync.Mutex
	signals     chan os.Signal
	onInterrupt func(ctx context.Context) error
	onShutdown  func(ctx context.Context) error
	onError     func(err error) error
	interrupted bool
	shutdown    bool
}

// NewHandler creates a new recovery handler.
func NewHandler() *Handler {
	return &Handler{
		signals: make(chan os.Signal, 1),
	}
}

// SetInterruptHandler sets the handler for interrupt signals (Ctrl+C).
func (h *Handler) SetInterruptHandler(fn func(ctx context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onInterrupt = fn
}

// SetShutdownHandler sets the handler for shutdown signals (SIGTERM).
func (h *Handler) SetShutdownHandler(fn func(ctx context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onShutdown = fn
}

// SetErrorHandler sets the handler for errors.
func (h *Handler) SetErrorHandler(fn func(err error) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onError = fn
}

// Start begins listening for signals.
func (h *Handler) Start() {
	signal.Notify(h.signals, syscall.SIGINT, syscall.SIGTERM)
}

// Stop stops listening for signals.
func (h *Handler) Stop() {
	signal.Stop(h.signals)
	close(h.signals)
}

// HandleSignals processes signals until the context is done.
func (h *Handler) HandleSignals(ctx context.Context) error {
	h.Start()
	defer h.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-h.signals:
			switch sig {
			case syscall.SIGINT:
				h.mu.Lock()
				if h.interrupted {
					h.mu.Unlock()
					// Second interrupt - force exit
					os.Exit(130)
				}
				h.interrupted = true
				onInterrupt := h.onInterrupt
				h.mu.Unlock()

				if onInterrupt != nil {
					if err := onInterrupt(ctx); err != nil {
						return fmt.Errorf("interrupt handler failed: %w", err)
					}
				}
			case syscall.SIGTERM:
				h.mu.Lock()
				if h.shutdown {
					h.mu.Unlock()
					os.Exit(143)
				}
				h.shutdown = true
				onShutdown := h.onShutdown
				h.mu.Unlock()

				if onShutdown != nil {
					if err := onShutdown(ctx); err != nil {
						return fmt.Errorf("shutdown handler failed: %w", err)
					}
				}
			}
		}
	}
}

// Recover handles a panic and prints to the writer.
func Recover(w io.Writer) {
	if r := recover(); r != nil {
		fmt.Fprintf(w, "panic: %v\n", r)
	}
}

// WithRecover wraps a function to recover from panics.
func WithRecover(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

// RetryWithBackoff retries a function with exponential backoff.
func RetryWithBackoff(ctx context.Context, maxRetries int, baseDelay, maxDelay int, fn func() error) error {
	var err error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			delay := baseDelay * (1 << uint(i-1))
			if delay > maxDelay {
				delay = maxDelay
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(delay) * time.Second):
			}
		}
		err = fn()
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, err)
}

// WithTimeout wraps a function with a timeout.
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(ctx)
}
