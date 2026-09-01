package app

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger writes JSON to stdout (12-factor XI). No PII fields are added here.
func NewLogger(level, service, version string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       parseLevel(level),
		ReplaceAttr: renameTime,
	})
	return slog.New(h).With("service", service, "version", version)
}

func parseLevel(level string) slog.Level {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToLower(level))); err != nil {
		return slog.LevelInfo
	}
	return lvl
}

func renameTime(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		a.Key = "ts"
	}
	return a
}
