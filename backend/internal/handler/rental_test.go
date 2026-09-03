package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	"github.com/brenonaraujo/canteiro/backend/internal/handler"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

// --- fakes --------------------------------------------------------------

type rentSvcFake struct {
	rentals         map[string]rental.Rental
	intents         map[string]rentsvc.PaymentIntent
	receipts        map[string]rental.Receipt
	cancellations   map[string]rentsvc.CancellationRecord
	createErr       error
	mu              sync.Mutex
	createCalls     int
	listTenantCalls int
	acceptCalls     int
	declineCalls    int
	cancelCalls     int
	authCalls       int
	getReceiptCalls int
}

func newRentSvcFake() *rentSvcFake {
	return &rentSvcFake{
		rentals:       map[string]rental.Rental{},
		intents:       map[string]rentsvc.PaymentIntent{},
		receipts:      map[string]rental.Receipt{},
		cancellations: map[string]rentsvc.CancellationRecord{},
	}
}

func (f *rentSvcFake) CreateIntent(_ context.Context, in rentsvc.CreateIntentInput) (rental.Rental, rental.MoneyBreakdown, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	if f.createErr != nil {
		return rental.Rental{}, rental.MoneyBreakdown{}, f.createErr
	}
	id := uuid.NewString()
	start := in.StartsAt
	end := in.EndsAt
	r := rental.Rental{
		ID:                    id,
		ListingID:             in.ListingID,
		TenantAccountID:       in.TenantID,
		StartsAt:              start,
		EndsAt:                end,
		State:                 rental.StatePending,
		WithOperator:          in.WithOperator,
		OperatorTermsAccepted: in.OperatorTermsAccepted,
		ListingSnapshot: rental.ListingSnapshot{
			OwnerID:          "owner-x",
			Title:            "Furadeira",
			Category:         "electric",
			PriceUnit:        "hour",
			PriceAmountCents: 5000,
			DepositCents:     20000,
			MinLeadTimeHours: 12,
			PickupCity:       "São Paulo",
			Operator:         rental.OperatorSnapshot{Mode: "none"},
		},
		RentCents:           10000,
		OperatorCents:       0,
		DepositCents:        20000,
		CommissionCents:     1200,
		OwnerPayoutCents:    8800,
		OperatorPayoutCents: 0,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	f.rentals[r.ID] = r
	b := rental.MoneyBreakdown{
		RentCents:           10000,
		OperatorCents:       0,
		DepositCents:        20000,
		TotalCents:          30000,
		CommissionBaseCents: 10000,
		CommissionCents:     1200,
		OwnerPayoutCents:    8800,
		OperatorPayoutCents: 0,
	}
	return r, b, nil
}

func (f *rentSvcFake) ListForTenant(_ context.Context, tenantID string) ([]rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listTenantCalls++
	var out []rental.Rental
	for _, r := range f.rentals {
		if r.TenantAccountID == tenantID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *rentSvcFake) Get(_ context.Context, id string) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rentals[id]
	if !ok {
		return rental.Rental{}, rental.ErrNotFound
	}
	return r, nil
}

func (f *rentSvcFake) Accept(_ context.Context, in rentsvc.AcceptInput) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acceptCalls++
	r, ok := f.rentals[in.RentalID]
	if !ok {
		return rental.Rental{}, rental.ErrNotFound
	}
	r.State = rental.StateConfirmed
	now := time.Now().UTC()
	r.ConfirmedAt = &now
	f.rentals[in.RentalID] = r
	return r, nil
}

func (f *rentSvcFake) Decline(_ context.Context, in rentsvc.DeclineInput) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.declineCalls++
	r, ok := f.rentals[in.RentalID]
	if !ok {
		return rental.Rental{}, rental.ErrNotFound
	}
	r.State = rental.StateDeclined
	r.DeclineReason = in.DeclineReason
	now := time.Now().UTC()
	r.DeclinedAt = &now
	f.rentals[in.RentalID] = r
	return r, nil
}

func (f *rentSvcFake) CancelPreAuth(_ context.Context, in rentsvc.CancelPreAuthInput) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelCalls++
	r, ok := f.rentals[in.RentalID]
	if !ok {
		return rental.Rental{}, rental.ErrNotFound
	}
	r.State = rental.StateCancelled
	f.rentals[in.RentalID] = r
	return r, nil
}

