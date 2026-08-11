// Package app assembles process-scoped dependencies for Lana commands.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deagy/lana/pkg/config"
	"github.com/deagy/lana/pkg/logger"
)

// Application contains the immutable dependencies shared by commands in one
// CLI invocation. New commands should obtain it from their command context
// instead of independently reloading configuration.
type Application struct {
	config *config.AppConfig
	logger *slog.Logger
}

// Options controls construction of an Application.
type Options struct {
	Config config.ResolveOptions
}

// New resolves configuration once and initializes the process logger from the
// resolved logging settings. It deliberately does not log the resolved config:
// logging configuration must cross the redaction boundary in config first.
func New(opts Options) (*Application, error) {
	resolved, err := config.Resolve(opts.Config)
	if err != nil {
		return nil, fmt.Errorf("resolve application configuration: %w", err)
	}
	logging := resolved.Logging()
	return &Application{
		config: resolved,
		logger: logger.Init(logger.Options{
			Level:    logging.Level,
			Format:   logging.Format,
			ToFile:   logging.ToFile,
			FilePath: logging.FilePath,
		}),
	}, nil
}

// Config returns the immutable resolved configuration.
func (a *Application) Config() *config.AppConfig { return a.config }

// Logger returns the invocation's configured logger.
func (a *Application) Logger() *slog.Logger { return a.logger }

type contextKey struct{}

// WithContext attaches app to a command context for dependency lookup.
func WithContext(ctx context.Context, app *Application) context.Context {
	return context.WithValue(ctx, contextKey{}, app)
}

// FromContext retrieves the invocation dependencies, if command wiring has
// initialized them. Commands may use this to preserve compatibility when they
// are directly unit-tested without the root command.
func FromContext(ctx context.Context) (*Application, bool) {
	app, ok := ctx.Value(contextKey{}).(*Application)
	return app, ok
}
