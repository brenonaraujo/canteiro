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
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
	"github.com/brenonaraujo/canteiro/backend/internal/handler"
	"github.com/brenonaraujo/canteiro/backend/internal/payment"
	"github.com/brenonaraujo/canteiro/backend/internal/platform/postgres"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
	f5svc "github.com/brenonaraujo/canteiro/backend/internal/rental/f5"
	"github.com/brenonaraujo/canteiro/backend/internal/repository"
	f5pg "github.com/brenonaraujo/canteiro/backend/internal/repository/f5"
	listingpg "github.com/brenonaraujo/canteiro/backend/internal/repository/listing"
	rentalpg "github.com/brenonaraujo/canteiro/backend/internal/repository/rental"
	reviewpg "github.com/brenonaraujo/canteiro/backend/internal/repository/review"
	lookuppgrental "github.com/brenonaraujo/canteiro/backend/internal/repository/review/lookuppgrental"
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
	authAPI := app.NewAuthAPI(cfg, db)
	listingSvc := listing.NewService(listingpg.New(db), authAPI.Accounts(), time.Now().UTC())
	listingAPI := handler.NewListingAPI(listingSvc, authAPI.CurrentAccountID)
	rentalRepo := rentalpg.New(db)
	rentalProvider := payment.NewNoop()
	rentalListingLookup := &listingLookupAdapter{svc: listingSvc}
	rentalSvc := rentsvc.NewService(rentalRepo, rentalListingLookup, authAPI.Accounts(), rentalProvider, rentsvc.Config{})
	rentalAPI := handler.NewRentalAPI(rentalSvc, authAPI.CurrentAccountID)
	paymentAPI := handler.NewPaymentAPI(rentalSvc, rentalProvider)
	// F5: wire the devolution + damage + debt service. The repos are
	// Postgres-backed; the rental lookup is the F3 rental.Service (the F5
	// service only needs Get(id)).
	f5ReturnRepo := f5pg.NewReturnRepo(db)
	f5DamageRepo := f5pg.NewDamageRepo(db)
	f5DebtRepo := f5pg.NewDebtRepo(db)
	rentalLookup := &rentalLookupAdapter{svc: rentalSvc}
	f5Svc := f5svc.NewService(f5svc.Config{}, rentalLookup, f5ReturnRepo, f5DamageRepo, f5DebtRepo)
	f5API := handler.NewF5API(f5Svc, f5Svc, f5Svc, authAPI.CurrentAccountID)
	// F6: review domain. The repository is Postgres-backed; the
	// rental lookup is a thin adapter over rental.Service.Get + a
	// direct SQL probe for the F5-terminal state (devolução closed
	// OR avaria resolved).
	reviewRepo := reviewpg.New(db)
	reviewRentalLookup := lookuppgrental.New(db, &reviewRentalLookupAdapter{svc: rentalSvc})
	reviewSvc := review.NewService(review.Config{}, reviewRentalLookup, reviewRepo)
	reviewAPI := handler.NewReviewAPI(reviewSvc, authAPI.CurrentAccountID)
	return app.NewRouter(app.ServerOpts{
		ServiceName: cfg.ServiceName,
		MetricsPath: cfg.MetricsPath,
		GinMode:     cfg.GinMode,
		Logger:      logger,
		Checkers:    checkers,
		Auth:        authAPI,
		Listing:     listingAPI,
		Rental:      rentalAPI,
		Payment:     paymentAPI,
		F5:          f5API,
		Review:      reviewAPI,
		CORSOrigin:  cfg.WebAppURL,
	})
}

// rentalLookupAdapter bridges rental.Service.Get to the
// f5.RentalLookup surface (which only needs ctx+id).
type rentalLookupAdapter struct {
	svc *rentsvc.Service
}

func (a *rentalLookupAdapter) Get(ctx context.Context, id string) (rental.Rental, error) {
	return a.svc.Get(ctx, id)
}

// listingLookupAdapter bridges listing.Service.GetPublic to the
// rental.ListingLookup surface (which only needs ctx+id). Public listings
// are the only kind the rental flow needs to read for snapshotting.
type listingLookupAdapter struct {
	svc *listing.Service
}

func (a *listingLookupAdapter) GetByID(ctx context.Context, id string) (listing.Listing, error) {
	return a.svc.GetPublic(ctx, id)
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
