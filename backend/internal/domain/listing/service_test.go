package listing_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
)

// --- in-memory fakes -------------------------------------------------------

type memRepo struct {
	byID     map[string]listing.Listing
	blocks   map[string][]listing.Block
	onboard  map[string]listing.OwnerOnboarding
	cats     map[listing.Category]listing.CategoryConfig
	catOrder []listing.CategoryConfig
	mu       sync.Mutex
}

func newMemRepo() *memRepo {
	r := &memRepo{
		byID:    map[string]listing.Listing{},
		blocks:  map[string][]listing.Block{},
		onboard: map[string]listing.OwnerOnboarding{},
		cats:    map[listing.Category]listing.CategoryConfig{},
	}
	for _, c := range []listing.CategoryConfig{
		{Category: listing.CategoryManual, Size: listing.SizeLight, DepositMinCents: 5000},
		{Category: listing.CategoryElectric, Size: listing.SizeLight, DepositMinCents: 8000},
		{Category: listing.CategoryLightConstruction, Size: listing.SizeLight, DepositMinCents: 15000},
		{Category: listing.CategoryAgricultural, Size: listing.SizeLight, DepositMinCents: 20000},
		{Category: listing.CategoryHeavy, Size: listing.SizeHeavy, DepositMinCents: 80000},
	} {
		r.cats[c.Category] = c
		r.catOrder = append(r.catOrder, c)
	}
	return r
}

func (r *memRepo) Create(_ context.Context, l listing.Listing) (listing.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[l.ID]; ok {
		return listing.Listing{}, listing.ErrInvalidInput
	}
	if l.ID == "" {
		return listing.Listing{}, listing.ErrInvalidInput
	}
	l.CreatedAt = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l.UpdatedAt = l.CreatedAt
	r.byID[l.ID] = l
	return l, nil
}

func (r *memRepo) Update(_ context.Context, l listing.Listing) (listing.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.byID[l.ID]
	if !ok {
		return listing.Listing{}, listing.ErrNotFound
	}
	l.CreatedAt = cur.CreatedAt
	l.UpdatedAt = time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	r.byID[l.ID] = l
	return l, nil
}

func (r *memRepo) GetByID(_ context.Context, id string) (listing.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[id]
	if !ok {
		return listing.Listing{}, listing.ErrNotFound
	}
	return l, nil
}

func (r *memRepo) ListByOwner(_ context.Context, ownerID string) ([]listing.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []listing.Listing
	for _, l := range r.byID {
		if l.OwnerAccountID == ownerID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (r *memRepo) GetPublic(_ context.Context, id string) (listing.Listing, error) {
	l, err := r.GetByID(context.Background(), id)
	if err != nil {
		return listing.Listing{}, err
	}
	if l.State != listing.StatePublished {
		return listing.Listing{}, listing.ErrNotFound
	}
	return l, nil
}

func (r *memRepo) UpdateState(_ context.Context, id string, s listing.State) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[id]
	if !ok {
		return listing.ErrNotFound
	}
	l.State = s
	l.UpdatedAt = time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	r.byID[id] = l
	return nil
}

func (r *memRepo) ReplacePhotos(_ context.Context, listingID string, photos []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[listingID]
	if !ok {
		return listing.ErrNotFound
	}
	l.Photos = append([]string{}, photos...)
	l.UpdatedAt = time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	r.byID[listingID] = l
	return nil
}

func (r *memRepo) AddBlock(_ context.Context, b listing.Block) (listing.Block, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b.ID == "" || b.ListingID == "" {
		return listing.Block{}, listing.ErrInvalidInput
	}
	b.CreatedAt = time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	r.blocks[b.ListingID] = append(r.blocks[b.ListingID], b)
	return b, nil
}

func (r *memRepo) ListBlocks(_ context.Context, listingID string) ([]listing.Block, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]listing.Block{}, r.blocks[listingID]...), nil
}

