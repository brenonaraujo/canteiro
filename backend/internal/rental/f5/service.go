// Package f5 owns the F5 devolução + avaria + dívida service. It is
// deliberately separate from package rental (which owns the F3 reservation
// lifecycle) and from the rental.Service (which F3 wires to the HTTP
// adapter). The F5 service is consumed by the F5 HTTP adapter in
// internal/handler/f5 and by the F3 rental.Service (Pilar 5: the "renter
// has open debt" gate).
//
// This file declares the F5 service skeleton: interfaces, shared config and
// the constructor. The actual use cases (return, damage, debt) live in the
// sibling files in this package.
package f5

import (
	"context"
	"time"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// IDGenerator produces UUIDs for the F5 entities. The fakes use a counter;
// production wires defaultIDGen from package rental.
type IDGenerator interface {
	String() string
}

// Clock returns the current UTC time. Injected for deterministic window
// tests (Pilar 1: 48h owner claim, 48h renter defense; Pilar 3: 5d
// settlement).
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// RentalLookup is the slice of rental.Service the F5 service needs to
// read the rental. Implemented by rental.Service in production; fakes in
// tests.
type RentalLookup interface {
	Get(ctx context.Context, id string) (rental.Rental, error)
}

// ReturnRepository persists Return rows. Implemented by the rentalpg
// subpackage in production; fakes in tests.
type ReturnRepository interface {
	Create(ctx context.Context, ret rental.Return) (rental.Return, error)
	GetByRental(ctx context.Context, rentalID string) (rental.Return, bool, error)
	UpdateState(ctx context.Context, id string, from, to rental.ReturnState, mutate func(ret *rental.Return)) (rental.Return, error)
}

// DamageRepository persists DamageClaim rows.
type DamageRepository interface {
	Create(ctx context.Context, claim rental.DamageClaim) (rental.DamageClaim, error)
	GetByID(ctx context.Context, id string) (rental.DamageClaim, error)
	GetByRental(ctx context.Context, rentalID string) (rental.DamageClaim, bool, error)
	UpdateState(ctx context.Context, id string, from, to rental.DamageState, mutate func(claim *rental.DamageClaim)) (rental.DamageClaim, error)
	ListExpiring(ctx context.Context, before time.Time) ([]rental.DamageClaim, error)
}

// DebtRepository persists Debt rows.
type DebtRepository interface {
	Create(ctx context.Context, d rental.Debt) (rental.Debt, error)
	GetByID(ctx context.Context, id string) (rental.Debt, error)
	GetByDamage(ctx context.Context, damageID string) (rental.Debt, bool, error)
	UpdateState(ctx context.Context, id string, from, to rental.DebtState, mutate func(d *rental.Debt)) (rental.Debt, error)
	ListOpenForRenter(ctx context.Context, renterID string) ([]rental.Debt, error)
	ListDueBy(ctx context.Context, before time.Time) ([]rental.Debt, error)
}

// Config groups the F5 time-window knobs. Per Decisão 3 these are config,
// not constants: defaults below are the SPEC values; callers can override
// in tests (and a future F12 admin tool would override in production).
type Config struct {
	Now                      Clock
	IDGen                    IDGenerator
	OwnerClaimWindow         time.Duration
	RenterDefenseWindow      time.Duration
	ReturnConfirmationWindow time.Duration
	ReturnGraceAfterEnd      time.Duration
	DebtSettlementWindow     time.Duration
}

// Defaults fills the zero-valued Config fields with the SPEC defaults.
// The numbers below are the only place in F5 that hard-codes a window; if
// the platform ever needs a different value, change here and bump
// defaults (Pilar 1, Decisão 3).
func (c *Config) Defaults() {
	if c.OwnerClaimWindow == 0 {
		c.OwnerClaimWindow = 48 * time.Hour
	}
	if c.RenterDefenseWindow == 0 {
		c.RenterDefenseWindow = 48 * time.Hour
	}
	if c.ReturnConfirmationWindow == 0 {
		c.ReturnConfirmationWindow = 48 * time.Hour
	}
	if c.DebtSettlementWindow == 0 {
		c.DebtSettlementWindow = 5 * 24 * time.Hour
	}
	if c.IDGen == nil {
		c.IDGen = defaultIDGen{}
	}
	if c.Now == nil {
		c.Now = realClock{}
	}
}

// Service orchestrates the F5 lifecycle.
type Service struct {
	rentals RentalLookup
	returns ReturnRepository
	damage  DamageRepository
	debts   DebtRepository
	cfg     Config
}

// NewService wires the F5 service.
func NewService(cfg Config, rentals RentalLookup, returns ReturnRepository, damage DamageRepository, debts DebtRepository) *Service {
	cfg.Defaults()
	return &Service{cfg: cfg, rentals: rentals, returns: returns, damage: damage, debts: debts}
}
