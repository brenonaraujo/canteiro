package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/brenonaraujo/canteiro/backend/internal/app"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if hasHealthcheck(os.Args[1:]) {
		runHealthcheck(os.Getenv("PORT"))
		return
	}
	cfg, err := app.LoadConfig()
	if err != nil {
		slog.Error("config", "error", err.Error())
		os.Exit(1)
	}
	logger := app.NewLogger(cfg.LogLevel, cfg.ServiceName, version)
	slog.SetDefault(logger)
	app.RegisterAppInfo(version, commit)
	if _, err := i18n.Load(); err != nil {
		logger.Error("i18n", "error", err.Error())
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	run(ctx, cfg, logger)
}
