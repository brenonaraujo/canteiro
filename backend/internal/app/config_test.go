package app

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestLoadConfig_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_ = os.Unsetenv("DATABASE_URL")
	_, err := LoadConfig()
	require.Error(t, err)
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:app@postgres:5432/app?sslmode=disable")
	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "8080", cfg.Port)
	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, "release", cfg.GinMode)
	require.Equal(t, "/metrics", cfg.MetricsPath)
	require.Equal(t, "canteiro", cfg.ServiceName)
	require.Equal(t, "pt-BR", cfg.DefaultLocale)
	require.Equal(t, 30*time.Second, cfg.ShutdownTimeout)
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel
func TestLoadConfig_Overrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@db/app?sslmode=disable")
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("GIN_MODE", "debug")
	t.Setenv("METRICS_PATH", "/internal/metrics")
	t.Setenv("SERVICE_NAME", "canteiro-api")
	t.Setenv("DEFAULT_LOCALE", "en")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "9090", cfg.Port)
	require.Equal(t, "debug", cfg.LogLevel)
	require.Equal(t, "debug", cfg.GinMode)
	require.Equal(t, "/internal/metrics", cfg.MetricsPath)
	require.Equal(t, "canteiro-api", cfg.ServiceName)
	require.Equal(t, "en", cfg.DefaultLocale)
	require.Equal(t, 5*time.Second, cfg.ShutdownTimeout)
}
