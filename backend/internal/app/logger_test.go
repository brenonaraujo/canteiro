package app

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"nope", slog.LevelInfo},
		{"", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, parseLevel(tt.in))
		})
	}
}

func TestRenameTime(t *testing.T) {
	t.Parallel()
	got := renameTime(nil, slog.Attr{Key: slog.TimeKey, Value: slog.StringValue("x")})
	assert.Equal(t, "ts", got.Key)
	kept := renameTime(nil, slog.Attr{Key: "msg", Value: slog.StringValue("ok")})
	assert.Equal(t, "msg", kept.Key)
}

func TestNewLogger_DoesNotPanic(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewLogger("info", "canteiro", "dev"))
}