func (r *memRepo) ListBlocksInWindow(_ context.Context, listingID string, from, to time.Time) ([]listing.Block, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []listing.Block
	for _, b := range r.blocks[listingID] {
		if to.Equal(b.StartsAt) || from.Equal(b.EndsAt) {
			continue
		}
		if from.Before(b.EndsAt) && b.StartsAt.Before(to) {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *memRepo) RemoveBlock(_ context.Context, listingID, blockID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	bs := r.blocks[listingID]
	for i, b := range bs {
		if b.ID == blockID {
			r.blocks[listingID] = append(bs[:i], bs[i+1:]...)
			return nil
		}
	}
	return listing.ErrNotFound
}

func (r *memRepo) GetOwnerOnboarding(_ context.Context, accountID string) (listing.OwnerOnboarding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.onboard[accountID]
	if !ok {
		return listing.OwnerOnboarding{AccountID: accountID}, nil
	}
	return o, nil
}

func (r *memRepo) UpsertOwnerOnboarding(_ context.Context, o listing.OwnerOnboarding) (listing.OwnerOnboarding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onboard[o.AccountID] = o
	return o, nil
}

func (r *memRepo) CategoryConfig(_ context.Context) ([]listing.CategoryConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]listing.CategoryConfig{}, r.catOrder...), nil
}

func (r *memRepo) CategoryByName(_ context.Context, c listing.Category) (listing.CategoryConfig, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg, ok := r.cats[c]
	return cfg, ok, nil
}

func (r *memRepo) SearchCatalog(_ context.Context, _ listing.SearchFilters) ([]listing.Listing, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []listing.Listing
	for _, l := range r.byID {
		if l.State == listing.StatePublished {
			out = append(out, l)
		}
	}
	return out, len(out), nil
}

// --- account fakes --------------------------------------------------------

type fakeAccountLookup struct {
	byID map[string]account.Account
	mu   sync.Mutex
}

func newAccountLookup() *fakeAccountLookup {
	return &fakeAccountLookup{byID: map[string]account.Account{}}
}

func (f *fakeAccountLookup) Put(a account.Account) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[a.ID] = a
}

func (f *fakeAccountLookup) GetByID(_ context.Context, id string) (account.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.byID[id]
	if !ok {
		return account.Account{}, account.ErrNotFound
	}
	return a, nil
}

// --- helpers ---------------------------------------------------------------

func newSvc(repo listing.Repository, acc accountLookup) *listing.Service {
	svc := listing.NewService(repo, acc, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	svc.SetIDFunc(staticID("11111111-1111-4111-8111-111111111111"))
	return svc
}

type staticID string

func (s staticID) String() string { return string(s) }

type accountLookup interface {
	GetByID(ctx context.Context, id string) (account.Account, error)
}

func activeAccount(id string) account.Account {
	return account.Account{
		ID:            id,
		GoogleSubject: "g-" + id,
		DisplayName:   "Ana",
		Phone:         "+5511999999999",
		Status:        account.StatusActive,
	}
}

func makeListingDraft(ownerID string) listing.Listing {
	return listing.Listing{
		ID:                 "11111111-1111-4111-8111-111111111111",
		OwnerAccountID:     ownerID,
		State:              listing.StateDraft,
		Title:              "Furadeira de impacto Bosch",
		Description:        "Furadeira de impacto 600W com maleta e brocas.",
		Category:           listing.CategoryElectric,
		PickupCity:         "São Paulo",
		PickupNeighborhood: "Vila Mariana",
		Delivery:           listing.Delivery{Enabled: false, Coverage: ""},
		PriceUnit:          listing.PriceDay,
		PriceAmountCents:   12000,
		DepositCents:       8000,
		MinLeadTimeHours:   12,
		Photos:             []string{"https://cdn.example.com/listing/1/photo-1.jpg"},
		Rules:              listing.Rules{DocumentRequired: true, MinAge: 21},
		Operator: listing.Operator{
			Mode:            listing.OperatorOptional,
			HourlyRateCents: 5000,
			MinHours:        4,
		},
		HeavyLegalCession: false,
	}
}

// --- CreateDraft -----------------------------------------------------------

func TestCreateDraft_Success(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)

	got, err := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	require.NoError(t, err)
	require.Equal(t, listing.StateDraft, got.State)
	require.Equal(t, "owner-1", got.OwnerAccountID)
	stored, err := repo.GetByID(context.Background(), got.ID)
	require.NoError(t, err)
	require.Equal(t, listing.StateDraft, stored.State)
}

func TestCreateDraft_InvalidInput(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)

	bad := makeListingDraft("owner-1")
	bad.Title = "abc" // below minTitle (4) — wait, 3 chars
	_, err := svc.CreateDraft(context.Background(), "owner-1", bad)
	require.ErrorIs(t, err, listing.ErrInvalidInput)
}