func (f *rentSvcFake) Cancel(_ context.Context, in rentsvc.CancelInput) (rentsvc.CancellationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelCalls++
	r, ok := f.rentals[in.RentalID]
	if !ok {
		return rentsvc.CancellationResult{}, rental.ErrNotFound
	}
	r.State = rental.StateCancelled
	f.rentals[in.RentalID] = r
	rec := rentsvc.CancellationRecord{
		ID:                  "canc-" + in.RentalID,
		RentalID:            in.RentalID,
		ActorID:             in.CallerAccountID,
		ActorKind:           in.ActorKind,
		WindowCode:          rental.WindowOwnerPrePickup,
		TenantRefundCents:   r.RentCents + r.OperatorCents,
		DepositState:        rental.DepositReleased,
		DepositReleaseCents: r.DepositCents,
		IssuedAt:            time.Now().UTC(),
	}
	return rentsvc.CancellationResult{Cancellation: rec, Rental: r}, nil
}

func (f *rentSvcFake) GetCancellation(_ context.Context, rentalID, callerAccountID string) (rentsvc.CancellationRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rentals[rentalID]
	if !ok {
		return rentsvc.CancellationRecord{}, rental.ErrNotFound
	}
	if r.TenantAccountID != callerAccountID && r.ListingSnapshot.OwnerID != callerAccountID {
		return rentsvc.CancellationRecord{}, rental.ErrForbidden
	}
	if c, ok := f.cancellations[rentalID]; ok {
		return c, nil
	}
	return rentsvc.CancellationRecord{
		ID:                  "canc-" + rentalID,
		RentalID:            rentalID,
		ActorID:             r.TenantAccountID,
		ActorKind:           rental.ActorTenant,
		WindowCode:          rental.WindowOwnerPrePickup,
		TenantRefundCents:   r.RentCents + r.OperatorCents,
		DepositState:        rental.DepositReleased,
		DepositReleaseCents: r.DepositCents,
		IssuedAt:            time.Now().UTC(),
	}, nil
}

// seedCancellation places a known cancellation record in the fake's map.
func (f *rentSvcFake) seedCancellation(rentalID string) rentsvc.CancellationRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := rentsvc.CancellationRecord{
		ID:                   "canc-" + rentalID,
		RentalID:             rentalID,
		ActorID:              "tenant-1",
		ActorKind:            rental.ActorTenant,
		WindowCode:           rental.WindowTenantGe24h,
		CancellationFeeCents: 1000,
		TenantRefundCents:    9000,
		CommissionCents:      1200,
		DepositState:         rental.DepositReleased,
		DepositReleaseCents:  20000,
		IssuedAt:             time.Now().UTC(),
	}
	f.cancellations[rentalID] = c
	return c
}

func (f *rentSvcFake) AuthorizeIntent(_ context.Context, in rentsvc.AuthorizeIntentInput) (rentsvc.PaymentIntent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.authCalls++
	intent := rentsvc.PaymentIntent{
		ID:                 "pi_" + in.RentalID,
		RentalID:           in.RentalID,
		Provider:           "noop",
		IdempotencyKey:     "rental-" + in.RentalID + "-attempt-1",
		Status:             "requires_action",
		Attempt:            1,
		AmountCents:        30000,
		DepositCents:       20000,
		ExpectedTotalCents: 30000,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	f.intents[in.RentalID] = intent
	return intent, nil
}

func (f *rentSvcFake) GetReceipt(_ context.Context, rentalID, tenantID string) (rental.Receipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getReceiptCalls++
	rec, ok := f.receipts[rentalID]
	if !ok {
		return rental.Receipt{}, rental.ErrNotFound
	}
	if rec.TenantAccountID != tenantID {
		return rental.Receipt{}, rental.ErrForbidden
	}
	return rec, nil
}

func (f *rentSvcFake) seedReceipt(rentalID, tenantID string) rental.Receipt {
	rec := rental.Receipt{
		RentalID:            rentalID,
		TenantAccountID:     tenantID,
		RentCents:           10000,
		OperatorCents:       0,
		DepositCents:        20000,
		TotalCents:          30000,
		CommissionBaseCents: 10000,
		CommissionCents:     1200,
		OwnerPayoutCents:    8800,
		OperatorPayoutCents: 0,
	}
	f.receipts[rentalID] = rec
	return rec
}

// seedRental places a known rental in the fake's map.
func (f *rentSvcFake) seedRental(rentalID, tenantID string) rental.Rental {
	r := rental.Rental{
		ID:                  rentalID,
		ListingID:           "11111111-1111-4111-8111-111111111111",
		TenantAccountID:     tenantID,
		State:               rental.StateAuthorized,
		StartsAt:            time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC),
		EndsAt:              time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC),
		RentCents:           10000,
		OperatorCents:       0,
		DepositCents:        20000,
		CommissionCents:     1200,
		OwnerPayoutCents:    8800,
		OperatorPayoutCents: 0,
		ListingSnapshot: rental.ListingSnapshot{
			OwnerID:          "owner-1",
			Title:            "Furadeira",
			Category:         "electric",
			PriceUnit:        "hour",
			PriceAmountCents: 5000,
			DepositCents:     20000,
			MinLeadTimeHours: 12,
			Operator:         rental.OperatorSnapshot{Mode: "none"},
		},
	}
	f.rentals[r.ID] = r
	return r
}

