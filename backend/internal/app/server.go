package app

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/auth"
	"github.com/brenonaraujo/canteiro/backend/internal/handler"
	"github.com/brenonaraujo/canteiro/backend/internal/repository"
)

// ServerOpts is the HTTP process wiring.
type ServerOpts struct { //nolint:govet // fieldalignment vs readable wiring
	Logger      *slog.Logger
	Checkers    []repository.Checker
	Auth        *auth.API
	Listing     *handler.ListingAPI
	ServiceName string
	MetricsPath string
	GinMode     string
	CORSOrigin  string
}

type apiMux struct {
	*handler.Server
	*auth.API
	*handler.ListingAPI
}

// NewRouter builds recovery, CORS, metrics, locale, health, auth and metrics scrape.
func NewRouter(opts ServerOpts) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), corsMiddleware(opts.CORSOrigin), metricsMiddleware(), localeMiddleware(), requestLogMiddleware(opts.Logger))
	acc := opts.Auth
	if acc == nil {
		acc = auth.NewAPI(auth.Deps{})
	}
	api.RegisterHandlers(r, apiMux{
		Server:     handler.NewServer(serviceName(opts.ServiceName), opts.Checkers),
		API:        acc,
		ListingAPI: opts.Listing,
	})
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
