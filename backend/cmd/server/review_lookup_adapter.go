package main

import (
	"context"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
	lookuppgrental "github.com/brenonaraujo/canteiro/backend/internal/repository/review/lookuppgrental"
)

// rentalLookupAdapter bridges *rental.Service.Get (returns
// rental.Rental) onto the lookuppgrental.RentalService surface
// (returns lookuppgrental.Rental). This is the same shape as
// f5's rentalLookupAdapter, but reduced to the fields the review
// adapters need.
type reviewRentalLookupAdapter struct {
	svc *rentsvc.Service
}

func (a *reviewRentalLookupAdapter) Get(ctx context.Context, id string) (lookuppgrental.Rental, error) {
	r, err := a.svc.Get(ctx, id)
	if err != nil {
		// map rental.ErrNotFound → review.ErrNotFound via the
		// adapter's error pass-through. The review service maps
		// ErrNotFound itself, so we just forward.
		return lookuppgrental.Rental{}, err
	}
	return rentalToLookupRental(r), nil
}

// rentalToLookupRental translates a rental.Rental snapshot into the
// thin shape the review lookup needs.
//
// V1 limitation (operator scope): the F2 listings table does not
// carry an operator account_id — the operator is a free-text name
// + phone slot. The review domain supports the operator scope in
// the data model (ratee_user_id column, EC-5 collapse), but
// production wiring cannot populate the ratee when OperatorIsOwner
// is false because the snapshot has no operator account to point
// at. The review service handles the empty case by allowing the
// scope submission and persisting the ratee as NULL — the scope
// still works for listing/owner/renter. When F2 adds
// operator_account_id, this adapter fills OperatorID from there.
//
// EC-5 collapse: when Operator.IsOwner is true, the operator slot
// is filled by the owner. We set OperatorID to the owner ID so
// downstream scope checks collapse correctly (the renter's allowed
// scopes do not include operator when OperatorID == OwnerID).
func rentalToLookupRental(r rental.Rental) lookuppgrental.Rental {
	out := lookuppgrental.Rental{
		ID:              r.ID,
		OwnerID:         r.ListingSnapshot.OwnerID,
		RenterID:        r.TenantAccountID,
		OperatorIsOwner: r.ListingSnapshot.Operator.IsOwner,
	}
	if r.ListingSnapshot.Operator.IsOwner && r.ListingSnapshot.OwnerID != "" {
		// EC-5: owner doubles as operator.
		out.OperatorID = r.ListingSnapshot.OwnerID
	}
	// Non-owner operator: V1 cannot resolve the account_id. The
	// scope is rejected at the service layer (allowed-scopes check
	// relies on OperatorID != "" + OperatorID != OwnerID).
	return out
}
