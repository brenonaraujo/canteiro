package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
	"github.com/brenonaraujo/canteiro/backend/internal/handler"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// --- fakes -----------------------------------------------------------------

type fakeRepo struct {
	byID    map[string]listing.Listing
	blocks  map[string][]listing.Block
	onboard map[string]listing.OwnerOnboarding
	cats    []listing.CategoryConfig
	mu      sync.Mutex
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byID:    map[string]listing.Listing{},
		blocks:  map[string][]listing.Block{},
		onboard: map[string]listing.OwnerOnboarding{},
		cats: []listing.CategoryConfig{
			{Category: listing.CategoryManual, Size: listing.SizeLight, DepositMinCents: 5000},
			{Category: listing.CategoryElectric, Size: listing.SizeLight, DepositMinCents: 8000},
			{Category: listing.CategoryLightConstruction, Size: listing.SizeLight, DepositMinCents: 15000},
			{Category: listing.CategoryAgricultural, Size: listing.SizeLight, DepositMinCents: 20000},
			{Category: listing.CategoryHeavy, Size: listing.SizeHeavy, DepositMinCents: 80000},
		},
	}
}

func (r *fakeRepo) Create(_ context.Context, l listing.Listing) (listing.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if l.ID == "" {
		return listing.Listing{}, listing.ErrInvalidInput
	}
	l.CreatedAt = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l.UpdatedAt = l.CreatedAt
	r.byID[l.ID] = l
	return l, nil
}
func (r *fakeRepo) Update(_ context.Context, l listing.Listing) (listing.Listing, error) {
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
func (r *fakeRepo) GetByID(_ context.Context, id string) (listing.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[id]
	if !ok {
		return listing.Listing{}, listing.ErrNotFound
	}
	return l, nil
}
func (r *fakeRepo) ListByOwner(_ context.Context, ownerID string) ([]listing.Listing, error) {
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
func (r *fakeRepo) GetPublic(_ context.Context, id string) (listing.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[id]
	if !ok || l.State != listing.StatePublished {
		return listing.Listing{}, listing.ErrNotFound
	}
	return l, nil
}
func (r *fakeRepo) UpdateState(_ context.Context, id string, s listing.State) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[id]
	if !ok {
		return listing.ErrNotFound
	}
	l.State = s
	r.byID[id] = l
	return nil
}
func (r *fakeRepo) ReplacePhotos(_ context.Context, id string, p []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.byID[id]
	if !ok {
		return listing.ErrNotFound
	}
	l.Photos = append([]string{}, p...)
	r.byID[id] = l
	return nil
}
func (r *fakeRepo) AddBlock(_ context.Context, b listing.Block) (listing.Block, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b.CreatedAt = time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	r.blocks[b.ListingID] = append(r.blocks[b.ListingID], b)
	return b, nil
}
func (r *fakeRepo) ListBlocks(_ context.Context, id string) ([]listing.Block, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]listing.Block{}, r.blocks[id]...), nil
}
func (r *fakeRepo) ListBlocksInWindow(_ context.Context, id string, from, to time.Time) ([]listing.Block, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []listing.Block
	for _, b := range r.blocks[id] {
		if to.Equal(b.StartsAt) || from.Equal(b.EndsAt) {
			continue
		}
		if from.Before(b.EndsAt) && b.StartsAt.Before(to) {
			out = append(out, b)
		}
	}
	return out, nil
}
func (r *fakeRepo) RemoveBlock(_ context.Context, id, blockID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, b := range r.blocks[id] {
		if b.ID == blockID {
			r.blocks[id] = append(r.blocks[id][:i], r.blocks[id][i+1:]...)
			return nil
		}
	}
	return listing.ErrNotFound
}
func (r *fakeRepo) GetOwnerOnboarding(_ context.Context, id string) (listing.OwnerOnboarding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.onboard[id]
	if !ok {
		return listing.OwnerOnboarding{AccountID: id}, nil
	}
	return o, nil
}
func (r *fakeRepo) UpsertOwnerOnboarding(_ context.Context, o listing.OwnerOnboarding) (listing.OwnerOnboarding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onboard[o.AccountID] = o
	return o, nil
}
func (r *fakeRepo) CategoryConfig(_ context.Context) ([]listing.CategoryConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]listing.CategoryConfig{}, r.cats...), nil
}
func (r *fakeRepo) CategoryByName(_ context.Context, c listing.Category) (listing.CategoryConfig, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.cats {
		if x.Category == c {
			return x, true, nil
		}
	}
	return listing.CategoryConfig{}, false, nil
}
func (r *fakeRepo) SearchCatalog(_ context.Context, _ listing.SearchFilters) ([]listing.Listing, int, error) {
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

type fakeAccountLookup struct {
	m  map[string]account.Account
	mu sync.Mutex
}

func newAccountLookup() *fakeAccountLookup {
	return &fakeAccountLookup{m: map[string]account.Account{}}
}

func (f *fakeAccountLookup) GetByID(_ context.Context, id string) (account.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.m[id]
	if !ok {
		return account.Account{}, account.ErrNotFound
	}
	return a, nil
}

// --- test rig --------------------------------------------------------------

func newRouter(t *testing.T, sessionID string, lookup *fakeAccountLookup) *gin.Engine {
	t.Helper()
	_, _ = i18n.Load()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	repo := newFakeRepo()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc := listing.NewService(repo, lookup, now)
	svc.SetIDFunc(fixedID("11111111-1111-4111-8111-111111111111"))
	listingAPI := handler.NewListingAPI(svc, func(c *gin.Context) (string, bool) {
		if sessionID == "" {
			return "", false
		}
		return sessionID, true
	})
	// Mount via the openapi codegen so we exercise the registered routes.
	apiSrv := listingServer{api: listingAPI}
	api.RegisterHandlers(r, apiSrv)
	return r
}

// listingServer embeds the listing api alongside no-op auth/ops so the
// ServerInterface dispatch still works for the F2 routes only.
type listingServer struct {
	api *handler.ListingAPI
}

// Override every auth/ops handler with a 404 to keep the test focused.
func (listingServer) GetAccount(*gin.Context)        { panic("unused") }
func (listingServer) UpdateAccount(*gin.Context)     { panic("unused") }
func (listingServer) DeactivateAccount(*gin.Context) { panic("unused") }
func (listingServer) StartGoogleAuth(*gin.Context)   { panic("unused") }
func (listingServer) GoogleCallback(*gin.Context)    { panic("unused") }
func (listingServer) Logout(*gin.Context)            { panic("unused") }
func (listingServer) Healthz(*gin.Context)           { panic("unused") }
func (listingServer) Readyz(*gin.Context)            { panic("unused") }

// Listing methods — implemented.
func (s listingServer) ListMineListings(c *gin.Context)   { s.api.ListMineListings(c) }
func (s listingServer) CreateListingDraft(c *gin.Context) { s.api.CreateListingDraft(c) }
func (s listingServer) GetMyListing(c *gin.Context, id openapiUUID) {
	s.api.GetMyListing(c, id)
}
func (s listingServer) UpdateListing(c *gin.Context, id openapiUUID) { s.api.UpdateListing(c, id) }
func (s listingServer) PublishListing(c *gin.Context, id openapiUUID) {
	s.api.PublishListing(c, id)
}
func (s listingServer) PauseListing(c *gin.Context, id openapiUUID) { s.api.PauseListing(c, id) }
func (s listingServer) ListBlocks(c *gin.Context, id openapiUUID)   { s.api.ListBlocks(c, id) }
func (s listingServer) AddBlock(c *gin.Context, id openapiUUID)     { s.api.AddBlock(c, id) }
func (s listingServer) RemoveBlock(c *gin.Context, id, b openapiUUID) {
	s.api.RemoveBlock(c, id, b)
}
func (s listingServer) GetOwnerOnboarding(c *gin.Context)    { s.api.GetOwnerOnboarding(c) }
func (s listingServer) UpdateOwnerOnboarding(c *gin.Context) { s.api.UpdateOwnerOnboarding(c) }
func (s listingServer) ListCategories(c *gin.Context)        { s.api.ListCategories(c) }

// F3 stubs (out of scope for listing tests).
func (listingServer) CreateRental(*gin.Context)                             { panic("unused") }
func (listingServer) ListMyRentals(*gin.Context)                            { panic("unused") }
func (listingServer) GetRental(*gin.Context, openapiUUID)                   { panic("unused") }
func (listingServer) AuthorizeRentalPayment(*gin.Context, openapiUUID)      { panic("unused") }
func (listingServer) AcceptRental(*gin.Context, openapiUUID)                { panic("unused") }
func (listingServer) DeclineRental(*gin.Context, openapiUUID)               { panic("unused") }
func (listingServer) CancelRental(*gin.Context, openapiUUID)                { panic("unused") }
func (listingServer) CreateRentalCancellation(*gin.Context, openapiUUID)    { panic("unused") }
func (listingServer) GetRentalCancellation(*gin.Context, openapiUUID)       { panic("unused") }
func (listingServer) GetRentalReceipt(*gin.Context, openapiUUID)            { panic("unused") }
func (listingServer) PaymentWebhook(*gin.Context, api.PaymentWebhookParams) { panic("unused") }
func (s listingServer) SearchCatalog(c *gin.Context, p api.SearchCatalogParams) {
	s.api.SearchCatalog(c, p)
}
func (s listingServer) GetPublicListing(c *gin.Context, id openapiUUID) {
	s.api.GetPublicListing(c, id)
}
func (s listingServer) GetPublicCalendar(c *gin.Context, id openapiUUID, p api.GetPublicCalendarParams) {
	s.api.GetPublicCalendar(c, id, p)
}

// openapiUUID aliases the codegen UUID type so this test file does not need
// a direct runtime/types import.
type openapiUUID = openapiUUIDT

// forward declaration filled in below
type openapiUUIDT = openapiUUIDReal

// --- tests -----------------------------------------------------------------

func TestCreateListing_RequiresSession(t *testing.T) {
	r := newRouter(t, "", newAccountLookup())
	w := httptest.NewRecorder()
	body := `{"title":"abc","description":"abcdef","category":"manual","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":1000,"deposit_cents":5000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	req, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateListing_HappyPath(t *testing.T) {
	lookup := newAccountLookup()
	acc := account.Account{
		ID: "owner-1", Status: account.StatusActive,
		DisplayName: "Ana", Phone: "+5511999999999",
	}
	lookup.m["owner-1"] = acc
	r := newRouter(t, "owner-1", lookup)
	w := httptest.NewRecorder()
	body := `{"title":"Furadeira Bosch GSB","description":"Furadeira de impacto 600W com maleta e brocas.","category":"electric","pickup_city":"Sao Paulo","pickup_neighborhood":"Vila Mariana","delivery":{"enabled":false,"coverage":""},"price_unit":"day","price_amount_cents":12000,"deposit_cents":8000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://cdn.example.com/x.jpg"],"rules":{"document_required":true,"min_age":21}}`
	req, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	var got api.Listing
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, listing.StateDraft, listing.State(got.State))
	require.Equal(t, "Furadeira Bosch GSB", got.Title)
}

func TestCreateListing_InvalidPayload(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/listings", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPublish_ForbiddenMapping(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive, DisplayName: "Owner", Phone: "+5511999999999"}
	r := newRouter(t, "owner-1", lookup)
	w := httptest.NewRecorder()
	// Create a draft via the repo, then publish with no onboarding → 422 owner_onboarding_required.
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W com maleta.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":12000,"deposit_cents":8000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	creq, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	creq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, creq)
	require.Equal(t, http.StatusCreated, w.Code)

	w2 := httptest.NewRecorder()
	preq, _ := http.NewRequest("POST", "/listings/11111111-1111-4111-8111-111111111111/publish", nil)
	r.ServeHTTP(w2, preq)
	require.Equal(t, http.StatusUnprocessableEntity, w2.Code)
	var apiErr api.Error
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &apiErr))
	require.Equal(t, "owner_onboarding_required", apiErr.Code)
}

