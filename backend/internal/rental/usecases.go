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
	if err := s.requireNoOpenDebt(ctx, in.TenantID); err != nil {
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
// F4 EC-8: idempotent — a replay of the same (provider, provider_event_id)
// returns ErrIdempotencyConflict and the existing row is reused. The
// caller is expected to translate that to a 200 OK (the webhook
// contract is "every delivery returns 200").
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
		// EC-8: replay — the unique index already has this event id.
		if errors.Is(err, rental.ErrIdempotencyConflict) {
			return nil
		}
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
		return s.markRefunded(ctx, r, ev.ProviderEventID)
	case "payment.chargeback.created":
		return s.markChargeback(ctx, r, ev.ProviderEventID)
	case "deposit.captured", "deposit.released":
		// F5 (avarias) is the source of truth for these events; F4 only
		// records them. The webhook row above is the audit.
		return nil
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

func (s *Service) markRefunded(ctx context.Context, r rental.Rental, processorOpID string) error {
	if _, err := s.repo.UpdateState(ctx, r.ID, r.State, rental.StateRefunded, nil); err != nil {
		if errors.Is(err, rental.ErrInvalidTransition) {
			return nil
		}
		return err
	}
	// Mirror the refund in the cancellation table so the receipt
	// surfaces the AC-11 split. We synthesise an OwnerPrePickup-like
	// decision and persist it without moving the rental state again.
	if _, found, _ := s.repo.GetCancellationByRental(ctx, r.ID); !found {
		decision, err := rental.ClassifyCancellation(rental.CancellationInput{
			Rental:  r,
			Actor:   rental.CancellationActor{Kind: rental.ActorOwner, AccountID: r.ListingSnapshot.OwnerID, Reason: "refund"},
			Now:     s.cfg.Now(),
			FeeBPS:  s.cfg.CancellationFeeBPS,
			WindowH: s.cfg.CancellationWindowH,
		})
		if err == nil {
			now := s.cfg.Now()
			_, _ = s.repo.SaveCancellation(ctx, CancellationRecord{
				ID:                                   s.cfg.IDGen.String(),
				RentalID:                             r.ID,
				ActorID:                              r.ListingSnapshot.OwnerID,
				ActorKind:                            decision.ActorKind,
				WindowCode:                           decision.WindowCode,
				CancellationFeeCents:                 decision.CancellationFeeCents,
				TenantRefundCents:                    decision.TenantRefundCents,
				OwnerPayoutCentsAfterCancellation:    decision.OwnerPayoutCents,
				OperatorPayoutCentsAfterCancellation: decision.OperatorPayoutCents,
				CommissionCents:                      decision.CommissionCents,
				DepositState:                         decision.DepositState,
				DepositCaptureCents:                  decision.DepositCaptureCents,
				DepositReleaseCents:                  decision.DepositReleaseCents,
				DepositPartialCaptureCents:           decision.DepositPartialCaptureCents,
				ProcessorOperationID:                 processorOpID,
				ReversalReason:                       "refund",
				IssuedAt:                             now,
			})
		}
	}
	return nil
}

// markChargeback handles payment.chargeback.created events. Per
// ADR-lite #3 + EC-5: reverses the owner's payout, recovers the
// commission from the platform's books, blocks the tenant's account
// until manual unblock, and writes an immutable cancellation row
// with the WindowPlatformChargeback code.
func (s *Service) markChargeback(ctx context.Context, r rental.Rental, processorOpID string) error {
	now := s.cfg.Now()
	if _, err := s.repo.UpdateState(ctx, r.ID, r.State, rental.StateCancellationInProgress, func(rt *rental.Rental) {}); err != nil {
		if !errors.Is(err, rental.ErrInvalidTransition) {
			return err
		}
	}
	if _, err := s.repo.UpdateState(ctx, r.ID, rental.StateCancellationInProgress, rental.StateCancelled, func(rt *rental.Rental) {}); err != nil {
		if !errors.Is(err, rental.ErrInvalidTransition) {
			return err
		}
	}
	if _, found, _ := s.repo.GetCancellationByRental(ctx, r.ID); !found {
		decision, err := rental.ClassifyCancellation(rental.CancellationInput{
			Rental:               r,
			Actor:                rental.CancellationActor{Kind: rental.ActorPlatform, AccountID: "platform", Reason: "chargeback"},
			Now:                  now,
			FeeBPS:               0,
			WindowH:              s.cfg.CancellationWindowH,
			IsChargebackReversal: true,
		})
		if err == nil {
			_, _ = s.repo.SaveCancellation(ctx, CancellationRecord{
				ID:                                   s.cfg.IDGen.String(),
				RentalID:                             r.ID,
				ActorID:                              "platform",
				ActorKind:                            decision.ActorKind,
				WindowCode:                           decision.WindowCode,
				CancellationFeeCents:                 decision.CancellationFeeCents,
				TenantRefundCents:                    decision.TenantRefundCents,
				OwnerPayoutCentsAfterCancellation:    decision.OwnerPayoutCents,
				OperatorPayoutCentsAfterCancellation: decision.OperatorPayoutCents,
				CommissionCents:                      decision.CommissionCents,
				DepositState:                         decision.DepositState,
				DepositCaptureCents:                  decision.DepositCaptureCents,
				DepositReleaseCents:                  decision.DepositReleaseCents,
				DepositPartialCaptureCents:           decision.DepositPartialCaptureCents,
				ProcessorOperationID:                 processorOpID,
				ReversalReason:                       "chargeback",
				IssuedAt:                             now,
			})
		}
	}
	// EC-5: block tenant account until manual unblock.
	return s.repo.SetTenantChargebackBlocked(ctx, r.TenantAccountID, true)
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

// Cancel is the F4 declarative cancellation entry point (AC-1..AC-8,
// EC-1, EC-3, EC-5, EC-6). The caller is the tenant, owner or platform;
// the policy derives the window + amounts, the receipt is updated with
// AC-11 fields, and the rental moves through
// `authorized|confirmed → cancellation_in_progress → cancelled`.
//
// Idempotent: a second call for the same rental returns the existing
// record (200) regardless of caller actor, since the platform is the
// authoritative source for the F4 row. The state machine prevents a
// second live transition from `cancelled` (terminal).
func (s *Service) Cancel(ctx context.Context, in CancelInput) (CancellationResult, error) {
	if !s.cfg.FeatureF4Enabled {
		return CancellationResult{}, rental.ErrInvalidTransition
	}
	r, err := s.repo.GetByID(ctx, in.RentalID)
	if err != nil {
		return CancellationResult{}, err
	}
	// EC-1: a rental already cancelled is a replay; return the row.
	if r.State == rental.StateCancelled {
		existing, found, lookupErr := s.repo.GetCancellationByRental(ctx, in.RentalID)
		if lookupErr != nil {
			return CancellationResult{}, lookupErr
		}
		if !found {
			return CancellationResult{}, rental.ErrNotFound
		}
		return CancellationResult{Cancellation: existing, Rental: r}, nil
	}
	if guardErr := s.guardCancellationActor(r, in); guardErr != nil {
		return CancellationResult{}, guardErr
	}
	decision, err := rental.ClassifyCancellation(rental.CancellationInput{
		Rental: r,
		Actor: rental.CancellationActor{
			Kind:      in.ActorKind,
			AccountID: in.CallerAccountID,
			Reason:    in.Reason,
		},
		Now:                  s.cfg.Now(),
		FeeBPS:               s.effectiveCancellationFeeBPS(in),
		WindowH:              s.cfg.CancellationWindowH,
		MinFractionHours:     s.cfg.MinFractionHours,
		IsChargebackReversal: in.IsChargebackReversal,
	})
	if err != nil {
		return CancellationResult{}, err
	}
	terminal, err := s.moveToCancelled(ctx, r)
	if err != nil {
		return CancellationResult{}, err
	}
	now := s.cfg.Now()
	record := decisionToCancellationRecord(s.cfg.IDGen.String(), terminal.ID, in, decision, now)
	persisted, err := s.repo.SaveCancellation(ctx, record)
	if err != nil {
		return CancellationResult{}, err
	}
	if err := s.writeCancellationReceipt(ctx, terminal, now); err != nil {
		return CancellationResult{}, err
	}
	// EC-5: chargeback blocks the tenant account until manual unblock.
	if in.IsChargebackReversal {
		if err := s.repo.SetTenantChargebackBlocked(ctx, terminal.TenantAccountID, true); err != nil {
			return CancellationResult{}, err
		}
	}
	return CancellationResult{Cancellation: persisted, Rental: terminal}, nil
}

// guardCancellationActor enforces who is allowed to cancel the rental
// on a given endpoint. Tenant may cancel their own; owner may cancel
// a rental on their listing; platform calls come from the webhook.
func (s *Service) guardCancellationActor(r rental.Rental, in CancelInput) error {
	switch in.ActorKind {
	case rental.ActorTenant:
		if !r.IsTenant(in.CallerAccountID) {
			return rental.ErrForbidden
		}
	case rental.ActorOwner:
		if !r.IsOwner(in.CallerAccountID) {
			return rental.ErrForbidden
		}
	case rental.ActorPlatform:
		// trusted — webhook-only path
	default:
		return rental.ErrInvalidInput
	}
	return nil
}

// effectiveCancellationFeeBPS honours an override when supplied,
// otherwise falls back to the configured F4 fee.
func (s *Service) effectiveCancellationFeeBPS(in CancelInput) int64 {
	if in.FeeBPSOverride > 0 {
		return in.FeeBPSOverride
	}
	return s.cfg.CancellationFeeBPS
}

// moveToCancelled drives the rental through the two F4 transitions
// (→ cancellation_in_progress → cancelled). Returns the terminal
// rental on success. UpdateState is the lock gate.
func (s *Service) moveToCancelled(ctx context.Context, r rental.Rental) (rental.Rental, error) {
	intermediate, err := s.repo.UpdateState(ctx, r.ID, r.State, rental.StateCancellationInProgress, func(rt *rental.Rental) {})
	if err != nil {
		return rental.Rental{}, err
	}
	return s.repo.UpdateState(ctx, intermediate.ID, rental.StateCancellationInProgress, rental.StateCancelled, func(rt *rental.Rental) {})
}

// decisionToCancellationRecord lifts the classificador output into the
// service-layer persistence shape, attaching the PSP id and reason.
func decisionToCancellationRecord(id, rentalID string, in CancelInput, d rental.CancellationDecision, now time.Time) CancellationRecord {
	return CancellationRecord{
		ID:                                   id,
		RentalID:                             rentalID,
		ActorID:                              in.CallerAccountID,
		ActorKind:                            d.ActorKind,
		WindowCode:                           d.WindowCode,
		CancellationFeeCents:                 d.CancellationFeeCents,
		TenantRefundCents:                    d.TenantRefundCents,
		OwnerPayoutCentsAfterCancellation:    d.OwnerPayoutCents,
		OperatorPayoutCentsAfterCancellation: d.OperatorPayoutCents,
		CommissionCents:                      d.CommissionCents,
		DepositState:                         d.DepositState,
		DepositCaptureCents:                  d.DepositCaptureCents,
		DepositReleaseCents:                  d.DepositReleaseCents,
		DepositPartialCaptureCents:           d.DepositPartialCaptureCents,
		ProcessorOperationID:                 in.ProcessorOpID,
		ReversalReason:                       in.Reason,
		IssuedAt:                             now,
	}
}

// writeCancellationReceipt writes the AC-11 receipt the first time.
// A subsequent cancel of an already-receipted rental leaves the row
// alone — the receipt is immutable per ADR-lite #2.
func (s *Service) writeCancellationReceipt(ctx context.Context, terminal rental.Rental, now time.Time) error {
	_, found, err := s.repo.GetReceipt(ctx, terminal.ID)
	if err != nil {
		return err
	}
	if found {
		// Receipt is immutable; future cancels won't overwrite it.
		return nil
	}
	receipt := rental.ReceiptFromRental(terminal, rental.MoneyBreakdown{
		RentCents:           terminal.RentCents,
		OperatorCents:       terminal.OperatorCents,
		DepositCents:        terminal.DepositCents,
		TotalCents:          terminal.RentCents + terminal.OperatorCents + terminal.DepositCents,
		CommissionBaseCents: terminal.RentCents + terminal.OperatorCents,
		CommissionCents:     terminal.CommissionCents,
		OwnerPayoutCents:    terminal.OwnerPayoutCents,
		OperatorPayoutCents: terminal.OperatorPayoutCents,
	})
	receipt.IssuedAt = now
	if _, err := s.repo.SaveReceipt(ctx, receipt); err != nil && !errors.Is(err, rental.ErrReceiptAlreadyExists) {
		return err
	}
	return nil
}

// GetCancellation returns the cancellation record (immutable, ADR-lite #2)
// for either party of the rental — tenant or owner. Returns ErrNotFound
// when no cancellation exists yet.
func (s *Service) GetCancellation(ctx context.Context, rentalID, callerAccountID string) (CancellationRecord, error) {
	r, err := s.repo.GetByID(ctx, rentalID)
	if err != nil {
		return CancellationRecord{}, err
	}
	if !r.IsTenant(callerAccountID) && !r.IsOwner(callerAccountID) {
		return CancellationRecord{}, rental.ErrForbidden
	}
	c, found, err := s.repo.GetCancellationByRental(ctx, rentalID)
	if err != nil {
		return CancellationRecord{}, err
	}
	if !found {
		return CancellationRecord{}, rental.ErrNotFound
	}
	return c, nil
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