func TestCreateDraft_NormalisesOwner(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)

	draft := makeListingDraft("owner-1")
	draft.OwnerAccountID = "attacker"
	got, err := svc.CreateDraft(context.Background(), "owner-1", draft)
	require.NoError(t, err)
	require.Equal(t, "owner-1", got.OwnerAccountID, "service must overwrite caller-supplied owner")
}

// --- Update ---------------------------------------------------------------

func TestUpdate_Draft_Success(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	patch := got
	patch.Title = "Furadeira Bosch GSB 13 RE"
	updated, err := svc.Update(context.Background(), "owner-1", patch)
	require.NoError(t, err)
	require.Equal(t, "Furadeira Bosch GSB 13 RE", updated.Title)
}

func TestUpdate_Published_Rejected(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)

	_, err = svc.Update(context.Background(), "owner-1", got)
	require.ErrorIs(t, err, listing.ErrAlreadyPublished)
}

func TestUpdate_NotOwner_Forbidden(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.Update(context.Background(), "owner-2", got)
	require.ErrorIs(t, err, listing.ErrForbidden)
}

func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	bogus := makeListingDraft("owner-1")
	bogus.ID = "99999999-9999-4999-8999-999999999999"
	_, err := svc.Update(context.Background(), "owner-1", bogus)
	require.ErrorIs(t, err, listing.ErrNotFound)
}

// --- Publish ---------------------------------------------------------------

func TestPublish_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	pub, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)
	require.Equal(t, listing.StatePublished, pub.State)
}

func TestPublish_DeactivatedAccount(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	a := activeAccount("owner-1")
	a.Status = account.StatusDeactivated
	acc.Put(a)
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrDeactivated)
}

func TestPublish_ProfileIncomplete(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(account.Account{ID: "owner-1", Status: account.StatusActive})
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrProfileIncomplete)
}

func TestPublish_MissingOnboarding(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrOwnerOnboardingRequired)
}

func TestPublish_StaleTerms(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v0",
	}
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrOwnerOnboardingRequired)
}

func TestPublish_MissingPhoto_EC1(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	draft := makeListingDraft("owner-1")
	draft.Photos = nil
	got, _ := svc.CreateDraft(context.Background(), "owner-1", draft)

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrPublishGates)
}

func TestPublish_DepositBelowMin_EC2(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	draft := makeListingDraft("owner-1")
	draft.DepositCents = 1000 // electric minimum is 8000
	got, _ := svc.CreateDraft(context.Background(), "owner-1", draft)

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrPublishGates)
}

func TestPublish_HeavyNoCession_EC3(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	draft := makeListingDraft("owner-1")
	draft.Category = listing.CategoryHeavy
	draft.DepositCents = 80000
	draft.HeavyLegalCession = false
	draft.Operator.Mode = listing.OperatorRequired
	draft.Operator.Identity.Name = "João"
	draft.Operator.Identity.Phone = "+5511988887777"
	draft.Operator.Identity.IsOwner = false
	got, _ := svc.CreateDraft(context.Background(), "owner-1", draft)

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrPublishGates)
}

func TestPublish_HeavyRequiredOperatorMissing_EC4(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	draft := makeListingDraft("owner-1")
	draft.Category = listing.CategoryHeavy
	draft.DepositCents = 80000
	draft.HeavyLegalCession = true
	draft.Operator.Mode = listing.OperatorRequired
	draft.Operator.Identity.Name = ""
	got, _ := svc.CreateDraft(context.Background(), "owner-1", draft)

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrPublishGates)
}

func TestPublish_DeliveryNoCoverage_EC8(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	draft := makeListingDraft("owner-1")
	draft.Delivery.Enabled = true
	draft.Delivery.Coverage = ""

	_, err := svc.CreateDraft(context.Background(), "owner-1", draft)
	require.ErrorIs(t, err, listing.ErrInvalidInput,
		"delivery enabled without coverage must be rejected at draft time (EC-8)")
}

func TestPublish_NotFound(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	svc := newSvc(repo, acc)

	_, err := svc.Publish(context.Background(), "owner-1", "99999999-9999-4999-8999-999999999999")
	require.ErrorIs(t, err, listing.ErrNotFound)
}

func TestPublish_NotOwner(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	acc.Put(activeAccount("owner-2"))
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.Publish(context.Background(), "owner-2", got.ID)
	require.ErrorIs(t, err, listing.ErrForbidden)
}

// --- Pause -----------------------------------------------------------------