func TestListCategories_Public(t *testing.T) {
	r := newRouter(t, "", newAccountLookup())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/catalog/categories", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var got []api.CategoryConfig
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.GreaterOrEqual(t, len(got), 5)
}

func TestSearchCatalog_PublicNoLogin(t *testing.T) {
	r := newRouter(t, "", newAccountLookup())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/catalog/listings?category=electric", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var page api.ListingPage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.GreaterOrEqual(t, page.PageSize, 1)
}

func TestGetPublicListing_DraftIs404(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	// Create a draft.
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W com maleta.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":12000,"deposit_cents":8000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Public endpoint should 404 for drafts (privacy).
	w2 := httptest.NewRecorder()
	greq, _ := http.NewRequest("GET", "/catalog/listings/11111111-1111-4111-8111-111111111111", nil)
	r.ServeHTTP(w2, greq)
	require.Equal(t, http.StatusNotFound, w2.Code)
}

func TestGetOwnerOnboarding_DefaultsEmpty(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/owner/onboarding", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var got api.OwnerOnboarding
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.False(t, got.PayoutSet)
	require.False(t, got.TermsAccepted)
}

func TestAddBlock_HappyPath(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	// create draft
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W com maleta.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":12000,"deposit_cents":8000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// add block
	start := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC)
	blockBody, _ := json.Marshal(api.AddBlockRequest{StartsAt: start, EndsAt: end})
	w2 := httptest.NewRecorder()
	breq, _ := http.NewRequest("POST", "/listings/11111111-1111-4111-8111-111111111111/blocks", bytes.NewReader(blockBody))
	breq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, breq)
	require.Equal(t, http.StatusCreated, w2.Code)
	var block api.AvailabilityBlock
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &block))
	require.NotEmpty(t, block.Id)
}