func (f *rentSvcFake) HandleWebhookEvent(_ context.Context, _ rentsvc.ProviderWebhookEvent) error {
	return nil
}

// --- harness ------------------------------------------------------------

func newRentalRouter(t *testing.T, sessionID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := newRentSvcFake()
	rentalAPI := handler.NewRentalAPI(svc, func(c *gin.Context) (string, bool) {
		if sessionID == "" {
			return "", false
		}
		return sessionID, true
	})
	r := gin.New()
	api.RegisterHandlers(r, rentalServer{api: rentalAPI})
	return r
}

type rentalServer struct {
	handler.F5Stubs
	api *handler.RentalAPI
}

func (rentalServer) GetAccount(*gin.Context)                             { panic("unused") }
func (rentalServer) UpdateAccount(*gin.Context)                          { panic("unused") }
func (rentalServer) DeactivateAccount(*gin.Context)                      { panic("unused") }
func (rentalServer) StartGoogleAuth(*gin.Context)                        { panic("unused") }
func (rentalServer) GoogleCallback(*gin.Context)                         { panic("unused") }
func (rentalServer) Logout(*gin.Context)                                 { panic("unused") }
func (rentalServer) Healthz(*gin.Context)                                { panic("unused") }
func (rentalServer) Readyz(*gin.Context)                                 { panic("unused") }
func (rentalServer) ListMineListings(*gin.Context)                       { panic("unused") }
func (rentalServer) CreateListingDraft(*gin.Context)                     { panic("unused") }
func (rentalServer) GetMyListing(*gin.Context, openapiUUID)              { panic("unused") }
func (rentalServer) UpdateListing(*gin.Context, openapiUUID)             { panic("unused") }
func (rentalServer) PublishListing(*gin.Context, openapiUUID)            { panic("unused") }
func (rentalServer) PauseListing(*gin.Context, openapiUUID)              { panic("unused") }
func (rentalServer) ListBlocks(*gin.Context, openapiUUID)                { panic("unused") }
func (rentalServer) AddBlock(*gin.Context, openapiUUID)                  { panic("unused") }
func (rentalServer) RemoveBlock(*gin.Context, openapiUUID, openapiUUID)  { panic("unused") }
func (rentalServer) GetOwnerOnboarding(*gin.Context)                     { panic("unused") }
func (rentalServer) UpdateOwnerOnboarding(*gin.Context)                  { panic("unused") }
func (rentalServer) ListCategories(*gin.Context)                         { panic("unused") }
func (rentalServer) SearchCatalog(*gin.Context, api.SearchCatalogParams) { panic("unused") }
func (rentalServer) GetPublicListing(*gin.Context, openapiUUID)          { panic("unused") }
func (rentalServer) GetPublicCalendar(*gin.Context, openapiUUID, api.GetPublicCalendarParams) {
	panic("unused")
}
func (rentalServer) PaymentWebhook(*gin.Context, api.PaymentWebhookParams) { panic("unused") }

