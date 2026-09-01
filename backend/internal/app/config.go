package app

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config is loaded exclusively from the process environment.
type Config struct {
	Port            string        `envconfig:"PORT" default:"8080"`
	LogLevel        string        `envconfig:"LOG_LEVEL" default:"info"`
	GinMode         string        `envconfig:"GIN_MODE" default:"release"`
	MetricsPath     string        `envconfig:"METRICS_PATH" default:"/metrics"`
	ServiceName     string        `envconfig:"SERVICE_NAME" default:"canteiro"`
	DatabaseURL     string        `envconfig:"DATABASE_URL" required:"true"`
	DefaultLocale   string        `envconfig:"DEFAULT_LOCALE" default:"pt-BR"`
	OTelEndpoint    string        `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"30s"`
}

// LoadConfig fails when required env vars are missing.
func LoadConfig() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &c, nil
}