func TestListMineAndGetMyListing_HappyPath(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W com maleta.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":12000,"deposit_cents":8000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	w2 := httptest.NewRecorder()
	lreq, _ := http.NewRequest("GET", "/listings", nil)
	r.ServeHTTP(w2, lreq)
	require.Equal(t, http.StatusOK, w2.Code)
	var list []api.Listing
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &list))
	require.NotEmpty(t, list)

	w3 := httptest.NewRecorder()
	greq, _ := http.NewRequest("GET", "/listings/11111111-1111-4111-8111-111111111111", nil)
	r.ServeHTTP(w3, greq)
	require.Equal(t, http.StatusOK, w3.Code)
}

func TestUpdateAndPauseListing(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive, DisplayName: "Owner", Phone: "+551****9999"}
	r := newRouter(t, "owner-1", lookup)
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W com maleta.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":12000,"deposit_cents":8000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	updBody := `{"title":"Furadeira v2","price_amount_cents":15000}`
	w2 := httptest.NewRecorder()
	ureq, _ := http.NewRequest("PATCH", "/listings/11111111-1111-4111-8111-111111111111", strings.NewReader(updBody))
	ureq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, ureq)
	require.Equal(t, http.StatusOK, w2.Code)

	// Pause requires a published listing (Service.CanPause gate), so the
	// draft returns 409 — that still exercises PauseListing for coverage.
	w3 := httptest.NewRecorder()
	preq, _ := http.NewRequest("POST", "/listings/11111111-1111-4111-8111-111111111111/pause", nil)
	r.ServeHTTP(w3, preq)
	require.Equal(t, http.StatusConflict, w3.Code)
}

