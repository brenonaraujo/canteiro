package app

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config is loaded exclusively from the process environment.
type Config struct { //nolint:govet // fieldalignment vs env-grouped fields
	Port                string        `envconfig:"PORT" default:"8080"`
	LogLevel            string        `envconfig:"LOG_LEVEL" default:"info"`
	GinMode             string        `envconfig:"GIN_MODE" default:"release"`
	MetricsPath         string        `envconfig:"METRICS_PATH" default:"/metrics"`
	ServiceName         string        `envconfig:"SERVICE_NAME" default:"canteiro"`
	DatabaseURL         string        `envconfig:"DATABASE_URL" required:"true"`
	DefaultLocale       string        `envconfig:"DEFAULT_LOCALE" default:"pt-BR"`
	OTelEndpoint        string        `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	GoogleClientID      string        `envconfig:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret  string        `envconfig:"GOOGLE_CLIENT_SECRET"`
	GoogleRedirectURL   string        `envconfig:"GOOGLE_REDIRECT_URL"`
	SessionSecret       string        `envconfig:"SESSION_SECRET"`
	WebAppURL           string        `envconfig:"WEB_APP_URL" default:"http://localhost:3000"`
	SessionCookieName   string        `envconfig:"SESSION_COOKIE_NAME" default:"canteiro_session"`
	ShutdownTimeout     time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"30s"`
	SessionTTL          time.Duration `envconfig:"SESSION_TTL" default:"168h"`
	SessionCookieSecure bool          `envconfig:"SESSION_COOKIE_SECURE" default:"false"`
}

// LoadConfig fails when required env vars are missing.
func LoadConfig() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &c, nil
}
