package rental_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

// F4 Cancel + GetCancellation + markChargeback coverage.
// Skill: pre-implementation-design — table-driven, one row per AC + EC.

// withF4 turns the feature flag on for the service instance.
func withF4(cfg *rentsvc.Config) {
	cfg.FeatureF4Enabled = true
	cfg.CancellationFeeBPS = 1000
	cfg.CancellationWindowH = 24
	cfg.MinFractionHours = 4
	cfg.Now = func() time.Time { return time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC) }
}

// seedAuthorizedRental writes a rental in `authorized` state ready for F4.
func seedAuthorizedRental(t *testing.T, repo *fakeRepo, tenantID, ownerID string) string {
	t.Helper()
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	r := rental.Rental{
		ID:                  "rental-f4-" + tenantID,
		ListingID:           "L1",
		TenantAccountID:     tenantID,
		State:               rental.StateAuthorized,
		StartsAt:            now.Add(48 * time.Hour),
		EndsAt:              now.Add(72 * time.Hour),
		RentCents:           10000,
		OperatorCents:       0,
		DepositCents:        20000,
		CommissionCents:     1200,
		OwnerPayoutCents:    8800,
		OperatorPayoutCents: 0,
		ListingSnapshot: rental.ListingSnapshot{
			OwnerID:          ownerID,
			Title:            "Furadeira",
			Category:         "electric",
			PriceUnit:        "hour",
			PriceAmountCents: 5000,
			DepositCents:     20000,
			MinLeadTimeHours: 12,
			Operator:         rental.OperatorSnapshot{Mode: "none"},
		},
	}
	repo.rentals[r.ID] = r
	return r.ID
}

func newF4Service(t *testing.T, repo *fakeRepo) *rentsvc.Service {
	t.Helper()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{
		"T1": activeTenant("T1"),
		"O1": activeTenant("O1"),
	}}
	return newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()}, withF4)
}

func TestCancel_FeatureFlagOff_Rejects(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newService(t, serviceDeps{
		Repo: repo, Listing: &fakeListing{listing: publishedListing("L1", "Ana", "none")},
		Account:  &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}},
		Provider: newFakeProvider(),
	})
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	_, err := svc.Cancel(context.Background(), rentsvc.CancelInput{
		CallerAccountID: "T1",
		RentalID:        rentalID,
		ActorKind:       rental.ActorTenant,
	})
	require.ErrorIs(t, err, rental.ErrInvalidTransition)
}

func TestCancel_TenantHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	res, err := svc.Cancel(context.Background(), rentsvc.CancelInput{
		CallerAccountID: "T1",
		RentalID:        rentalID,
		ActorKind:       rental.ActorTenant,
		Reason:          "change of plans",
	})
	require.NoError(t, err)
	require.Equal(t, rental.StateCancelled, res.Rental.State)
	require.Equal(t, rental.ActorTenant, res.Cancellation.ActorKind)
	require.Equal(t, rental.WindowTenantGe24h, res.Cancellation.WindowCode)
	require.Equal(t, int64(9000), res.Cancellation.TenantRefundCents, "rent - 10% fee")
	require.Equal(t, int64(0), res.Cancellation.OwnerPayoutCentsAfterCancellation)
	require.Equal(t, rental.DepositReleased, res.Cancellation.DepositState)
	require.Equal(t, int64(20000), res.Cancellation.DepositReleaseCents)
	require.Equal(t, int64(1000), res.Cancellation.CancellationFeeCents)
}

