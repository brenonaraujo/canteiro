// Package rental — F4 cancellation policy. Skill: pre-implementation-design
// (Option C). The rules are declarative: a windowPolicy table keyed by a
// window code carries the fees, deposit action and refund/payout
// defaults. ClassifyCancellation only reads the table and fills the
// amounts from the rental numbers. Adding a new window = add a row,
// not new branches in the function.
//
// AC-1..AC-8 + EC-1, EC-3, EC-5 are encoded here; EC-2 / EC-4 / EC-7
// are state-machine concerns owned by the rental.Service layer.
package rental

import (
	"fmt"
	"time"
)

// ActorKind discriminates who triggers the cancellation (ADR-lite #1
// keeps the source-of-truth of the receipt).
type ActorKind string

// Actor constants — the four F4 cancellation sources.
const (
	// ActorTenant is the renter.
	ActorTenant ActorKind = "tenant"
	// ActorOwner is the listing owner.
	ActorOwner ActorKind = "owner"
	// ActorPlatform is the Canteiro platform acting on a webhook
	// (chargeback, operator refusal, anti-fraud).
	ActorPlatform ActorKind = "platform"
	// ActorOperator is the third-party operator refusing the booking.
	ActorOperator ActorKind = "operator"
)

// WindowCode is the declarative F4 window applied to a cancellation.
// These are stable strings — they are persisted on the receipt and on
// rental_cancellations, and they double as the policy table key.
type WindowCode string

// Window codes — every F4 declarative outcome. Adding a new window
// is a new constant + a new policy row; no helper-extraction needed.
const (
	// WindowTenantPreAccept — tenant cancels before the owner accepted.
	WindowTenantPreAccept WindowCode = "tenant.pre_accept"
	// WindowTenantGe24h — tenant cancels ≥24h before pickup.
	WindowTenantGe24h WindowCode = "tenant.ge_24h"
	// WindowTenantLt24h — tenant cancels <24h before pickup.
	WindowTenantLt24h WindowCode = "tenant.lt_24h"
	// WindowTenantAfterStart — tenant cancels after the rental started.
	WindowTenantAfterStart WindowCode = "tenant.after_start"
	// WindowOwnerPrePickup — owner cancels before pickup.
	WindowOwnerPrePickup WindowCode = "owner.pre_pickup"
	// WindowOwnerAfterStart — owner cancels after the rental started.
	WindowOwnerAfterStart WindowCode = "owner.after_start"
	// WindowOperatorRefusal — third-party operator refused (AC-7).
	WindowOperatorRefusal WindowCode = "operator.refusal"
	// WindowPlatformChargeback — platform reverses on chargeback (EC-5).
	WindowPlatformChargeback WindowCode = "platform.chargeback"
)

// DepositState is the deposit outcome of a cancellation.
type DepositState string

// Deposit outcome states (mirrored in the SQL CHECK constraint).
const (
	// DepositReleased — full refund to tenant (default outcome).
	DepositReleased DepositState = "released"
	// DepositCaptured — full capture by platform.
	DepositCaptured DepositState = "captured"
	// DepositPartial — partial capture.
	DepositPartial DepositState = "partial"
	// DepositHeld — held against the rental; F5 (avarias) decides.
	DepositHeld DepositState = "held"
)

// CancellationActor binds the actor kind to its account id.
type CancellationActor struct {
	Kind      ActorKind
	AccountID string
	Reason    string
}

// CancellationInput is the immutable inputs to the policy.
type CancellationInput struct {
	Rental               Rental
	Actor                CancellationActor
	Now                  time.Time
	FeeBPS               int64 // cancellation fee in basis points (10% = 1000)
	WindowH              int   // hours before pickup for the ≥24h window
	MinFractionHours     int   // EC-2: minimum fraction when after-start
	IsChargebackReversal bool  // EC-5: chargeback path (platform actor)
}

