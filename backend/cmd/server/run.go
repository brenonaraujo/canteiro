package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/app"
	"github.com/brenonaraujo/canteiro/backend/internal/platform/postgres"
	"github.com/brenonaraujo/canteiro/backend/internal/repository"
)

func run(ctx context.Context, cfg *app.Config, logger *slog.Logger) {
	db, err := app.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("db", "error", err.Error())
		os.Exit(1)
	}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router(cfg, logger, []repository.Checker{postgres.DBChecker{DB: db}}, db),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go serve(srv, logger)
	<-ctx.Done()
	logger.Info("shutting down")
	shutdown(srv, cfg.ShutdownTimeout, logger)
}

func router(cfg *app.Config, logger *slog.Logger, checkers []repository.Checker, db *gorm.DB) http.Handler {
	if cfg.GinMode != "" {
		gin.SetMode(cfg.GinMode)
	}
	return app.NewRouter(app.ServerOpts{
		ServiceName: cfg.ServiceName,
		MetricsPath: cfg.MetricsPath,
		GinMode:     cfg.GinMode,
		Logger:      logger,
		Checkers:    checkers,
		Auth:        app.NewAuthAPI(cfg, db),
		CORSOrigin:  cfg.WebAppURL,
	})
}

func serve(srv *http.Server, logger *slog.Logger) {
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err.Error())
		os.Exit(1)
	}
}

func shutdown(srv *http.Server, timeout time.Duration, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "error", err.Error())
	}
}