func TestCancel_OwnerForbiddenForOtherListing(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	_, err := svc.Cancel(context.Background(), rentsvc.CancelInput{
		CallerAccountID: "intruder",
		RentalID:        rentalID,
		ActorKind:       rental.ActorOwner,
	})
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestCancel_PlatformReversal_BlocksTenantAccount(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	res, err := svc.Cancel(context.Background(), rentsvc.CancelInput{
		CallerAccountID:      "platform",
		RentalID:             rentalID,
		ActorKind:            rental.ActorPlatform,
		ProcessorOpID:        "ch_TEST",
		IsChargebackReversal: true,
	})
	require.NoError(t, err)
	require.Equal(t, rental.WindowPlatformChargeback, res.Cancellation.WindowCode)
	require.True(t, repo.blockedAccts["T1"], "tenant account blocked on chargeback")
}

func TestCancel_ReplaysIdempotent(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	first, err := svc.Cancel(context.Background(), rentsvc.CancelInput{
		CallerAccountID: "T1",
		RentalID:        rentalID,
		ActorKind:       rental.ActorTenant,
	})
	require.NoError(t, err)
	second, err := svc.Cancel(context.Background(), rentsvc.CancelInput{
		CallerAccountID: "O1",
		RentalID:        rentalID,
		ActorKind:       rental.ActorOwner,
	})
	require.NoError(t, err)
	require.Equal(t, first.Cancellation.ID, second.Cancellation.ID,
		"second cancel returns the existing record (EC-1 anti-double-penalty)")
}

func TestGetCancellation_OnlyPartiesAllowed(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	_, err := svc.Cancel(context.Background(), rentsvc.CancelInput{
		CallerAccountID: "T1",
		RentalID:        rentalID,
		ActorKind:       rental.ActorTenant,
	})
	require.NoError(t, err)
	_, err = svc.GetCancellation(context.Background(), rentalID, "T1")
	require.NoError(t, err)
	_, err = svc.GetCancellation(context.Background(), rentalID, "O1")
	require.NoError(t, err)
	_, err = svc.GetCancellation(context.Background(), rentalID, "intruder")
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestGetCancellation_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	_, err := svc.GetCancellation(context.Background(), rentalID, "T1")
	require.ErrorIs(t, err, rental.ErrNotFound)
}

func TestHandleWebhookEvent_Chargeback_ReversesAndBlocks(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	err := svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider:        "noop",
		ProviderEventID: "evt_ch_1",
		EventType:       "payment.chargeback.created",
		RentalID:        rentalID,
	})
	require.NoError(t, err)
	require.True(t, repo.blockedAccts["T1"], "EC-5 chargeback blocks tenant account")
	rec, found, err := repo.GetCancellationByRental(context.Background(), rentalID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, rental.WindowPlatformChargeback, rec.WindowCode)
}

func TestHandleWebhookEvent_Refunded_MirrorsCancellation(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	err := svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider:        "noop",
		ProviderEventID: "evt_refund_1",
		EventType:       "payment.refunded",
		RentalID:        rentalID,
	})
	require.NoError(t, err)
	_, found, err := repo.GetCancellationByRental(context.Background(), rentalID)
	require.NoError(t, err)
	require.True(t, found)
}

func TestHandleWebhookEvent_Idempotent_ReplaysAreNoop(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newF4Service(t, repo)
	rentalID := seedAuthorizedRental(t, repo, "T1", "O1")
	ev := rentsvc.ProviderWebhookEvent{
		Provider:        "noop",
		ProviderEventID: "evt_idem_1",
		EventType:       "payment.authorized",
		RentalID:        rentalID,
		AmountCents:     30000,
		Authorized:      true,
	}
	// First call marks authorized; second call is a replay and must
	// not double-write the rental state or the webhook row.
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), ev))
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), ev))
}

func TestRecordWebhookEvent_ReturnsIdempotencyConflictOnReplay(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	ev := rentsvc.WebhookEvent{
		Provider: "noop", ProviderEventID: "evt_x",
		EventType: "payment.authorized", SignatureValid: true,
		ReceivedAt: time.Now(),
	}
	_, err := repo.RecordWebhookEvent(context.Background(), ev)
	require.NoError(t, err)
	_, err = repo.RecordWebhookEvent(context.Background(), ev)
	require.ErrorIs(t, err, rental.ErrIdempotencyConflict)
}

// fakeListing/ListingLookup minimal stub.
// fakeAccounts/AccountLookup minimal stub.
// newFakeProvider returns a noop provider used by handler fakes.
// publishedListing returns a minimal valid listing for tests.
