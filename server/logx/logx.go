// Package logx centralizes structured logging setup. We use slog so every log line
// is machine-parseable and carries the same keys (invocation_id, tool, etc.). Level
// comes from LOG_LEVEL (debug|info|warn|error); default info.
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a logger writing to stderr at the level named by LOG_LEVEL.
func New() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: levelFromEnv()}))
}

func levelFromEnv() slog.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Discard returns a logger that drops everything — convenient default for tests that
// don't assert on logs.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, nil))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