func (s rentalServer) CreateRental(c *gin.Context)  { s.api.CreateRental(c) }
func (s rentalServer) ListMyRentals(c *gin.Context) { s.api.ListMyRentals(c) }
func (s rentalServer) GetRental(c *gin.Context, id openapiUUID) {
	s.api.GetRental(c, id)
}
func (s rentalServer) AuthorizeRentalPayment(c *gin.Context, id openapiUUID) {
	s.api.AuthorizeRentalPayment(c, id)
}
func (s rentalServer) AcceptRental(c *gin.Context, id openapiUUID) {
	s.api.AcceptRental(c, id)
}
func (s rentalServer) DeclineRental(c *gin.Context, id openapiUUID) {
	s.api.DeclineRental(c, id)
}
func (s rentalServer) CancelRental(c *gin.Context, id openapiUUID) {
	s.api.CancelRental(c, id)
}
func (s rentalServer) CreateRentalCancellation(c *gin.Context, id openapiUUID) {
	s.api.CreateRentalCancellation(c, id)
}
func (s rentalServer) GetRentalCancellation(c *gin.Context, id openapiUUID) {
	s.api.GetRentalCancellation(c, id)
}
func (s rentalServer) GetRentalReceipt(c *gin.Context, id openapiUUID) {
	s.api.GetRentalReceipt(c, id)
}

// --- tests --------------------------------------------------------------

func TestCreateRental_HappyPath(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	w := httptest.NewRecorder()
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z","with_operator":false,"operator_terms_accepted":false}`
	req, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateRental_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z"}`
	req, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateRental_InvalidPayload(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRental_ServiceErrorPropagates(t *testing.T) {
	// Build a router with a fake that always returns an error so the
	// writeServiceErr branch runs end-to-end.
	gin.SetMode(gin.TestMode)
	svc := newRentSvcFake()
	svc.createErr = rental.ErrCalendarOverlap
	api2 := handler.NewRentalAPI(svc, func(c *gin.Context) (string, bool) { return "tenant-1", true })
	r := gin.New()
	api.RegisterHandlers(r, rentalServer{api: api2})
	w := httptest.NewRecorder()
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z"}`
	req, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
	var errOut api.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errOut))
	require.Equal(t, "calendar_overlap", errOut.Code)
}

func TestListMyRentals_HappyPath(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rentals", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var out []api.Rental
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
}

func TestListMyRentals_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rentals", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetRental_HappyPath(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	// Create a rental first via the POST endpoint.
	post := httptest.NewRecorder()
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z"}`
	preq, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
	preq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(post, preq)
	require.Equal(t, http.StatusCreated, post.Code)

	// Now list to grab the id.
	list := httptest.NewRecorder()
	lreq, _ := http.NewRequest("GET", "/rentals", nil)
	r.ServeHTTP(list, lreq)
	var items []api.Rental
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &items))
	require.NotEmpty(t, items)

	get := httptest.NewRecorder()
	greq, _ := http.NewRequest("GET", "/rentals/"+items[0].Id.String(), nil)
	r.ServeHTTP(get, greq)
	require.Equal(t, http.StatusOK, get.Code)
}

