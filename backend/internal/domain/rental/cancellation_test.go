package rental_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// ClassifyCancellation is the F4 declarative policy (Pilar 1 / Pilar 2).
// Skill: pre-implementation-design — Option C: a windowPolicy table
// keyed by window code carries the rules; the function only reads the
// table and fills in the amounts from the rental numbers. Tests are
// table-driven across AC-1..AC-8 and EC-1..EC-8.

func baseRental() rental.Rental {
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	return rental.Rental{
		ID:              "r1",
		ListingID:       "L1",
		TenantAccountID: "T1",
		State:           rental.StateAuthorized,
		StartsAt:        now.Add(48 * time.Hour), // 48h ahead of pickup
		EndsAt:          now.Add(72 * time.Hour),
		RentCents:       10000,
		OperatorCents:   0,
		DepositCents:    20000,
		CommissionCents: 1200, // 12% over rent (no operator)
		OwnerPayoutCents: 8800,
		OperatorPayoutCents: 0,
		ListingSnapshot: rental.ListingSnapshot{
			OwnerID:  "O1",
			Operator: rental.OperatorSnapshot{Mode: "none", IsOwner: false},
		},
	}
}

func TestClassifyCancellation_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*rental.Rental)
		actor   rental.CancellationActor
		now     time.Time
		want    rental.CancellationDecision
		wantErr error
	}{
		{
			name: "AC-1 tenant pre-accept — full refund, deposit released",
			mutate: func(r *rental.Rental) {
				r.State = rental.StatePending
			},
			actor: rental.CancellationActor{Kind: rental.ActorTenant, AccountID: "T1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowTenantPreAccept,
				ActorKind:          rental.ActorTenant,
				CancellationFeeCents: 0,
				TenantRefundCents:  10000,
				OwnerPayoutCents:   0,
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositReleased,
				DepositReleaseCents: 20000,
				CommissionCents:    1200,
			},
		},
		{
			name: "AC-2 tenant ge 24h — 10% fee, deposit released, operator 0",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateAuthorized
			},
			actor: rental.CancellationActor{Kind: rental.ActorTenant, AccountID: "T1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowTenantGe24h,
				ActorKind:          rental.ActorTenant,
				CancellationFeeCents: 1000,
				TenantRefundCents:  9000,
				OwnerPayoutCents:   0,
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositReleased,
				DepositReleaseCents: 20000,
				CommissionCents:    1200,
			},
		},
		{
			name: "AC-3 tenant lt 24h, not started — rent retained, deposit released",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateAuthorized
				r.StartsAt = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) // 4h ahead
			},
			actor: rental.CancellationActor{Kind: rental.ActorTenant, AccountID: "T1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowTenantLt24h,
				ActorKind:          rental.ActorTenant,
				CancellationFeeCents: 0,
				TenantRefundCents:  0,
				OwnerPayoutCents:   8800, // owner keeps (rent - 12% commission)
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositReleased,
				DepositReleaseCents: 20000,
				CommissionCents:    1200,
			},
		},
		{
			name: "AC-4 tenant after_start — rent proportional + deposit held",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateConfirmed
				r.StartsAt = time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
				r.EndsAt = r.StartsAt.Add(24 * time.Hour)
			},
			actor: rental.CancellationActor{Kind: rental.ActorTenant, AccountID: "T1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowTenantAfterStart,
				ActorKind:          rental.ActorTenant,
				CancellationFeeCents: 0,
				TenantRefundCents:  0,
				OwnerPayoutCents:   8800, // full rent retained (after_start: 100% elapsed)
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositHeld,
				CommissionCents:    1200,
			},
		},
		{
			name: "AC-5 owner pre-pickup — full refund, operator 0",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateAuthorized
			},
			actor: rental.CancellationActor{Kind: rental.ActorOwner, AccountID: "O1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowOwnerPrePickup,
				ActorKind:          rental.ActorOwner,
				CancellationFeeCents: 0,
				TenantRefundCents:  10000,
				OwnerPayoutCents:   0,
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositReleased,
				DepositReleaseCents: 20000,
				CommissionCents:    1200,
			},
		},
		{
			name: "AC-5b owner pre-pickup with operator — operator gets nothing here (pre-pickup = full refund)",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateAuthorized
				r.OperatorCents = 6000
				r.CommissionCents = 1920
				r.OwnerPayoutCents = 8080
				r.OperatorPayoutCents = 6000
				r.OperatorTermsAccepted = true
				r.ListingSnapshot.Operator = rental.OperatorSnapshot{
					Mode: "required", IsOwner: false, HourlyRateCents: 1000, MinHours: 4,
				}
			},
			actor: rental.CancellationActor{Kind: rental.ActorOwner, AccountID: "O1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowOwnerPrePickup,
				ActorKind:          rental.ActorOwner,
				CancellationFeeCents: 0,
				TenantRefundCents:  16000, // rent + operator
				OwnerPayoutCents:   0,
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositReleased,
				DepositReleaseCents: 20000,
				CommissionCents:    1920,
			},
		},
		{
			name: "AC-6 owner after_start — full refund, deposit held",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateConfirmed
				r.StartsAt = time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
				r.EndsAt = r.StartsAt.Add(24 * time.Hour)
			},
			actor: rental.CancellationActor{Kind: rental.ActorOwner, AccountID: "O1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowOwnerAfterStart,
				ActorKind:          rental.ActorOwner,
				CancellationFeeCents: 0,
				TenantRefundCents:  10000,
				OwnerPayoutCents:   0,
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositHeld,
				CommissionCents:    1200,
			},
		},
		{
			name: "AC-7 third-party operator refused — owner pre-pickup",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateAuthorized
			},
			actor: rental.CancellationActor{Kind: rental.ActorOperator, AccountID: "OP1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowOwnerPrePickup,
				ActorKind:          rental.ActorOperator,
				CancellationFeeCents: 0,
				TenantRefundCents:  10000,
				OwnerPayoutCents:   0,
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositReleased,
				DepositReleaseCents: 20000,
				CommissionCents:    1200,
			},
		},
		{
			name: "AC-8 operator is the owner — single payout retained (after 12% commission)",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateAuthorized
				r.OperatorCents = 4000
				r.CommissionCents = 1680
				r.OwnerPayoutCents = 12320
				r.OperatorPayoutCents = 0
				r.OperatorTermsAccepted = true
				r.ListingSnapshot.Operator = rental.OperatorSnapshot{
					Mode: "optional", IsOwner: true, HourlyRateCents: 1000, MinHours: 4,
				}
			},
			actor: rental.CancellationActor{Kind: rental.ActorTenant, AccountID: "T1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowTenantGe24h, // 48h to pickup → ge24h
				ActorKind:          rental.ActorTenant,
				CancellationFeeCents: 1000, // 10% of rent 10000
				TenantRefundCents:  9000,
				OwnerPayoutCents:   0,
				OperatorPayoutCents: 0,
				DepositState:       rental.DepositReleased,
				DepositReleaseCents: 20000,
				CommissionCents:    1680,
			},
		},
		{
			name: "EC-1 tenant same day lt 24h — window lt24h",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateAuthorized
				r.StartsAt = time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
			},
			actor: rental.CancellationActor{Kind: rental.ActorTenant, AccountID: "T1"},
			now:   time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			want: rental.CancellationDecision{
				WindowCode:         rental.WindowTenantLt24h,
				ActorKind:          rental.ActorTenant,
				CancellationFeeCents: 0,
				TenantRefundCents:  0,
				OwnerPayoutCents:   8800,
				DepositState:       rental.DepositReleased,
				DepositReleaseCents: 20000,
				CommissionCents:    1200,
			},
		},
		{
			name: "rejects tenant cancellation after terminal",
			mutate: func(r *rental.Rental) {
				r.State = rental.StateCancelled
			},
			actor:   rental.CancellationActor{Kind: rental.ActorTenant, AccountID: "T1"},
			now:     time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			wantErr: rental.ErrInvalidTransition,
		},
		{
			name: "rejects owner cancellation in pending (no payment)",
			mutate: func(r *rental.Rental) {
				r.State = rental.StatePending
			},
			actor:   rental.CancellationActor{Kind: rental.ActorOwner, AccountID: "O1"},
			now:     time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
			wantErr: rental.ErrInvalidTransition,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := baseRental()
			if tt.mutate != nil {
				tt.mutate(&r)
			}
			d, err := rental.ClassifyCancellation(rental.CancellationInput{
				Rental:  r,
				Actor:   tt.actor,
				Now:     tt.now,
				FeeBPS:  1000,
				WindowH: 24,
				MinFractionHours: 4,
			})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want.WindowCode, d.WindowCode, "window code")
			require.Equal(t, tt.want.CancellationFeeCents, d.CancellationFeeCents, "fee")
			require.Equal(t, tt.want.TenantRefundCents, d.TenantRefundCents, "refund")
			require.Equal(t, tt.want.OwnerPayoutCents, d.OwnerPayoutCents, "owner payout")
			require.Equal(t, tt.want.OperatorPayoutCents, d.OperatorPayoutCents, "operator payout")
			require.Equal(t, tt.want.DepositState, d.DepositState, "deposit state")
			require.Equal(t, tt.want.DepositCaptureCents, d.DepositCaptureCents, "deposit capture")
			require.Equal(t, tt.want.DepositReleaseCents, d.DepositReleaseCents, "deposit release")
			require.Equal(t, tt.want.CommissionCents, d.CommissionCents, "commission")
		})
	}
}

