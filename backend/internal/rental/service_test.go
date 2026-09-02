package rental_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

// --- fakes ---------------------------------------------------------------

type fakeRepo struct {
	mu        sync.Mutex
	rentals   map[string]rental.Rental
	intents   map[string]rentsvc.PaymentIntent
	webhooks  map[string]rentsvc.WebhookEvent
	receipts  map[string]rental.Receipt
	blocks    map[string][]rentsvc.Block
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		rentals:  map[string]rental.Rental{},
		intents:  map[string]rentsvc.PaymentIntent{},
		webhooks: map[string]rentsvc.WebhookEvent{},
		receipts: map[string]rental.Receipt{},
		blocks:   map[string][]rentsvc.Block{},
	}
}

func (f *fakeRepo) CreateIntent(_ context.Context, r rental.Rental, _ []byte) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if existing, ok := f.rentals[r.IntentKey]; ok {
		return existing, nil
	}
	f.rentals[r.IntentKey] = r
	f.rentals[r.ID] = r
	return r, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rentals {
		if r.ID == id {
			return r, nil
		}
	}
	return rental.Rental{}, rental.ErrNotFound
}

func (f *fakeRepo) GetByIntentKey(_ context.Context, tenantID, listingID, key string) (rental.Rental, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rentals {
		if r.TenantAccountID == tenantID && r.ListingID == listingID && r.IntentKey == key {
			return r, true, nil
		}
	}
	return rental.Rental{}, false, nil
}

