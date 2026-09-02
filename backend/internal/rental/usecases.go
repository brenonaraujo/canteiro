package rental

import (
	"context"
	"errors"
	"time"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// CreateIntentInput is the service-layer input for the "create reservation intent" use case.
type CreateIntentInput struct {
	StartsAt              time.Time
	EndsAt                time.Time
	TenantID              string
	ListingID             string
	WithOperator          bool
	OperatorTermsAccepted bool
}

// CreateIntent validates preconditions and persists a `pending` rental.
func (s *Service) CreateIntent(ctx context.Context, in CreateIntentInput) (rental.Rental, rental.MoneyBreakdown, error) {
	if err := s.requireActiveTenant(ctx, in.TenantID); err != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, err
	}
	_, snap, err := s.requirePublishedListingSnapshot(ctx, in.ListingID)
	if err != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, err
	}
	if err = rental.ValidateWindow(in.StartsAt, in.EndsAt, s.cfg.Now(), time.Duration(snap.MinLeadTimeHours)*time.Hour); err != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, err
	}
	if err = s.requireNoOverlap(ctx, in.ListingID, in.StartsAt, in.EndsAt); err != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, err
	}
	if snap.Operator.Mode == "required" && !in.OperatorTermsAccepted {
		return rental.Rental{}, rental.MoneyBreakdown{}, rental.ErrOperatorTermsRequired
	}
	if in.WithOperator && snap.Operator.Mode == "none" {
		return rental.Rental{}, rental.MoneyBreakdown{}, rental.ErrOperatorNotAvailable
	}
	breakdown, err := rental.PriceQuote(rental.QuoteInput{
		Snapshot:      snap,
		StartsAt:      in.StartsAt,
		EndsAt:        in.EndsAt,
		WithOperator:  in.WithOperator,
		CommissionBPS: s.cfg.CommissionBPS,
	})
	if err != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, err
	}
	key := intentKeyFromWindow(in.ListingID, in.StartsAt, in.EndsAt)
	var existing rental.Rental
	var hb rental.MoneyBreakdown
	existing, hb, found, err := s.lookupExistingIntent(ctx, in.TenantID, in.ListingID, key)
	if err != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, err
	}
	if found {
		return existing, hb, nil
	}
	r := s.newPendingRental(in, snap, key)
	breakdown.ApplyToRental(&r)
	snapBytes, err := rental.MarshalSnapshot(snap)
	if err != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, err
	}
	persisted, err := s.repo.CreateIntent(ctx, r, snapBytes)
	if err != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, err
	}
	return persisted, breakdown, nil
}