func TestClassifyCancellation_AC9_CommissionExcludesDeposit(t *testing.T) {
	t.Parallel()
	r := baseRental()
	r.State = rental.StateAuthorized
	r.RentCents = 50000
	r.OperatorCents = 10000
	r.DepositCents = 30000
	r.CommissionCents = rental.ApplyCommissionBPS(r.RentCents+r.OperatorCents, 1200)
	r.OwnerPayoutCents = r.RentCents + r.OperatorCents - r.CommissionCents
	d, err := rental.ClassifyCancellation(rental.CancellationInput{
		Rental: r,
		Actor:  rental.CancellationActor{Kind: rental.ActorOwner, AccountID: "O1"},
		Now:    time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
		FeeBPS: 1000,
		WindowH: 24,
	})
	require.NoError(t, err)
	require.Equal(t, rental.WindowOwnerPrePickup, d.WindowCode)
	require.Equal(t, int64(60000), d.TenantRefundCents, "rent+operator (deposit NOT in refund)")
	require.Equal(t, int64(30000), d.DepositReleaseCents, "deposit fully released")
	require.Equal(t, int64(0), d.OwnerPayoutCents, "owner pre-pickup = 0 payout")
	require.Equal(t, int64(7200), d.CommissionCents, "12% of (50000+10000) = 7200")
}