func (f *fakeRepo) ListForOwner(_ context.Context, ownerID string, states []rental.State) ([]rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	var out []rental.Rental
	for _, r := range f.rentals {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		if ownerID == "" || r.ListingSnapshot.OwnerID == ownerID {
			if len(states) == 0 || containsState(states, r.State) {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) ListForTenant(_ context.Context, tenantID string, states []rental.State) ([]rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	var out []rental.Rental
	for _, r := range f.rentals {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		if r.TenantAccountID == tenantID {
			if len(states) == 0 || containsState(states, r.State) {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateState(_ context.Context, id string, from, to rental.State, mutate func(r *rental.Rental)) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	updated := false
	for k, r := range f.rentals {
		if r.ID == id {
			if !rental.CanTransition(from, to) {
				return rental.Rental{}, rental.ErrInvalidTransition
			}
			r.State = to
			if mutate != nil {
				mutate(&r)
			}
			r.UpdatedAt = time.Now().UTC()
			f.rentals[k] = r
			updated = true
		}
	}
	if !updated {
		return rental.Rental{}, rental.ErrNotFound
	}
	for k, r := range f.rentals {
		if r.ID == id && k != id {
			r.State = to
			if mutate != nil {
				mutate(&r)
			}
			r.UpdatedAt = time.Now().UTC()
			f.rentals[k] = r
		}
	}
	for _, r := range f.rentals {
		if r.ID == id {
			return r, nil
		}
	}
	return rental.Rental{}, rental.ErrNotFound
}

func (f *fakeRepo) ListActiveOverlapping(_ context.Context, listingID string, start, end time.Time) ([]rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []rental.Rental
	for _, r := range f.rentals {
		if r.ListingID != listingID || !r.State.OccupiesCalendar() {
			continue
		}
		if rental.HasOverlap(start, end, r.StartsAt, r.EndsAt) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListOwnerBlocks(_ context.Context, listingID string, start, end time.Time) ([]rentsvc.Block, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []rentsvc.Block
	for _, b := range f.blocks[listingID] {
		if rental.HasOverlap(start, end, b.StartsAt, b.EndsAt) {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeRepo) SaveReceipt(_ context.Context, rec rental.Receipt) (rental.Receipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.receipts[rec.RentalID]; exists {
		return rec, rental.ErrReceiptAlreadyExists
	}
	rec.IssuedAt = time.Now().UTC()
	f.receipts[rec.RentalID] = rec
	return rec, nil
}

func (f *fakeRepo) GetReceipt(_ context.Context, rentalID string) (rental.Receipt, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.receipts[rentalID]
	return rec, ok, nil
}

func (f *fakeRepo) UpsertPaymentIntent(_ context.Context, intent rentsvc.PaymentIntent) (rentsvc.PaymentIntent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if existing, ok := f.intents[intent.IdempotencyKey]; ok {
		return existing, nil
	}
	f.intents[intent.IdempotencyKey] = intent
	return intent, nil
}

func (f *fakeRepo) GetPaymentIntent(_ context.Context, rentalID string) (rentsvc.PaymentIntent, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, i := range f.intents {
		if i.RentalID == rentalID {
			return i, true, nil
		}
	}
	return rentsvc.PaymentIntent{}, false, nil
}

func (f *fakeRepo) RecordWebhookEvent(_ context.Context, ev rentsvc.WebhookEvent) (rentsvc.WebhookEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ev.Provider + "|" + ev.ProviderEventID
	if _, exists := f.webhooks[key]; exists {
		return f.webhooks[key], nil
	}
	f.webhooks[key] = ev
	return ev, nil
}

func containsState(states []rental.State, s rental.State) bool {
	for _, x := range states {
		if x == s {
			return true
		}
	}
	return false
}

type fakeListing struct{ listing listing.Listing }

func (f *fakeListing) GetByID(_ context.Context, id string) (listing.Listing, error) {
	if id != f.listing.ID {
		return listing.Listing{}, listing.ErrNotFound
	}
	return f.listing, nil
}

type fakeAccounts struct{ byID map[string]account.Account }

func (f *fakeAccounts) GetByID(_ context.Context, id string) (account.Account, error) {
	a, ok := f.byID[id]
	if !ok {
		return account.Account{}, account.ErrNotFound
	}
	return a, nil
}

type fakeProvider struct {
	mu          sync.Mutex
	intents     map[string]rentsvc.CreateIntentResponse
	createCalls int
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{intents: map[string]rentsvc.CreateIntentResponse{}}
}

func (f *fakeProvider) CreateIntent(_ context.Context, req rentsvc.CreateIntentRequest) (rentsvc.CreateIntentResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	if existing, ok := f.intents[req.IdempotencyKey]; ok {
		return existing, nil
	}
	resp := rentsvc.CreateIntentResponse{
		Provider:          "noop",
		ProviderPaymentID: "pi_" + req.IdempotencyKey,
		Status:            "requires_action",
	}
	f.intents[req.IdempotencyKey] = resp
	return resp, nil
}

func (f *fakeProvider) VerifyWebhookSignature(_ context.Context, _ []byte, _ string) (rentsvc.ProviderWebhookEvent, error) {
	return rentsvc.ProviderWebhookEvent{}, nil
}

func activeTenant(id string) account.Account {
	return account.Account{
		ID:          id,
		Status:      account.StatusActive,
		DisplayName: "Tenant " + id,
		Phone:       "+5511999999999",
	}
}

func publishedListing(id, ownerName string, mode string) listing.Listing {
	return listing.Listing{
		ID:                 id,
		OwnerAccountID:     ownerName,
		State:              listing.StatePublished,
		Title:              "Furadeira",
		Category:           listing.CategoryElectric,
		PickupCity:         "São Paulo",
		PickupNeighborhood: "Pinheiros",
		PriceUnit:          listing.PriceHour,
		PriceAmountCents:   5000,
		DepositCents:       20000,
		Operator: listing.Operator{
			Mode:            listing.OperatorMode(mode),
			HourlyRateCents: 3000,
			MinHours:        4,
			Identity:        listing.OperatorIdentity{Name: ownerName, IsOwner: mode == "optional" || mode == "required"},
		},
	}
}

func newService(t *testing.T, repo rentsvc.Repository, l Listing, a Account, p rentsvc.PaymentProvider, opts ...func(*rentsvc.Config)) *rentsvc.Service {
	t.Helper()
	cfg := rentsvc.Config{}
	for _, o := range opts {
		o(&cfg)
	}
	return rentsvc.NewService(repo, l, a, p, cfg)
}

type Listing = rentsvc.ListingLookup
type Account = rentsvc.AccountLookup

// --- tests ---------------------------------------------------------------

func TestCreateIntent_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, b, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1",
		StartsAt: start, EndsAt: end, WithOperator: false,
	})
	require.NoError(t, err)
	require.Equal(t, rental.StatePending, r.State)
	require.Equal(t, int64(10000), b.RentCents)
	require.Equal(t, int64(20000), b.DepositCents)
	require.Equal(t, int64(1200), b.CommissionCents, "12% of rent (operator=0)")
}

func TestCreateIntent_RejectsOverlap_EC1(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1"), "T2": activeTenant("T2")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r1, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r1.RentCents + r1.OperatorCents + r1.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_o1", EventType: "payment.authorized",
		RentalID: r1.ID, AmountCents: expected,
	}))
	_, _, err = svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T2", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.ErrorIs(t, err, rental.ErrCalendarOverlap)
}

func TestCreateIntent_AcceptsNonOverlappingWindow(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1"), "T2": activeTenant("T2")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	_, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	_, _, err = svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T2", ListingID: "L1", StartsAt: end, EndsAt: end.Add(2 * time.Hour),
	})
	require.NoError(t, err, "back-to-back windows must not be flagged as overlap (half-open)")
}

func TestCreateIntent_RejectsOperatorTermsRequired_AC5(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "required")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	_, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end, WithOperator: true,
	})
	require.ErrorIs(t, err, rental.ErrOperatorTermsRequired)
}