func TestGetRental_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rentals/11111111-1111-4111-8111-111111111111", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthorizeRentalPayment_HappyPath(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	// Seed rental.
	post := httptest.NewRecorder()
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z"}`
	preq, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
	preq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(post, preq)
	list := httptest.NewRecorder()
	lreq, _ := http.NewRequest("GET", "/rentals", nil)
	r.ServeHTTP(list, lreq)
	var items []api.Rental
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &items))
	require.NotEmpty(t, items)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals/"+items[0].Id.String()+"/authorize", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestAuthorizeRentalPayment_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals/11111111-1111-4111-8111-111111111111/authorize", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAcceptRental_HappyPath(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	post := httptest.NewRecorder()
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z"}`
	preq, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
	preq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(post, preq)
	list := httptest.NewRecorder()
	lreq, _ := http.NewRequest("GET", "/rentals", nil)
	r.ServeHTTP(list, lreq)
	var items []api.Rental
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &items))
	require.NotEmpty(t, items)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals/"+items[0].Id.String()+"/accept", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestAcceptRental_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals/11111111-1111-4111-8111-111111111111/accept", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeclineRental_HappyPath(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	post := httptest.NewRecorder()
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z"}`
	preq, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
	preq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(post, preq)
	list := httptest.NewRecorder()
	lreq, _ := http.NewRequest("GET", "/rentals", nil)
	r.ServeHTTP(list, lreq)
	var items []api.Rental
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &items))
	require.NotEmpty(t, items)

	w := httptest.NewRecorder()
	reason := `{"reason":"owner_unavailable"}`
	req, _ := http.NewRequest("POST", "/rentals/"+items[0].Id.String()+"/decline", bytes.NewReader([]byte(reason)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestDeclineRental_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals/11111111-1111-4111-8111-111111111111/decline", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCancelRental_HappyPath(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	post := httptest.NewRecorder()
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z"}`
	preq, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
	preq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(post, preq)
	list := httptest.NewRecorder()
	lreq, _ := http.NewRequest("GET", "/rentals", nil)
	r.ServeHTTP(list, lreq)
	var items []api.Rental
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &items))
	require.NotEmpty(t, items)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals/"+items[0].Id.String()+"/cancel", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestCreateRentalCancellation_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newRentSvcFake()
	svc.seedRental("11111111-1111-4111-8111-111111111111", "tenant-1")
	api2 := handler.NewRentalAPI(svc, func(c *gin.Context) (string, bool) { return "tenant-1", true })
	r := gin.New()
	api.RegisterHandlers(r, rentalServer{api: api2})
	w := httptest.NewRecorder()
	body := `{"actor_kind":"tenant","reason":"change of plans"}`
	req, _ := http.NewRequest("POST", "/rentals/11111111-1111-4111-8111-111111111111/cancellations", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	var got api.RentalCancellation
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "tenant", string(got.ActorKind))
}

func TestCreateRentalCancellation_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals/11111111-1111-4111-8111-111111111111/cancellations", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetRentalCancellation_NotFound(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rentals/11111111-1111-4111-8111-111111111111/cancellations", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetRentalCancellation_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newRentSvcFake()
	svc.seedRental("11111111-1111-4111-8111-111111111111", "tenant-1")
	svc.seedCancellation("11111111-1111-4111-8111-111111111111")
	api2 := handler.NewRentalAPI(svc, func(c *gin.Context) (string, bool) { return "tenant-1", true })
	r := gin.New()
	api.RegisterHandlers(r, rentalServer{api: api2})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rentals/11111111-1111-4111-8111-111111111111/cancellations", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestCancelRental_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/rentals/11111111-1111-4111-8111-111111111111/cancel", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetRentalReceipt_RequiresSession(t *testing.T) {
	r := newRentalRouter(t, "")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rentals/11111111-1111-4111-8111-111111111111/receipt", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetRentalReceipt_NotFound(t *testing.T) {
	r := newRentalRouter(t, "tenant-1")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rentals/11111111-1111-4111-8111-111111111111/receipt", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetRentalReceipt_HappyPath(t *testing.T) {
	// Build a custom router whose fake has a seeded receipt.
	gin.SetMode(gin.TestMode)
	svc := newRentSvcFake()
	svc.seedReceipt("11111111-1111-4111-8111-111111111111", "tenant-1")
	api3 := handler.NewRentalAPI(svc, func(c *gin.Context) (string, bool) { return "tenant-1", true })
	r := gin.New()
	api.RegisterHandlers(r, rentalServer{api: api3})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/rentals/11111111-1111-4111-8111-111111111111/receipt", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

// --- writeServiceErr branch coverage -----------------------------------

type rentSvcErrFake struct {
	rentSvcFake
}

func (f *rentSvcErrFake) Accept(_ context.Context, _ rentsvc.AcceptInput) (rental.Rental, error) {
	return rental.Rental{}, rental.ErrAcceptanceExpired
}

func (f *rentSvcErrFake) Decline(_ context.Context, _ rentsvc.DeclineInput) (rental.Rental, error) {
	return rental.Rental{}, rental.ErrForbidden
}

func (f *rentSvcErrFake) CancelPreAuth(_ context.Context, _ rentsvc.CancelPreAuthInput) (rental.Rental, error) {
	return rental.Rental{}, rental.ErrInvalidTransition
}

func (f *rentSvcErrFake) AuthorizeIntent(_ context.Context, _ rentsvc.AuthorizeIntentInput) (rentsvc.PaymentIntent, error) {
	return rentsvc.PaymentIntent{}, rental.ErrTenantHasDebt
}

func (f *rentSvcErrFake) Get(_ context.Context, _ string) (rental.Rental, error) {
	return rental.Rental{}, rental.ErrNotFound
}

func TestRental_ErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &rentSvcErrFake{rentSvcFake: *newRentSvcFake()}
	api4 := handler.NewRentalAPI(svc, func(c *gin.Context) (string, bool) { return "tenant-1", true })
	r := gin.New()
	api.RegisterHandlers(r, rentalServer{api: api4})

	// Seed a rental so the Get path doesn't bail at ErrNotFound before writeServiceErr.
	id := uuid.NewString()
	svc.rentals[id] = rental.Rental{ID: id, ListingID: "L", TenantAccountID: "tenant-1", State: rental.StatePending}

	cases := []struct {
		method, path string
		wantCode     string
		wantStatus   int
	}{
		{method: "GET", path: "/rentals/" + id, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{method: "POST", path: "/rentals/" + id + "/accept", wantStatus: http.StatusConflict, wantCode: "acceptance_expired"},
		{method: "POST", path: "/rentals/" + id + "/decline", wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{method: "POST", path: "/rentals/" + id + "/cancel", wantStatus: http.StatusConflict, wantCode: "invalid_transition"},
		{method: "POST", path: "/rentals/" + id + "/authorize", wantStatus: http.StatusForbidden, wantCode: "tenant_has_debt"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tc.method, tc.path, nil)
		r.ServeHTTP(w, req)
		require.Equal(t, tc.wantStatus, w.Code, "method=%s path=%s", tc.method, tc.path)
		var errOut api.Error
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errOut))
		require.Equal(t, tc.wantCode, errOut.Code, "method=%s path=%s", tc.method, tc.path)
	}
}