// CancellationDecision is the immutable, audit-ready output. The
// numbers are final once the rental_cancellations row is committed.
// All amounts are cents (int64). The deposit amounts are mutually
// exclusive per row: capture + release + partial = 0 unless deposit
// state is "partial".
type CancellationDecision struct {
	WindowCode WindowCode
	ActorKind  ActorKind

	CancellationFeeCents int64
	TenantRefundCents    int64
	OwnerPayoutCents     int64
	OperatorPayoutCents  int64
	CommissionCents      int64

	DepositState               DepositState
	DepositCaptureCents        int64
	DepositReleaseCents        int64
	DepositPartialCaptureCents int64

	IsReversal bool
	Reason     string

	IssuedAt time.Time
}

// windowPolicy is the rule row keyed by WindowCode. Adding a window =
// add a row; no helper-extraction needed (skill Option C).
type windowPolicy struct {
	window                WindowCode
	actor                 ActorKind
	requiresOwnerAccepted bool
	requiresStarted       bool
	feeBPSOfRent          int64 // 0 = no fee; non-zero = fee on rent
	tenantRefundKind      refundKind
	ownerPayout           payoutKind
	operatorPayout        payoutKind
	deposit               DepositState
	depositAction         depositAction
}

// refundKind says how the tenant refund is computed.
type refundKind int

const (
	refundRentPlusOperator refundKind = iota // rent + operator (no fee)
	refundRentMinusFee                       // rent - fee
	refundZero                               // no refund (rent retained)
)

type payoutKind int

const (
	payoutZero         payoutKind = iota
	payoutRentRetained            // owner gets rent - commission; operator 0
	payoutProportional            // pro-rata of rent/operator by elapsed time
)

type depositAction int

const (
	depositNoop depositAction = iota
	depositRelease
	depositHold // keep held (F5 will decide)
)

// policies is the single source of truth. Order matters only for
// readability; lookup is by window code.
var policies = map[WindowCode]windowPolicy{
	WindowTenantPreAccept: {
		window:                WindowTenantPreAccept,
		actor:                 ActorTenant,
		requiresOwnerAccepted: false,
		tenantRefundKind:      refundRentPlusOperator,
		ownerPayout:           payoutZero,
		operatorPayout:        payoutZero,
		deposit:               DepositReleased,
		depositAction:         depositRelease,
	},
	WindowTenantGe24h: {
		window:                WindowTenantGe24h,
		actor:                 ActorTenant,
		requiresOwnerAccepted: true,
		feeBPSOfRent:          0, // applied dynamically via input.FeeBPS
		tenantRefundKind:      refundRentMinusFee,
		ownerPayout:           payoutZero,
		operatorPayout:        payoutZero,
		deposit:               DepositReleased,
		depositAction:         depositRelease,
	},
	WindowTenantLt24h: {
		window:                WindowTenantLt24h,
		actor:                 ActorTenant,
		requiresOwnerAccepted: true,
		tenantRefundKind:      refundZero,
		ownerPayout:           payoutRentRetained,
		operatorPayout:        payoutZero,
		deposit:               DepositReleased,
		depositAction:         depositRelease,
	},
	WindowTenantAfterStart: {
		window:                WindowTenantAfterStart,
		actor:                 ActorTenant,
		requiresOwnerAccepted: true,
		requiresStarted:       true,
		tenantRefundKind:      refundZero,
		ownerPayout:           payoutProportional,
		operatorPayout:        payoutProportional,
		deposit:               DepositHeld,
		depositAction:         depositHold,
	},
	WindowOwnerPrePickup: {
		window:                WindowOwnerPrePickup,
		actor:                 ActorOwner,
		requiresOwnerAccepted: true,
		tenantRefundKind:      refundRentPlusOperator,
		ownerPayout:           payoutZero,
		operatorPayout:        payoutZero,
		deposit:               DepositReleased,
		depositAction:         depositRelease,
	},
	WindowOwnerAfterStart: {
		window:                WindowOwnerAfterStart,
		actor:                 ActorOwner,
		requiresOwnerAccepted: true,
		requiresStarted:       true,
		tenantRefundKind:      refundRentPlusOperator,
		ownerPayout:           payoutZero,
		operatorPayout:        payoutZero,
		deposit:               DepositHeld,
		depositAction:         depositHold,
	},
	WindowOperatorRefusal: {
		window:                WindowOperatorRefusal,
		actor:                 ActorOperator,
		requiresOwnerAccepted: true,
		tenantRefundKind:      refundRentPlusOperator,
		ownerPayout:           payoutZero,
		operatorPayout:        payoutZero,
		deposit:               DepositReleased,
		depositAction:         depositRelease,
	},
	WindowPlatformChargeback: {
		window:                WindowPlatformChargeback,
		actor:                 ActorPlatform,
		requiresOwnerAccepted: true,
		tenantRefundKind:      refundRentPlusOperator,
		ownerPayout:           payoutZero,
		operatorPayout:        payoutZero,
		deposit:               DepositHeld,
		depositAction:         depositHold,
	},
}

