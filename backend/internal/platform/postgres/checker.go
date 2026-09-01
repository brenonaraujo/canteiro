package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/app"
)

// DBChecker pings Postgres for /readyz. It never migrates.
type DBChecker struct {
	DB *gorm.DB
}

// Name implements repository.Checker.
func (c DBChecker) Name() string { return "db" }

// Check pings the pool and records db_* metrics.
func (c DBChecker) Check(ctx context.Context) error {
	if c.DB == nil {
		app.DBQueriesTotal.WithLabelValues("ping", "postgres", "error").Inc()
		return fmt.Errorf("db not configured")
	}
	sqlDB, err := c.DB.DB()
	if err != nil {
		app.DBQueriesTotal.WithLabelValues("ping", "postgres", "error").Inc()
		return fmt.Errorf("sql db: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		app.DBQueriesTotal.WithLabelValues("ping", "postgres", "error").Inc()
		return fmt.Errorf("ping: %w", err)
	}
	app.DBQueriesTotal.WithLabelValues("ping", "postgres", "ok").Inc()
	return nil
}
