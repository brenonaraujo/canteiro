package rental_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

func TestConfig_Defaults_ZeroValues(t *testing.T) {
	t.Parallel()
	cfg := rentsvc.Config{}
	cfg.Defaults()
	require.Equal(t, 12*time.Hour, cfg.AcceptanceWindow)
	require.Equal(t, rental.DefaultCommissionBPS, cfg.CommissionBPS)
	require.Equal(t, "BRL", cfg.DefaultCurrency)
	require.Equal(t, "noop", cfg.ProviderName)
	require.NotNil(t, cfg.Now)
	require.NotNil(t, cfg.IDGen)
	id := cfg.IDGen.String()
	require.Len(t, id, 36)
}

func TestConfig_Defaults_PreservesExplicitValues(t *testing.T) {
	t.Parallel()
	cfg := rentsvc.Config{
		AcceptanceWindow: 6 * time.Hour,
		CommissionBPS:    500,
		DefaultCurrency:  "USD",
		ProviderName:     "stripe",
	}
	cfg.Defaults()
	require.Equal(t, 6*time.Hour, cfg.AcceptanceWindow)
	require.Equal(t, int64(500), cfg.CommissionBPS)
	require.Equal(t, "USD", cfg.DefaultCurrency)
	require.Equal(t, "stripe", cfg.ProviderName)
}

func TestRequireActiveTenant_RejectsDeactivated(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": {
		ID: "T1", Status: account.StatusDeactivated,
		DisplayName: "T", Phone: "+551****9999",
	}}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	_, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1",
		StartsAt: time.Now().Add(2 * time.Hour), EndsAt: time.Now().Add(4 * time.Hour),
	})
	require.ErrorIs(t, err, rental.ErrAccountDeactivated)
}

func TestRequireActiveTenant_RejectsProfileIncomplete(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": {
		ID: "T1", Status: account.StatusActive,
	}}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	_, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1",
		StartsAt: time.Now().Add(2 * time.Hour), EndsAt: time.Now().Add(4 * time.Hour),
	})
	require.ErrorIs(t, err, rental.ErrProfileIncomplete)
}

func TestRequireActiveTenant_AccountLookupErrorBubbles(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	_, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "missing", ListingID: "L1",
		StartsAt: time.Now().Add(2 * time.Hour), EndsAt: time.Now().Add(4 * time.Hour),
	})
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestRequirePublishedListingSnapshot_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	_, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "missing",
		StartsAt: time.Now().Add(2 * time.Hour), EndsAt: time.Now().Add(4 * time.Hour),
	})
	require.ErrorIs(t, err, rental.ErrNotFound)
}

func TestRequirePublishedListingSnapshot_NotPublished(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	paused := publishedListing("L1", "Ana", "none")
	paused.State = listing.StatePaused
	fl := &fakeListing{listing: paused}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	_, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1",
		StartsAt: time.Now().Add(2 * time.Hour), EndsAt: time.Now().Add(4 * time.Hour),
	})
	require.ErrorIs(t, err, rental.ErrListingUnavailable)
}

func TestRequireNoOverlap_DetectsOwnerBlock(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp})
	start := time.Now().Add(2 * time.Hour).UTC()
	end := start.Add(2 * time.Hour)
	repo.blocks["L1"] = []rentsvc.Block{{ListingID: "L1", StartsAt: start, EndsAt: end}}
	_, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.ErrorIs(t, err, rental.ErrCalendarOverlap)
}

func TestListForTenant_Empty(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	got, err := svc.ListForTenant(context.Background(), "nobody")
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestListForOwner_Empty(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	got, err := svc.ListForOwner(context.Background(), "nobody")
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestGet_DelegatesToRepo(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	end := start.Add(time.Hour)
	created, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	loaded, err := svc.Get(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, loaded.ID)
}

func TestGet_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	_, err := svc.Get(context.Background(), "missing")
	require.ErrorIs(t, err, rental.ErrNotFound)
}

func TestGetReceipt_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp}, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_rec", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	rec, err := svc.GetReceipt(context.Background(), r.ID, "T1")
	require.NoError(t, err)
	require.Equal(t, r.ID, rec.RentalID)
	require.Equal(t, "T1", rec.TenantAccountID)
}