func TestCreateIntent_IdempotentOnRetry(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r1, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	r2, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	require.Equal(t, r1.ID, r2.ID)
}

func TestHandleWebhookEvent_AuthorizesAndPersistsReceipt_AC5_AC6(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
		c.AcceptanceWindow = 12 * time.Hour
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_1", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	loaded, err := repo.GetByID(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, rental.StateAuthorized, loaded.State)
	require.NotNil(t, loaded.AcceptanceDeadlineAt)
	rec, found, err := repo.GetReceipt(context.Background(), r.ID)
	require.NoError(t, err)
	require.True(t, found, "receipt must be persisted on authorization")
	require.Equal(t, int64(20000), rec.DepositCents, "deposit in receipt (informational)")
	require.Equal(t, int64(10000), rec.CommissionBaseCents, "deposit OUT (AC-5/AC-7)")
}

func TestHandleWebhookEvent_EC4_RejectsAmountMismatch(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	err = svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_2", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: 1,
	})
	require.ErrorIs(t, err, rental.ErrPaymentTotalMismatch)
	loaded, err := repo.GetByID(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, rental.StatePending, loaded.State)
}

func TestHandleWebhookEvent_EC2_DuplicateEvent_NoOp(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	ev := rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_dup", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), ev))
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), ev))
	loaded, err := repo.GetByID(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, rental.StateAuthorized, loaded.State)
}

func TestAccept_Within12hWindow_AC6(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
		c.AcceptanceWindow = 12 * time.Hour
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_a", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	accepted, err := svc.Accept(context.Background(), rentsvc.AcceptInput{OwnerID: "Ana", RentalID: r.ID})
	require.NoError(t, err)
	require.Equal(t, rental.StateConfirmed, accepted.State)
	require.NotNil(t, accepted.ConfirmedAt)
}

func TestAccept_AfterDeadline_EC5(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
		c.AcceptanceWindow = 12 * time.Hour
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_b", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	svc2 := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now.Add(13 * time.Hour) }
		c.AcceptanceWindow = 12 * time.Hour
	})
	_, err = svc2.Accept(context.Background(), rentsvc.AcceptInput{OwnerID: "Ana", RentalID: r.ID})
	require.ErrorIs(t, err, rental.ErrAcceptanceExpired)
}

func TestDecline_FromAuthorized(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_d", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	declined, err := svc.Decline(context.Background(), rentsvc.DeclineInput{
		OwnerID: "Ana", RentalID: r.ID, DeclineReason: "Fora do prazo",
	})
	require.NoError(t, err)
	require.Equal(t, rental.StateDeclined, declined.State)
	require.Equal(t, "Fora do prazo", declined.DeclineReason)
}

func TestExpireSweep_FlagsOverdue(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
		c.AcceptanceWindow = 12 * time.Hour
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_e", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	moved, err := svc.ExpireSweep(context.Background(), now.Add(13*time.Hour), 10)
	require.NoError(t, err)
	require.Equal(t, 1, moved)
	loaded, err := repo.GetByID(context.Background(), r.ID)
	require.NoError(t, err)
	require.Equal(t, rental.StateExpired, loaded.State)
}

func TestAuthorizeIntent_CreatesPSPIntentAndIsIdempotent_EC2(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	intent1, err := svc.AuthorizeIntent(context.Background(), rentsvc.AuthorizeIntentInput{
		TenantID: "T1", RentalID: r.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "noop", intent1.Provider)
	intent2, err := svc.AuthorizeIntent(context.Background(), rentsvc.AuthorizeIntentInput{
		TenantID: "T1", RentalID: r.ID,
	})
	require.NoError(t, err)
	require.Equal(t, intent1.ID, intent2.ID)
	require.Equal(t, 1, fp.createCalls, "provider.CreateIntent should be called once")
}

func TestExpireSweep_IdempotentOnAlreadyTerminal(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	fp := newFakeProvider()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, repo, fl, fa, fp, func(c *rentsvc.Config) {
		c.Now = func() time.Time { return now }
		c.AcceptanceWindow = 12 * time.Hour
	})
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)
	expected := r.RentCents + r.OperatorCents + r.DepositCents
	require.NoError(t, svc.HandleWebhookEvent(context.Background(), rentsvc.ProviderWebhookEvent{
		Provider: "noop", ProviderEventID: "evt_i", EventType: "payment.authorized",
		RentalID: r.ID, AmountCents: expected,
	}))
	moved1, err := svc.ExpireSweep(context.Background(), now.Add(13*time.Hour), 10)
	require.NoError(t, err)
	require.Equal(t, 1, moved1)
	moved2, err := svc.ExpireSweep(context.Background(), now.Add(14*time.Hour), 10)
	require.NoError(t, err)
	require.Equal(t, 0, moved2)
}
