package nilslog

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"golang.org/x/exp/slog"
)

func TestNil(t *testing.T) {
	logger := New()

	// ensure that logging all levels "works" but does nothing
	levels := []slog.Level{
		slog.LevelDebug,
		slog.LevelInfo,
		slog.LevelWarn,
		slog.LevelError,
	}
	for _, level := range levels {
		logger.Log(context.Background(), level, "should not be logged")
	}

	// calls Handler.WithAttr and Handler.WithGroup
	logger.WithGroup("group").With("key", "value").Error("should not be logged")
}

func TestNewIfNil(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf))

	NewIfNil(logger).Error("should log")
	NewIfNil(nil).Error("should not log")

	if !strings.HasSuffix(buf.String(), `msg="should log"`+"\n") {
		t.Error(buf.String())
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger := New()
		if logger == nil {
			b.Fatalf("logger must not be nil: %p", logger)
		}
	}
}

func BenchmarkWith(b *testing.B) {
	logger := New()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger2 := logger.With("key", "value")
		if logger2 == nil {
			b.Fatalf("logger2 must not be nil: %p", logger2)
		}
	}
}

func BenchmarkGroup(b *testing.B) {
	logger := New()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger2 := logger.WithGroup("group")
		if logger2 == nil {
			b.Fatalf("logger2 must not be nil: %p", logger2)
		}
	}
}
