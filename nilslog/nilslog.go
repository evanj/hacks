// Package nilslog creates a disabled slog.Logger that never logs anything.
package nilslog

import (
	"context"

	"golang.org/x/exp/slog"
)

type nilSlogHandler struct{}

// Enabled implements slog.Handler by always returning false.
func (n *nilSlogHandler) Enabled(context.Context, slog.Level) bool {
	return false
}

// Handle should never be called: Enabled() always returns false.
func (n *nilSlogHandler) Handle(context.Context, slog.Record) error {
	panic("BUG: must not be called: Enabled() is false")
}

// WithAttrs implements slog.Handler by returning the same handler that does nothing.
func (n *nilSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return n
}

// WithGroup implements slog.Handler by returning the same handler that does nothing.
func (n *nilSlogHandler) WithGroup(name string) slog.Handler {
	return n
}

var nilLogger *slog.Logger = slog.New(&nilSlogHandler{})

// New returns a new *slog.Logger that never logs.
func New() *slog.Logger {
	return nilLogger
}

// NewIfNil returns logger if it is not nil, or calls New() if it is nil.
func NewIfNil(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return New()
}