// Cover the remaining writeServiceErr branches by injecting errors directly
// via the existing rentSvcFake and reading the response code/message.
type injectFake struct {
	*rentSvcFake
	err error
}

func (i injectFake) CreateIntent(_ context.Context, _ rentsvc.CreateIntentInput) (rental.Rental, rental.MoneyBreakdown, error) {
	return rental.Rental{}, rental.MoneyBreakdown{}, i.err
}

func TestRental_WriteServiceErrAllBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"listing_id":"11111111-1111-4111-8111-111111111111","starts_at":"2026-10-10T10:00:00Z","ends_at":"2026-10-10T12:00:00Z"}`
	cases := []struct {
		err        error
		wantCode   string
		wantStatus int
	}{
		{err: rental.ErrInvalidInput, wantCode: "invalid_input", wantStatus: http.StatusUnprocessableEntity},
		{err: rental.ErrOperatorTermsRequired, wantCode: "operator_terms_required", wantStatus: http.StatusUnprocessableEntity},
		{err: rental.ErrOperatorNotAvailable, wantCode: "operator_not_available", wantStatus: http.StatusUnprocessableEntity},
		{err: rental.ErrProfileIncomplete, wantCode: "profile_incomplete", wantStatus: http.StatusForbidden},
		{err: rental.ErrListingUnavailable, wantCode: "listing_unavailable", wantStatus: http.StatusConflict},
		{err: rental.ErrPaymentTotalMismatch, wantCode: "payment_mismatch", wantStatus: http.StatusUnprocessableEntity},
		{err: errors.New("other"), wantCode: "internal_error", wantStatus: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		base := newRentSvcFake()
		svc := injectFake{rentSvcFake: base, err: tc.err}
		api5 := handler.NewRentalAPI(svc, func(c *gin.Context) (string, bool) { return "tenant-1", true })
		r := gin.New()
		api.RegisterHandlers(r, rentalServer{api: api5})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/rentals", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, tc.wantStatus, w.Code, "err=%v", tc.err)
		var errOut api.Error
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errOut))
		require.Equal(t, tc.wantCode, errOut.Code, "err=%v", tc.err)
	}
}

// Force reference to listing/account so unused-import checks are happy
// for the wider test package — these symbols are referenced by the suite.
var _ = listing.StatePublished
var _ = account.StatusActive
var _ = rental.ErrInvalidInput