func TestPause_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)

	paused, err := svc.Pause(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)
	require.Equal(t, listing.StatePaused, paused.State)
}

func TestPause_NotPublished(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.Pause(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrNotPublished)
}

// --- GetMine / ListMine ----------------------------------------------------

func TestGetMine_OwnerOnly(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.GetMine(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)
	_, err = svc.GetMine(context.Background(), "owner-2", got.ID)
	require.ErrorIs(t, err, listing.ErrForbidden)
}

// --- Public ----------------------------------------------------------------

func TestGetPublic_OnlyPublished(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.GetPublic(context.Background(), got.ID)
	require.ErrorIs(t, err, listing.ErrNotFound)
}

func TestSearchCatalog_PublicNoLogin(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	items, total, err := svc.SearchCatalog(context.Background(), listing.SearchFilters{PageSize: 10})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, 0, total)
}

// --- Owner onboarding ------------------------------------------------------

func TestGetOwnerOnboarding_DefaultsEmpty(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)

	o, err := svc.GetOwnerOnboarding(context.Background(), "owner-1")
	require.NoError(t, err)
	require.False(t, o.PayoutSet())
	require.False(t, o.TermsAccepted("v1"))
}

func TestUpsertOwnerOnboarding_AcceptTerms(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)

	now := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	o, err := svc.UpsertOwnerOnboarding(context.Background(), "owner-1", listing.OwnerOnboarding{
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: now,
		TermsVersion:    "v1",
	})
	require.NoError(t, err)
	require.True(t, o.PayoutSet())
	require.True(t, o.TermsAccepted("v1"))
	require.False(t, o.TermsAccepted("v2"))
}

// --- Blocks ----------------------------------------------------------------

func TestAddBlock_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	start := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	b, err := svc.AddBlock(context.Background(), "owner-1", got.ID, listing.Block{
		ListingID: got.ID,
		StartsAt:  start,
		EndsAt:    end,
		Reason:    "manutenção",
	})
	require.NoError(t, err)
	require.NotEmpty(t, b.ID)
}

func TestAddBlock_WindowError(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	start := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	_, err := svc.AddBlock(context.Background(), "owner-1", got.ID, listing.Block{
		ListingID: got.ID,
		StartsAt:  start,
		EndsAt:    start, // zero-length window
	})
	require.ErrorIs(t, err, listing.ErrBlockWindow)
}

func TestAddBlock_OverlapRejected(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	start := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)
	_, err := svc.AddBlock(context.Background(), "owner-1", got.ID, listing.Block{
		ListingID: got.ID, StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)

	_, err = svc.AddBlock(context.Background(), "owner-1", got.ID, listing.Block{
		ListingID: got.ID,
		StartsAt:  start.Add(2 * time.Hour),
		EndsAt:    end.Add(2 * time.Hour),
	})
	require.ErrorIs(t, err, listing.ErrBlockOverlap)
}

func TestAddBlock_NotOwner(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	start := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	_, err := svc.AddBlock(context.Background(), "owner-2", got.ID, listing.Block{
		ListingID: got.ID, StartsAt: start, EndsAt: end,
	})
	require.ErrorIs(t, err, listing.ErrForbidden)
}

func TestListBlocks_OwnerOnly(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	bs, err := svc.ListBlocks(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)
	require.Empty(t, bs)
	_, err = svc.ListBlocks(context.Background(), "owner-2", got.ID)
	require.ErrorIs(t, err, listing.ErrForbidden)
}

func TestRemoveBlock_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	start := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	b, err := svc.AddBlock(context.Background(), "owner-1", got.ID, listing.Block{
		ListingID: got.ID, StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)

	require.NoError(t, svc.RemoveBlock(context.Background(), "owner-1", got.ID, b.ID))
	require.ErrorIs(t, svc.RemoveBlock(context.Background(), "owner-1", got.ID, b.ID), listing.ErrNotFound)
}

// --- sanity ---------------------------------------------------------------

var _ listing.Repository = (*memRepo)(nil)
var _ account.Account = account.Account{}

// --- coverage boost: list/get public paths ---------------------------------