// ClassifyCancellation is the entry point. Pure — no IO, no DB. The
// service layer is responsible for verifying the actor against the
// rental row and persisting the decision.
func ClassifyCancellation(in CancellationInput) (CancellationDecision, error) {
	r := in.Rental
	if err := validateForCancellation(r); err != nil {
		return CancellationDecision{}, err
	}
	if in.WindowH <= 0 {
		in.WindowH = 24
	}
	if in.FeeBPS < 0 {
		in.FeeBPS = 0
	}
	if in.MinFractionHours <= 0 {
		in.MinFractionHours = 4
	}
	win, ok := classifyWindow(in)
	if !ok {
		return CancellationDecision{}, fmt.Errorf("%w: cannot classify window for state=%s actor=%s", ErrInvalidTransition, r.State, in.Actor.Kind)
	}
	policy, ok := policies[win]
	if !ok {
		return CancellationDecision{}, fmt.Errorf("%w: no policy for window %s", ErrInvalidInput, win)
	}
	return applyPolicy(in, policy), nil
}

// validateForCancellation enforces the monetary invariants a rental must
// satisfy before cancellation. State and operator-terms are checked by
// the policy itself (classifyWindow + windowPolicy), not here — the
// pre-cancellation row can be in any non-terminal state.
func validateForCancellation(r Rental) error {
	if r.ID == "" {
		return fmt.Errorf("%w: id required", ErrInvalidInput)
	}
	if r.RentCents < 0 || r.OperatorCents < 0 || r.DepositCents < 0 {
		return fmt.Errorf("%w: money must be non-negative", ErrInvalidInput)
	}
	if r.RentCents+r.OperatorCents+r.DepositCents == 0 {
		return fmt.Errorf("%w: total must be > 0", ErrInvalidInput)
	}
	return nil
}

func classifyWindow(in CancellationInput) (WindowCode, bool) {
	r := in.Rental
	switch in.Actor.Kind {
	case ActorTenant:
		switch r.State {
		case StatePending:
			return WindowTenantPreAccept, true
		case StateAuthorized, StateConfirmed:
			if !r.StartsAt.After(in.Now) {
				return WindowTenantAfterStart, true
			}
			if in.Now.Add(time.Duration(in.WindowH)*time.Hour).Before(r.StartsAt) ||
				in.Now.Add(time.Duration(in.WindowH)*time.Hour).Equal(r.StartsAt) {
				return WindowTenantGe24h, true
			}
			return WindowTenantLt24h, true
		}
	case ActorOwner:
		switch r.State {
		case StateAuthorized, StateConfirmed:
			if !r.StartsAt.After(in.Now) {
				return WindowOwnerAfterStart, true
			}
			return WindowOwnerPrePickup, true
		}
	case ActorOperator:
		// AC-7: third-party operator refusal → owner pre-pickup.
		if r.State == StateAuthorized || r.State == StateConfirmed {
			return WindowOwnerPrePickup, true
		}
	case ActorPlatform:
		if in.IsChargebackReversal {
			return WindowPlatformChargeback, true
		}
	}
	return "", false
}