func TestListAndRemoveBlock(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W com maleta.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":12000,"deposit_cents":8000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	start := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC)
	blockBody, _ := json.Marshal(api.AddBlockRequest{StartsAt: start, EndsAt: end})
	w2 := httptest.NewRecorder()
	breq, _ := http.NewRequest("POST", "/listings/11111111-1111-4111-8111-111111111111/blocks", bytes.NewReader(blockBody))
	breq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, breq)
	require.Equal(t, http.StatusCreated, w2.Code)
	var block api.AvailabilityBlock
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &block))

	w3 := httptest.NewRecorder()
	lreq, _ := http.NewRequest("GET", "/listings/11111111-1111-4111-8111-111111111111/blocks", nil)
	r.ServeHTTP(w3, lreq)
	require.Equal(t, http.StatusOK, w3.Code)

	w4 := httptest.NewRecorder()
	rreq, _ := http.NewRequest("DELETE", "/listings/11111111-1111-4111-8111-111111111111/blocks/"+block.Id.String(), nil)
	r.ServeHTTP(w4, rreq)
	require.Equal(t, http.StatusNoContent, w4.Code)
}

func TestUpdateOwnerOnboarding(t *testing.T) {
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	w3 := httptest.NewRecorder()
	ureq, _ := http.NewRequest("PATCH", "/owner/onboarding", strings.NewReader(`{"payout_kind":"pix","payout_last4":"1234","accept_terms":true,"terms_version":"v1"}`))
	ureq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, ureq)
	require.Equal(t, http.StatusOK, w3.Code)
}

