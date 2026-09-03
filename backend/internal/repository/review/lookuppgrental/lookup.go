// Package lookuppgrental bridges rental.Service.Get onto the
// review.RentalLookup surface. The review domain stays free of
// cross-package coupling on the F5 rental state machine — this
// adapter is the only place that knows about GORM and the F5 tables.
//
// The GetParticipant half reads the rental via rental.Service.Get
// and resolves the participants (including the operator=owner
// collapse per EC-5). The IsTerminal half runs a single SELECT
// against devolucoes + avaria_pedidos so the review service stays
// free of cross-package coupling on the F5 state machine.
package lookuppgrental

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
)

// RentalService is the slice of rental.Service the adapter needs.
// Defined as a structural interface to avoid importing the rental
// package from this thin adapter.
type RentalService interface {
	Get(ctx context.Context, id string) (Rental, error)
}

// Rental is the slice of rental.Rental the adapter needs.
type Rental struct {
	ID              string
	OwnerID         string
	RenterID        string
	OperatorID      string // empty when no operator slot
	OperatorIsOwner bool
}

// Adapter implements review.RentalLookup.
type Adapter struct {
	DB      *gorm.DB
	Service RentalService
}

// New returns the adapter.
func New(db *gorm.DB, svc RentalService) *Adapter {
	return &Adapter{DB: db, Service: svc}
}

// GetParticipant implements review.RentalLookup. Returns
// review.ErrNotFound when the rental id is unknown.
func (a *Adapter) GetParticipant(ctx context.Context, rentalID string) (review.RentalParticipant, error) {
	if a == nil || a.Service == nil {
		return review.RentalParticipant{}, errors.New("review lookup: rental service not configured")
	}
	r, err := a.Service.Get(ctx, rentalID)
	if err != nil {
		return review.RentalParticipant{}, err
	}
	return rentalToParticipant(r), nil
}

// IsTerminal implements review.RentalLookup. Returns true when the
// rental reached F5-terminal state: devolução closed OR any damage
// claim resolved (renter_agreed / staff_resolved).
//
// The probe is intentionally scoped to a single SELECT against two
// tables with a UNION — cheap, independent of the F5 service state
// machine, and the F12 cleanup job is the one that eventually
// re-derives aggregates after window expiry.
func (a *Adapter) IsTerminal(ctx context.Context, rentalID string) (bool, error) {
	if a == nil || a.DB == nil {
		return false, errors.New("review lookup: db not configured")
	}
	var n int64
	row := a.DB.WithContext(ctx).Raw(`
		SELECT (
			(SELECT COUNT(*) FROM devolucoes
			 WHERE rental_id = ? AND state = 'closed')
			+
			(SELECT COUNT(*) FROM avaria_pedidos
			 WHERE rental_id = ?
			   AND state IN ('renter_agreed', 'staff_resolved'))
		)::bigint AS terminal`, rentalID, rentalID).Row()
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// rentalToParticipant translates the rental adapter shape into the
// review participant shape. The operator slot collapses when
// OperatorIsOwner is set (EC-5).
func rentalToParticipant(r Rental) review.RentalParticipant {
	parts := review.RentalParticipant{
		RentalID: r.ID,
		RenterID: r.RenterID,
		OwnerID:  r.OwnerID,
	}
	if !r.OperatorIsOwner && r.OperatorID != "" {
		parts.OperatorID = r.OperatorID
	}
	parts.OperatorIsOwner = r.OperatorIsOwner
	return parts
}