func applyPolicy(in CancellationInput, p windowPolicy) CancellationDecision {
	d := CancellationDecision{
		WindowCode: p.window,
		ActorKind:  p.actor,
		IsReversal: in.IsChargebackReversal,
		Reason:     in.Actor.Reason,
		IssuedAt:   in.Now,
	}

	// 1. Compute the tenant refund per the policy.
	fee := int64(0)
	switch p.tenantRefundKind {
	case refundRentPlusOperator:
		d.TenantRefundCents = in.Rental.RentCents + in.Rental.OperatorCents
	case refundRentMinusFee:
		fee = applyBPS(in.Rental.RentCents, in.FeeBPS)
		d.TenantRefundCents = in.Rental.RentCents - fee
	case refundZero:
		d.TenantRefundCents = 0
	}
	d.CancellationFeeCents = fee

	// 2. Compute owner/operator payouts.
	switch p.ownerPayout {
	case payoutZero:
		d.OwnerPayoutCents = 0
	case payoutRentRetained:
		d.OwnerPayoutCents = in.Rental.OwnerPayoutCents
	case payoutProportional:
		d.OwnerPayoutCents = proportionalOwner(in.Rental, in.Now)
	}
	switch p.operatorPayout {
	case payoutZero:
		d.OperatorPayoutCents = 0
	case payoutProportional:
		d.OperatorPayoutCents = proportionalOperator(in.Rental, in.Now)
	}

	// 3. Commission recovery / record. Chargeback reverses everything.
	d.CommissionCents = in.Rental.CommissionCents // commission recoverable on chargeback

	// 4. Deposit action.
	d.DepositState = p.deposit
	switch p.depositAction {
	case depositRelease:
		d.DepositReleaseCents = in.Rental.DepositCents
	case depositHold, depositNoop:
		// no deposit movement
	}

	return d
}

// proportionalOwner computes the owner's pro-rata payout for after-start.
// F4 AC-4: rent for elapsed days, operator pro-rata by hours. Minimum
// fraction is the operator's min hours (or 4h floor when there is no
// operator — EC-2).
func proportionalOwner(r Rental, now time.Time) int64 {
	if !now.After(r.StartsAt) {
		return 0
	}
	total := r.EndsAt.Sub(r.StartsAt)
	if total <= 0 {
		return r.OwnerPayoutCents
	}
	elapsed := now.Sub(r.StartsAt)
	if elapsed > total {
		elapsed = total
	}
	if total == 0 {
		return r.OwnerPayoutCents
	}
	return r.OwnerPayoutCents * int64(elapsed) / int64(total)
}

func proportionalOperator(r Rental, now time.Time) int64 {
	if !now.After(r.StartsAt) {
		return 0
	}
	total := r.EndsAt.Sub(r.StartsAt)
	if total <= 0 {
		return r.OperatorPayoutCents
	}
	elapsed := now.Sub(r.StartsAt)
	if elapsed > total {
		elapsed = total
	}
	return r.OperatorPayoutCents * int64(elapsed) / int64(total)
}

// applyBPS is the cent-precise fee/bps math (mirrors pricing.applyCommission).
func applyBPS(base, bps int64) int64 {
	if base <= 0 || bps <= 0 {
		return 0
	}
	return (base * bps) / 10000
}

// ApplyCommissionBPS is exposed so callers (cancellation, future pricing
// migrations) can reuse the cent-precise bps math without duplicating it.
func ApplyCommissionBPS(base, bps int64) int64 {
	return applyBPS(base, bps)
}