func TestClassifyCancellation_AC12_FreshWindowOnReentry(t *testing.T) {
	t.Parallel()
	r := baseRental()
	r.State = rental.StateConfirmed
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	r.StartsAt = now.Add(-2 * time.Hour) // already started
	r.EndsAt = now.Add(22 * time.Hour)
	d, err := rental.ClassifyCancellation(rental.CancellationInput{
		Rental:  r,
		Actor:   rental.CancellationActor{Kind: rental.ActorTenant, AccountID: "T1"},
		Now:     now,
		FeeBPS:  1000,
		WindowH: 24,
	})
	require.NoError(t, err)
	require.Equal(t, rental.WindowTenantAfterStart, d.WindowCode)
}

func TestClassifyCancellation_ChargebackReversal_EC5(t *testing.T) {
	t.Parallel()
	r := baseRental()
	r.State = rental.StateConfirmed
	d, err := rental.ClassifyCancellation(rental.CancellationInput{
		Rental:              r,
		Actor:               rental.CancellationActor{Kind: rental.ActorPlatform, AccountID: "platform", Reason: "chargeback"},
		Now:                 time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
		FeeBPS:              0,
		WindowH:             24,
		IsChargebackReversal: true,
	})
	require.NoError(t, err)
	require.Equal(t, rental.WindowPlatformChargeback, d.WindowCode)
	require.Equal(t, int64(10000), d.TenantRefundCents, "chargeback = full refund of rent to tenant")
	require.Equal(t, int64(0), d.OwnerPayoutCents, "owner payout reversed")
	require.Equal(t, int64(0), d.OperatorPayoutCents, "operator payout reversed")
	require.Equal(t, int64(1200), d.CommissionCents)
	require.True(t, d.IsReversal)
}