func TestListMine_ReturnsOwnerOnly(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	_, err := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	require.NoError(t, err)

	svc2 := listing.NewService(repo, acc, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	svc2.SetIDFunc(staticID("22222222-2222-4222-8222-222222222222"))
	_, err = svc2.CreateDraft(context.Background(), "owner-2", makeListingDraft("owner-2"))
	require.NoError(t, err)

	mine, err := svc.ListMine(context.Background(), "owner-1")
	require.NoError(t, err)
	require.Len(t, mine, 1)
	require.Equal(t, "owner-1", mine[0].OwnerAccountID)
}

func TestGetPublicCalendar_AllBlocks(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)

	start := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC)
	_, err = svc.AddBlock(context.Background(), "owner-1", got.ID, listing.Block{
		ListingID: got.ID, StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)

	bs, err := svc.GetPublicCalendar(context.Background(), got.ID, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, bs, 1)

	bs, err = svc.GetPublicCalendar(context.Background(), got.ID,
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, bs, 1)

	bs, err = svc.GetPublicCalendar(context.Background(), got.ID,
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Empty(t, bs)
}

func TestGetPublicCalendar_DraftNotFound(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	_, err := svc.GetPublicCalendar(context.Background(), got.ID, time.Time{}, time.Time{})
	require.ErrorIs(t, err, listing.ErrNotFound)
}

func TestListCategories_ReturnsAll(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	cats, err := svc.ListCategories(context.Background())
	require.NoError(t, err)
	require.Len(t, cats, 5)
}

// --- coverage: UpsertOwnerOnboarding branches ------------------------------

func TestUpsertOwnerOnboarding_RejectsStaleTerms(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	_, err := svc.UpsertOwnerOnboarding(context.Background(), "owner-1", listing.OwnerOnboarding{
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC),
		TermsVersion:    "v0",
	})
	require.ErrorIs(t, err, listing.ErrOwnerOnboardingRequired)
}

func TestUpsertOwnerOnboarding_PreservesTermsIfNotReset(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	now := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	_, err := svc.UpsertOwnerOnboarding(context.Background(), "owner-1", listing.OwnerOnboarding{
		PayoutKind: "pix", PayoutLast4: "1234",
		TermsAcceptedAt: now, TermsVersion: "v1",
	})
	require.NoError(t, err)

	o, err := svc.UpsertOwnerOnboarding(context.Background(), "owner-1", listing.OwnerOnboarding{
		PayoutKind: "bank", PayoutLast4: "5678",
	})
	require.NoError(t, err)
	require.True(t, o.PayoutSet())
	require.Equal(t, "bank", o.PayoutKind, "payout kind must update")
	require.Equal(t, "5678", o.PayoutLast4, "payout last4 must update")
	require.True(t, o.TermsAccepted("v1"), "previous terms must be preserved")
}

// --- coverage: more service branches --------------------------------------

func TestDefaultIDGen_FormatIsUUID(t *testing.T) {
	t.Parallel()
	g := listing.DefaultIDGenerator()
	id := g.String()
	require.Len(t, id, 36)
	require.Equal(t, '-', rune(id[8]))
	require.Equal(t, '-', rune(id[13]))
	require.Equal(t, '-', rune(id[18]))
	require.Equal(t, '-', rune(id[23]))
}

func TestPublish_FromPaused(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)
	_, err = svc.Pause(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)

	pub, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)
	require.Equal(t, listing.StatePublished, pub.State)
}

func TestPublish_AccountLookupError(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))

	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.ErrorIs(t, err, listing.ErrForbidden,
		"unknown owner must surface as forbidden (no leak)")
}

func TestPause_NotFound(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	_, err := svc.Pause(context.Background(), "owner-1", "99999999-9999-4999-8999-999999999999")
	require.ErrorIs(t, err, listing.ErrNotFound)
}

func TestPause_NotOwner(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	acc.Put(activeAccount("owner-1"))
	repo.onboard["owner-1"] = listing.OwnerOnboarding{
		AccountID:       "owner-1",
		PayoutKind:      "pix",
		PayoutLast4:     "1234",
		TermsAcceptedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermsVersion:    "v1",
	}
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	_, err := svc.Publish(context.Background(), "owner-1", got.ID)
	require.NoError(t, err)
	_, err = svc.Pause(context.Background(), "owner-2", got.ID)
	require.ErrorIs(t, err, listing.ErrForbidden)
}

func TestRemoveBlock_NotOwner(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	acc := newAccountLookup()
	svc := newSvc(repo, acc)
	got, _ := svc.CreateDraft(context.Background(), "owner-1", makeListingDraft("owner-1"))
	err := svc.RemoveBlock(context.Background(), "owner-2", got.ID, "x")
	require.ErrorIs(t, err, listing.ErrForbidden)
}