func TestGetReceipt_WrongTenantForbidden(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{
		"T1": activeTenant("T1"),
		"T2": activeTenant("T2"),
	}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = svc.GetReceipt(context.Background(), r.ID, "T2")
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestGetReceipt_MissingReceipt(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = svc.GetReceipt(context.Background(), r.ID, "T1")
	require.ErrorIs(t, err, rental.ErrNotFound)
}

func TestCancelPreAuth_FromPending(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	cancelled, err := svc.CancelPreAuth(context.Background(), rentsvc.CancelPreAuthInput{
		TenantID: "T1", RentalID: r.ID,
	})
	require.NoError(t, err)
	require.Equal(t, rental.StateCancelled, cancelled.State)
}

func TestCancelPreAuth_FromAuthorized(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	// pending → cancelled is the only valid cancel pre-auth path;
	// the service rejects cancel from authorized because EC-3 expects
	// the refund flow (F4) to own that transition.
	_, err = svc.CancelPreAuth(context.Background(), rentsvc.CancelPreAuthInput{
		TenantID: "T1", RentalID: r.ID,
	})
	require.NoError(t, err)
	loaded, err := svc.Get(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, rental.StateCancelled, loaded.State)
}

func TestCancelPreAuth_WrongTenantForbidden(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = svc.CancelPreAuth(context.Background(), rentsvc.CancelPreAuthInput{
		TenantID: "T2", RentalID: r.ID,
	})
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestCancelPreAuth_AfterConfirmationInvalid(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp}, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
		c.AcceptanceWindow = 12 * time.Hour
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_x", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	_, err = svc.Accept(context.Background(), rentsvc.AcceptInput{OwnerID: "Ana", RentalID: r.ID})
	require.NoError(t, err)
	_, err = svc.CancelPreAuth(context.Background(), rentsvc.CancelPreAuthInput{
		TenantID: "T1", RentalID: r.ID,
	})
	require.ErrorIs(t, err, rental.ErrInvalidTransition)
}

func TestDecline_WrongOwnerForbidden(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_d1", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	_, err = svc.Decline(context.Background(), rentsvc.DeclineInput{
		OwnerID: "Bob", RentalID: r.ID, DeclineReason: "x",
	})
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestDecline_NotAuthorizedInvalid(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = svc.Decline(context.Background(), rentsvc.DeclineInput{
		OwnerID: "Ana", RentalID: r.ID, DeclineReason: "x",
	})
	require.ErrorIs(t, err, rental.ErrInvalidTransition)
}

func TestAccept_NotAuthorizedInvalid(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = svc.Accept(context.Background(), rentsvc.AcceptInput{OwnerID: "Ana", RentalID: r.ID})
	require.ErrorIs(t, err, rental.ErrInvalidTransition)
}

func TestAccept_NoDeadlineInvalid(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = repo.UpdateState(context.Background(), r.ID, rental.StatePending, rental.StateAuthorized, nil)
	require.NoError(t, err)
	_, err = svc.Accept(context.Background(), rentsvc.AcceptInput{OwnerID: "Ana", RentalID: r.ID})
	require.ErrorIs(t, err, rental.ErrAcceptanceExpired)
}

func TestHandleWebhookEvent_PaymentFailed(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_fail", EventType: "payment.failed",
		RentalID: r.ID, FailureCode: "card_declined", FailureMessage: "Limite.",
	}))
	loaded, err := svc.Get(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, rental.StatePending, loaded.State)
}

func TestHandleWebhookEvent_PaymentRefunded(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp}, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
		c.AcceptanceWindow = 12 * time.Hour
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_authz", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	_, err = svc.Accept(context.Background(), rentsvc.AcceptInput{OwnerID: "Ana", RentalID: r.ID})
	require.NoError(t, err)
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_refund", EventType: "payment.refunded",
		RentalID: r.ID,
	}))
	loaded, err := svc.Get(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, rental.StateRefunded, loaded.State)
}

func TestHandleWebhookEvent_UnknownEventTypeIsNoOp(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_xyz", EventType: "payment.exotic",
		RentalID: r.ID,
	}))
	loaded, err := svc.Get(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, rental.StatePending, loaded.State)
}

func TestHandleWebhookEvent_PaymentAuthorizedFromAlreadyTerminalIsNoOp(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp}, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_a", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_b", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
}

func TestAuthorizeIntent_RejectsForeignTenant(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = svc.AuthorizeIntent(context.Background(), rentsvc.AuthorizeIntentInput{
		TenantID: "intruder", RentalID: r.ID,
	})
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestAuthorizeIntent_RejectsNonPending(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()})
	start := time.Now().Add(2 * time.Hour).UTC()
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = repo.UpdateState(context.Background(), r.ID, rental.StatePending, rental.StateCancelled, nil)
	require.NoError(t, err)
	_, err = svc.AuthorizeIntent(context.Background(), rentsvc.AuthorizeIntentInput{
		TenantID: "T1", RentalID: r.ID,
	})
	require.ErrorIs(t, err, rental.ErrInvalidTransition)
}

func TestExpireSweep_RespectsBatchLimit(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: fp}, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
		c.AcceptanceWindow = 12 * time.Hour
	})
	for i := 0; i < 3; i++ {
		start := now.Add(time.Duration(10+i*5) * time.Hour)
		end := start.Add(time.Hour)
		r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
			TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
		})
		require.NoError(t, err)
		expected := r.RentCents + r.OperatorCents + r.DepositCents
		require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
			Provider: "noop", ProviderEventID: "evt_b" + intToStr(i),
			EventType: "payment.authorized", RentalID: r.ID, AmountCents: expected,
		}))
	}
	moved, err := svc.ExpireSweep(context.Background(), now.Add(48*time.Hour), 2)
	require.NoError(t, err)
	require.Equal(t, 2, moved, "batch limit respected")
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