func TestPatchOperatorViaUpdate(t *testing.T) {
	// Exercises patchOperator (the unexported helper in listing.go).
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	create := httptest.NewRecorder()
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":12000,"deposit_cents":8000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	creq, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	creq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(create, creq)
	require.Equal(t, http.StatusCreated, create.Code)
	w := httptest.NewRecorder()
	patchBody := `{"operator":{"mode":"required","hourly_rate_cents":5000,"min_hours":4,"identity":{"name":"Carlos","phone":"+551****7777","is_owner":false}}}`
	ureq, _ := http.NewRequest("PATCH", "/listings/11111111-1111-4111-8111-111111111111", strings.NewReader(patchBody))
	ureq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, ureq)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestNoSession_ReturnsUnauthorized(t *testing.T) {
	// Calling handler.NewListingAPI with nil current falls back to noSession.
	gin.SetMode(gin.TestMode)
	lookup := newAccountLookup()
	repo := newFakeRepo()
	api6 := handler.NewListingAPI(listing.NewService(repo, lookup, time.Now().UTC()), nil)
	r := gin.New()
	api.RegisterHandlers(r, listingServer{api: api6})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/listings", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSearchCatalog_WithFilters(t *testing.T) {
	// Exercises parseSearchFilters branches for category + city + size.
	svc := listing.NewService(newFakeRepo(), newAccountLookup(), time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	cat := listing.CategoryElectric
	_, err := svc.CreateDraft(context.Background(), "owner-1", listing.Listing{
		Title: "Furadeira", Description: "Furadeira de impacto 600W.",
		Category: cat, PickupCity: "Sao Paulo", PickupNeighborhood: "x",
		PriceUnit: listing.PriceDay, PriceAmountCents: 12000, DepositCents: 8000,
		MinLeadTimeHours: 12, Photos: []string{"https://x/a.jpg"},
	})
	require.NoError(t, err)
	api8 := handler.NewListingAPI(svc, func(c *gin.Context) (string, bool) { return "owner-1", true })
	r := gin.New()
	api.RegisterHandlers(r, listingServer{api: api8})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/catalog/listings?category=electric&city=Sao+Paulo&size=light", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestPauseListing_HappyPath(t *testing.T) {
	// Exercises the Pause listing branch (was at 0% before).
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive, DisplayName: "Ana", Phone: "+551****9999"}
	repo := newFakeRepo()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc := listing.NewService(repo, lookup, now)
	l := listing.Listing{
		ID:                 "22222222-2222-4222-8222-222222222222",
		OwnerAccountID:     "owner-1",
		State:              listing.StateDraft,
		Title:              "Furadeira",
		Description:        "Furadeira de impacto 600W.",
		Category:           listing.CategoryElectric,
		PickupCity:         "SP",
		PickupNeighborhood: "x",
		PriceUnit:          listing.PriceDay,
		PriceAmountCents:   12000,
		DepositCents:       8000,
		MinLeadTimeHours:   12,
		Operator:           listing.Operator{Mode: listing.OperatorNone},
		Photos:             []string{"https://x/a.jpg"},
	}
	_, err := repo.Create(context.Background(), l)
	require.NoError(t, err)
	_, err = repo.UpsertOwnerOnboarding(context.Background(), listing.OwnerOnboarding{
		AccountID: "owner-1", PayoutKind: "pix", PayoutLast4: "1234",
		TermsAcceptedAt: now, TermsVersion: "v1",
	})
	require.NoError(t, err)
	api9 := handler.NewListingAPI(svc, func(c *gin.Context) (string, bool) { return "owner-1", true })
	r2 := gin.New()
	api.RegisterHandlers(r2, listingServer{api: api9})
	pub := httptest.NewRecorder()
	preq, _ := http.NewRequest("POST", "/listings/22222222-2222-4222-8222-222222222222/publish", nil)
	r2.ServeHTTP(pub, preq)
	require.Equal(t, http.StatusOK, pub.Code, "publish body: %s", pub.Body.String())
	pause := httptest.NewRecorder()
	pareq, _ := http.NewRequest("POST", "/listings/22222222-2222-4222-8222-222222222222/pause", nil)
	r2.ServeHTTP(pause, pareq)
	require.Equal(t, http.StatusOK, pause.Code)
}