// --- coverage: model validation & enums -----------------------------------

func TestCategoryValid(t *testing.T) {
	t.Parallel()
	for _, c := range []listing.Category{
		listing.CategoryManual, listing.CategoryElectric,
		listing.CategoryLightConstruction, listing.CategoryAgricultural,
		listing.CategoryHeavy,
	} {
		require.True(t, c.Valid(), "%s should be valid", c)
	}
	require.False(t, listing.Category("bogus").Valid())
}

func TestOperatorModeValid(t *testing.T) {
	t.Parallel()
	for _, m := range []listing.OperatorMode{
		listing.OperatorNone, listing.OperatorOptional, listing.OperatorRequired,
	} {
		require.True(t, m.Valid())
	}
	require.False(t, listing.OperatorMode("").Valid())
	require.False(t, listing.OperatorMode("forced").Valid())
}

func TestPriceUnitValid(t *testing.T) {
	t.Parallel()
	require.True(t, listing.PriceHour.Valid())
	require.True(t, listing.PriceDay.Valid())
	require.False(t, listing.PriceUnit("week").Valid())
}

func TestCategorySize(t *testing.T) {
	t.Parallel()
	require.Equal(t, listing.SizeHeavy, listing.CategoryHeavy.Size())
	require.Equal(t, listing.SizeLight, listing.CategoryManual.Size())
	require.Equal(t, listing.SizeLight, listing.CategoryElectric.Size())
}

func TestValidate_Bounds(t *testing.T) {
	t.Parallel()
	base := makeListingDraft("owner-1")
	cases := []struct {
		mut  func(*listing.Listing)
		name string
	}{
		{name: "empty title", mut: func(l *listing.Listing) { l.Title = "" }},
		{name: "title too long", mut: func(l *listing.Listing) { l.Title = strings.Repeat("x", 121) }},
		{name: "empty desc", mut: func(l *listing.Listing) { l.Description = "" }},
		{name: "desc too long", mut: func(l *listing.Listing) { l.Description = strings.Repeat("x", 4001) }},
		{name: "unknown category", mut: func(l *listing.Listing) { l.Category = listing.Category("nope") }},
		{name: "empty city", mut: func(l *listing.Listing) { l.PickupCity = "" }},
		{name: "neighborhood too long", mut: func(l *listing.Listing) { l.PickupNeighborhood = strings.Repeat("x", 81) }},
		{name: "hourly price unit bad", mut: func(l *listing.Listing) { l.PriceUnit = listing.PriceUnit("week") }},
		{name: "price zero", mut: func(l *listing.Listing) { l.PriceAmountCents = 0 }},
		{name: "price too high", mut: func(l *listing.Listing) { l.PriceAmountCents = 200_000_000 }},
		{name: "negative deposit", mut: func(l *listing.Listing) { l.DepositCents = -1 }},
		{name: "negative lead", mut: func(l *listing.Listing) { l.MinLeadTimeHours = -1 }},
		{name: "op hourly too high", mut: func(l *listing.Listing) { l.Operator.HourlyRateCents = 200_000_000 }},
		{name: "op min hours negative", mut: func(l *listing.Listing) { l.Operator.MinHours = -1 }},
		{name: "op name too long", mut: func(l *listing.Listing) { l.Operator.Identity.Name = strings.Repeat("x", 81) }},
		{name: "op phone too long", mut: func(l *listing.Listing) { l.Operator.Identity.Phone = strings.Repeat("1", 33) }},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			l := base
			c.mut(&l)
			require.ErrorIs(t, l.Validate(), listing.ErrInvalidInput)
		})
	}
}

func TestHasOverlappingBlock_AdjacentOK(t *testing.T) {
	t.Parallel()
	a := listing.Block{StartsAt: time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 2, 1, 14, 0, 0, 0, time.UTC)}
	b := listing.Block{StartsAt: time.Date(2026, 2, 1, 14, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 2, 1, 18, 0, 0, 0, time.UTC)}
	require.False(t, listing.HasOverlappingBlock([]listing.Block{a}, b.StartsAt, b.EndsAt))
}

func TestPublishMissingHelper(t *testing.T) {
	t.Parallel()
	p := listing.PublishMissing{"a", "b"}
	require.Equal(t, []string{"a", "b"}, p.Missing())
}

// _ to silence unused imports if any subtree gets pruned.
var _ = strings.Repeat
