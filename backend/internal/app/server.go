package app

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/handler"
	"github.com/brenonaraujo/canteiro/backend/internal/repository"
)

// ServerOpts is the HTTP process wiring. Keep product features out of F0.
type ServerOpts struct {
	ServiceName string
	MetricsPath string
	GinMode     string
	Logger      *slog.Logger
	Checkers    []repository.Checker
}

// NewRouter builds recovery, metrics, locale, health, ready and metrics scrape.
func NewRouter(opts ServerOpts) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), metricsMiddleware(), localeMiddleware(), requestLogMiddleware(opts.Logger))
	api.RegisterHandlers(r, handler.NewServer(serviceName(opts.ServiceName), opts.Checkers))
	r.GET(metricsPath(opts.MetricsPath), gin.WrapH(promhttp.Handler()))
	return r
}

func ginMode(mode string) string {
	if mode == "" {
		return gin.ReleaseMode
	}
	return mode
}

func serviceName(name string) string {
	if name == "" {
		return "canteiro"
	}
	return name
}

func metricsPath(path string) string {
	if path == "" {
		return "/metrics"
	}
	return path
}