// newPendingRental assembles the immutable initial Rental for a freshly
// created intent. IDs, timestamps, and the deterministic intent key live
// here; pricing fields are written separately by breakdown.ApplyToRental.
func (s *Service) newPendingRental(in CreateIntentInput, snap rental.ListingSnapshot, key string) rental.Rental {
	now := s.cfg.Now()
	return rental.Rental{
		ID:                    s.cfg.IDGen.String(),
		ListingID:             in.ListingID,
		TenantAccountID:       in.TenantID,
		ListingSnapshot:       snap,
		StartsAt:              in.StartsAt,
		EndsAt:                in.EndsAt,
		WithOperator:          in.WithOperator,
		OperatorTermsAccepted: in.OperatorTermsAccepted,
		State:                 rental.StatePending,
		IntentKey:             key,
		TenantClaimDebt:       "none",
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

// lookupExistingIntent returns the rental matching an idempotency key along
// with its hydrated MoneyBreakdown. found=false means the caller should
// create a new intent. Hydration of TotalCents/CommissionBaseCents mirrors
// the projection used by Read APIs — kept local so future schema changes
// stay isolated.
func (s *Service) lookupExistingIntent(ctx context.Context, tenantID, listingID, key string) (rental.Rental, rental.MoneyBreakdown, bool, error) {
	existing, found, err := s.repo.GetByIntentKey(ctx, tenantID, listingID, key)
	if err != nil || !found {
		return rental.Rental{}, rental.MoneyBreakdown{}, found, err
	}
	hb := rental.MoneyBreakdown{
		RentCents:           existing.RentCents,
		OperatorCents:       existing.OperatorCents,
		DepositCents:        existing.DepositCents,
		CommissionCents:     existing.CommissionCents,
		OwnerPayoutCents:    existing.OwnerPayoutCents,
		OperatorPayoutCents: existing.OperatorPayoutCents,
	}
	hb.TotalCents = existing.RentCents + existing.OperatorCents + existing.DepositCents
	hb.CommissionBaseCents = existing.RentCents + existing.OperatorCents
	return existing, hb, true, nil
}

// AuthorizeIntentInput is the input to the payment authorization.
type AuthorizeIntentInput struct {
	TenantID        string
	RentalID        string
	PaymentIntentID string
}

// AuthorizeIntent creates (or returns) PSP intent and persists it.
func (s *Service) AuthorizeIntent(ctx context.Context, in AuthorizeIntentInput) (PaymentIntent, error) {
	r, err := s.repo.GetByID(ctx, in.RentalID)
	if err != nil {
		return PaymentIntent{}, err
	}
	if !r.IsTenant(in.TenantID) {
		return PaymentIntent{}, rental.ErrForbidden
	}
	if existing, found, lookupErr := s.repo.GetPaymentIntent(ctx, in.RentalID); lookupErr != nil {
		return PaymentIntent{}, lookupErr
	} else if found {
		return existing, nil
	}
	if r.State != rental.StatePending {
		return PaymentIntent{}, rental.ErrInvalidTransition
	}
	if tenantHasOpenDebt(r) {
		return PaymentIntent{}, rental.ErrTenantHasDebt
	}
	key := "rental-" + r.ID + "-attempt-1"
	resp, err := s.payment.CreateIntent(ctx, CreateIntentRequest{
		RentalID:         r.ID,
		IdempotencyKey:   key,
		AmountCents:      r.RentCents + r.OperatorCents + r.DepositCents,
		DepositCents:     r.DepositCents,
		Currency:         s.cfg.DefaultCurrency,
		AcceptanceWindow: s.cfg.AcceptanceWindow,
		Metadata: map[string]string{
			"rental_id":  r.ID,
			"listing_id": r.ListingID,
			"tenant_id":  r.TenantAccountID,
		},
	})
	if err != nil {
		return PaymentIntent{}, err
	}
	intent := PaymentIntent{
		ID:                 s.cfg.IDGen.String(),
		RentalID:           r.ID,
		Provider:           resp.Provider,
		ProviderPaymentID:  resp.ProviderPaymentID,
		IdempotencyKey:     key,
		Attempt:            1,
		AmountCents:        r.RentCents + r.OperatorCents + r.DepositCents,
		DepositCents:       r.DepositCents,
		ExpectedTotalCents: r.RentCents + r.OperatorCents + r.DepositCents,
		Status:             resp.Status,
		FailureCode:        resp.FailureCode,
		FailureMessage:     resp.FailureMessage,
	}
	persisted, err := s.repo.UpsertPaymentIntent(ctx, intent)
	if err != nil {
		return PaymentIntent{}, err
	}
	return persisted, nil
}

// HandleWebhookEvent is called by the HTTP handler after PSP signature verification.
func (s *Service) HandleWebhookEvent(ctx context.Context, ev ProviderWebhookEvent) error {
	_, err := s.repo.RecordWebhookEvent(ctx, WebhookEvent{
		Provider:        ev.Provider,
		ProviderEventID: ev.ProviderEventID,
		EventType:       ev.EventType,
		RentalID:        ev.RentalID,
		PaymentIntentID: ev.PaymentIntentID,
		SignatureValid:  true,
		ReceivedAt:      s.cfg.Now(),
	})
	if err != nil {
		return err
	}
	r, err := s.repo.GetByID(ctx, ev.RentalID)
	if err != nil {
		return err
	}
	switch ev.EventType {
	case "payment.authorized":
		return s.markAuthorized(ctx, r, ev)
	case "payment.failed":
		return s.markFailed(ctx, r, ev)
	case "payment.refunded":
		return s.markRefunded(ctx, r)
	}
	return nil
}

func (s *Service) markAuthorized(ctx context.Context, r rental.Rental, ev ProviderWebhookEvent) error {
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	if ev.AmountCents != expected {
		return rental.ErrPaymentTotalMismatch
	}
	deadline := s.cfg.Now().Add(s.cfg.AcceptanceWindow)
	updated, err := s.repo.UpdateState(ctx, r.ID, r.State, rental.StateAuthorized, func(rt *rental.Rental) {
		rt.AcceptanceDeadlineAt = &deadline
	})
	if err != nil {
		if errors.Is(err, rental.ErrInvalidTransition) {
			return nil
		}
		return err
	}
	receipt := rental.ReceiptFromRental(updated, rental.MoneyBreakdown{
		RentCents:           updated.RentCents,
		OperatorCents:       updated.OperatorCents,
		DepositCents:        updated.DepositCents,
		TotalCents:          updated.RentCents + updated.OperatorCents + updated.DepositCents,
		CommissionBaseCents: updated.RentCents + updated.OperatorCents,
		CommissionCents:     updated.CommissionCents,
		OwnerPayoutCents:    updated.OwnerPayoutCents,
		OperatorPayoutCents: updated.OperatorPayoutCents,
	})
	receipt.IssuedAt = s.cfg.Now()
	if _, err := s.repo.SaveReceipt(ctx, receipt); err != nil {
		if errors.Is(err, rental.ErrReceiptAlreadyExists) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Service) markFailed(ctx context.Context, r rental.Rental, ev ProviderWebhookEvent) error {
	if _, err := s.repo.UpdateState(ctx, r.ID, r.State, rental.StatePending, func(rt *rental.Rental) {
		rt.State = rental.StatePending
	}); err != nil {
		if errors.Is(err, rental.ErrInvalidTransition) {
			return nil
		}
		return err
	}
	_, _ = s.repo.RecordWebhookEvent(ctx, WebhookEvent{
		Provider:        ev.Provider,
		ProviderEventID: ev.ProviderEventID + "-failure",
		EventType:       ev.EventType,
		RentalID:        ev.RentalID,
		ReceivedAt:      s.cfg.Now(),
	})
	return nil
}

func (s *Service) markRefunded(ctx context.Context, r rental.Rental) error {
	if _, err := s.repo.UpdateState(ctx, r.ID, r.State, rental.StateRefunded, nil); err != nil {
		if errors.Is(err, rental.ErrInvalidTransition) {
			return nil
		}
		return err
	}
	return nil
}

// AcceptInput is the input for the owner-side accept.
type AcceptInput struct {
	OwnerID  string
	RentalID string
}

// Accept is the owner's accept action (AC-6 + EC-3 enforcement: 12h window).
func (s *Service) Accept(ctx context.Context, in AcceptInput) (rental.Rental, error) {
	r, err := s.repo.GetByID(ctx, in.RentalID)
	if err != nil {
		return rental.Rental{}, err
	}
	if !r.IsOwner(in.OwnerID) {
		return rental.Rental{}, rental.ErrForbidden
	}
	if r.State != rental.StateAuthorized {
		return rental.Rental{}, rental.ErrInvalidTransition
	}
	if r.AcceptanceDeadlineAt == nil || !s.cfg.Now().Before(*r.AcceptanceDeadlineAt) {
		return rental.Rental{}, rental.ErrAcceptanceExpired
	}
	now := s.cfg.Now()
	return s.repo.UpdateState(ctx, r.ID, rental.StateAuthorized, rental.StateConfirmed, func(rt *rental.Rental) {
		rt.ConfirmedAt = &now
	})
}

// DeclineInput is the input for the owner-side decline.
type DeclineInput struct {
	OwnerID       string
	RentalID      string
	DeclineReason string
}

// Decline transitions authorized → declined.
func (s *Service) Decline(ctx context.Context, in DeclineInput) (rental.Rental, error) {
	r, err := s.repo.GetByID(ctx, in.RentalID)
	if err != nil {
		return rental.Rental{}, err
	}
	if !r.IsOwner(in.OwnerID) {
		return rental.Rental{}, rental.ErrForbidden
	}
	if r.State != rental.StateAuthorized {
		return rental.Rental{}, rental.ErrInvalidTransition
	}
	now := s.cfg.Now()
	return s.repo.UpdateState(ctx, r.ID, rental.StateAuthorized, rental.StateDeclined, func(rt *rental.Rental) {
		rt.DeclinedAt = &now
		rt.DeclineReason = in.DeclineReason
	})
}

// ExpireSweep is the deterministic expiry pass.
func (s *Service) ExpireSweep(ctx context.Context, now time.Time, batch int) (int, error) {
	rentals, err := s.repo.ListForOwner(ctx, "", []rental.State{rental.StateAuthorized})
	if err != nil {
		return 0, err
	}
	moved := 0
	for i := 0; i < len(rentals) && i < batch; i++ {
		r := rentals[i]
		if r.AcceptanceDeadlineAt == nil || !now.After(*r.AcceptanceDeadlineAt) {
			continue
		}
		if _, err := s.repo.UpdateState(ctx, r.ID, rental.StateAuthorized, rental.StateExpired, nil); err != nil {
			if errors.Is(err, rental.ErrInvalidTransition) {
				continue
			}
			return moved, err
		}
		moved++
	}
	return moved, nil
}

// GetReceipt returns the tenant receipt.
func (s *Service) GetReceipt(ctx context.Context, rentalID, tenantID string) (rental.Receipt, error) {
	r, err := s.repo.GetByID(ctx, rentalID)
	if err != nil {
		return rental.Receipt{}, err
	}
	if !r.IsTenant(tenantID) {
		return rental.Receipt{}, rental.ErrForbidden
	}
	rec, found, err := s.repo.GetReceipt(ctx, rentalID)
	if err != nil {
		return rental.Receipt{}, err
	}
	if !found {
		return rental.Receipt{}, rental.ErrNotFound
	}
	return rec, nil
}

// CancelPreAuthInput is the tenant's pre-acceptance cancel.
type CancelPreAuthInput struct {
	TenantID string
	RentalID string
}

// CancelPreAuth cancels a rental before owner acceptance.
func (s *Service) CancelPreAuth(ctx context.Context, in CancelPreAuthInput) (rental.Rental, error) {
	r, err := s.repo.GetByID(ctx, in.RentalID)
	if err != nil {
		return rental.Rental{}, err
	}
	if !r.IsTenant(in.TenantID) {
		return rental.Rental{}, rental.ErrForbidden
	}
	if r.State != rental.StatePending && r.State != rental.StateAuthorized {
		return rental.Rental{}, rental.ErrInvalidTransition
	}
	return s.repo.UpdateState(ctx, r.ID, r.State, rental.StateCancelled, nil)
}

// ListForTenant lists rentals for the tenant.
func (s *Service) ListForTenant(ctx context.Context, tenantID string) ([]rental.Rental, error) {
	return s.repo.ListForTenant(ctx, tenantID, nil)
}

// ListForOwner lists rentals over the owner's listings.
func (s *Service) ListForOwner(ctx context.Context, ownerID string) ([]rental.Rental, error) {
	return s.repo.ListForOwner(ctx, ownerID, nil)
}

// Get returns a single rental.
func (s *Service) Get(ctx context.Context, id string) (rental.Rental, error) {
	return s.repo.GetByID(ctx, id)
}
