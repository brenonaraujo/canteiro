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
// the data model (ratee_user_id column, EC-5 collapse), but the
// production adapter cannot populate the ratee when
// OperatorIsOwner is false because the snapshot has no operator
// account to point at. In that case OperatorID stays empty, and
// the review service's raterScopeAllowed check rejects the
// operator scope (AllowedRaterScopes requires OperatorID != "" AND
// OperatorID != OwnerID to surface ScopeOperator to the renter).
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
	// Non-owner operator (V1): no operator_account_id in F2, so
	// OperatorID stays empty. AllowedRaterScopes only surfaces
	// ScopeOperator to the renter when OperatorID != "" AND
	// OperatorID != OwnerID; the allow-list check at
	// service.go's raterScopeAllowed therefore returns
	// ErrNotParticipant for any caller that submits the
	// operator scope under V1.
	return out
}