func TestUpdateLocationPatchAndRulesPatch(t *testing.T) {
	// Exercises applyLocationPatch + applyPricingPatch via PATCH.
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	create := httptest.NewRecorder()
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":1000,"deposit_cents":5000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	creq, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	creq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(create, creq)
	require.Equal(t, http.StatusCreated, create.Code)
	w := httptest.NewRecorder()
	patchBody := `{"pickup_city":"Sao Paulo","pickup_neighborhood":"Pinheiros","price_amount_cents":12345,"deposit_cents":6789,"rules":{"document_required":true,"min_age":25,"experience_required":true,"travel_restricted":true}}`
	ureq, _ := http.NewRequest("PATCH", "/listings/11111111-1111-4111-8111-111111111111", strings.NewReader(patchBody))
	ureq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, ureq)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestGetPublicCalendar_EmptyWindow(t *testing.T) {
	// Exercises the GetPublicCalendar endpoint with default window.
	lookup := newAccountLookup()
	repo := newFakeRepo()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc := listing.NewService(repo, lookup, now)
	_, err := svc.CreateDraft(context.Background(), "owner-1", listing.Listing{
		Title: "Furadeira", Description: "Furadeira de impacto 600W.",
		Category: listing.CategoryElectric, PickupCity: "SP", PickupNeighborhood: "x",
		PriceUnit: listing.PriceDay, PriceAmountCents: 1000, DepositCents: 5000,
		MinLeadTimeHours: 12, Photos: []string{"https://x/a.jpg"},
	})
	require.NoError(t, err)
	apiA := handler.NewListingAPI(svc, nil)
	r := gin.New()
	api.RegisterHandlers(r, listingServer{api: apiA})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/catalog/listings/11111111-1111-4111-8111-111111111111/calendar", nil)
	r.ServeHTTP(w, req)
	// Draft returns 404 on public calendar (per F2 spec).
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestListBlocks_Owner(t *testing.T) {
	// Exercises the ListBlocks endpoint on an existing listing.
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	// Seed a draft via POST.
	create := httptest.NewRecorder()
	body := `{"title":"Furadeira","description":"Furadeira de impacto 600W.","category":"electric","pickup_city":"SP","pickup_neighborhood":"x","price_unit":"day","price_amount_cents":1000,"deposit_cents":5000,"min_lead_time_hours":12,"operator":{"mode":"none"},"photos":["https://x/a.jpg"]}`
	creq, _ := http.NewRequest("POST", "/listings", strings.NewReader(body))
	creq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(create, creq)
	require.Equal(t, http.StatusCreated, create.Code)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/listings/11111111-1111-4111-8111-111111111111/blocks", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestListMineListings_Empty(t *testing.T) {
	// Exercises the empty-tenant branch of ListMineListings.
	lookup := newAccountLookup()
	r := newRouter(t, "owner-1", lookup)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/listings", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestOwnerOnboarding_TermsNotAccepted(t *testing.T) {
	// Exercises the GetOwnerOnboarding path when terms are not yet accepted.
	lookup := newAccountLookup()
	lookup.m["owner-1"] = account.Account{ID: "owner-1", Status: account.StatusActive}
	r := newRouter(t, "owner-1", lookup)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/owner/onboarding", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestSearchCatalog_InvalidCategory(t *testing.T) {
	// Exercises parseSearchFilters with a malformed category (returns empty page).
	svc := listing.NewService(newFakeRepo(), newAccountLookup(), time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	apiB := handler.NewListingAPI(svc, nil)
	r := gin.New()
	api.RegisterHandlers(r, listingServer{api: apiB})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/catalog/listings?category=garbage&size=heavy", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

// --- helpers ---------------------------------------------------------------

type openapiUUIDReal = openapi_types.UUID

// --- fixed id generator ---------------------------------------------------

type fixedID string

func (f fixedID) String() string { return string(f) }

// Ensure we don't accidentally lose the fakeRepo contract.
var _ listing.Repository = (*fakeRepo)(nil)
