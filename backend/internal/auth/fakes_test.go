package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
)

type memSessions struct {
	byHash map[string]Session
	mu     sync.Mutex
}

func (m *memSessions) Create(sess Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byHash == nil {
		m.byHash = map[string]Session{}
	}
	m.byHash[sess.TokenHash] = sess
	return nil
}

func (m *memSessions) GetByTokenHash(hash string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byHash[hash]
	if !ok || time.Now().After(s.ExpiresAt) {
		return Session{}, ErrNoSession
	}
	return s, nil
}

func (m *memSessions) DeleteByTokenHash(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.byHash, hash)
	return nil
}

type memAccounts struct {
	byID     map[string]account.Account
	byGoogle map[string]string
	mu       sync.Mutex
}

func (m *memAccounts) GetByID(_ context.Context, id string) (account.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.byID[id]
	if !ok {
		return account.Account{}, account.ErrNotFound
	}
	return a, nil
}

func (m *memAccounts) GetByGoogleSubject(_ context.Context, subject string) (account.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byGoogle[subject]
	if !ok {
		return account.Account{}, account.ErrNotFound
	}
	return m.byID[id], nil
}

func (m *memAccounts) Create(_ context.Context, acc account.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byID == nil {
		m.byID = map[string]account.Account{}
		m.byGoogle = map[string]string{}
	}
	if _, ok := m.byGoogle[acc.GoogleSubject]; ok {
		return account.ErrDuplicateGoogle
	}
	m.byID[acc.ID] = acc
	m.byGoogle[acc.GoogleSubject] = acc.ID
	return nil
}

func (m *memAccounts) Update(_ context.Context, acc account.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[acc.ID]; !ok {
		return account.ErrNotFound
	}
	m.byID[acc.ID] = acc
	return nil
}

type fakeGoogle struct {
	codes map[string]Identity
}

func (f fakeGoogle) AuthCodeURL(state string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + url.QueryEscape(state)
}

func (f fakeGoogle) Exchange(_ context.Context, code string) (Identity, error) {
	id, ok := f.codes[code]
	if !ok {
		return Identity{}, errors.New("google exchange failed")
	}
	return id, nil
}

func testAPI() (*API, *State) {
	st := NewState("0123456789abcdef0123456789abcdef")
	apih := NewAPI(Deps{
		Accounts:  account.NewService(&memAccounts{}),
		Sessions:  &memSessions{},
		Google:    fakeGoogle{codes: map[string]Identity{"ok-code": {Subject: "google-sub-1"}}},
		State:     st,
		Cookie:    CookieSettings{Name: "canteiro_session", TTL: time.Hour},
		WebAppURL: "http://localhost:3000",
	})
	return apih, st
}

func routerFor(a *API) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), "pt-BR"))
		c.Next()
	})
	api.RegisterHandlers(r, wired{API: a})
	return r
}

type wired struct {
	*API
}

func (wired) Healthz(c *gin.Context) { c.Status(http.StatusOK) }
func (wired) Readyz(c *gin.Context)  { c.Status(http.StatusOK) }

// Catalog/listing routes are out of scope for the auth tests; they answer 404
// so the codegen dispatch stays satisfied without pulling in listing deps.
func (wired) ListCategories(c *gin.Context)                           { c.Status(http.StatusNotFound) }
func (wired) SearchCatalog(c *gin.Context, _ api.SearchCatalogParams) { c.Status(http.StatusNotFound) }
func (wired) GetPublicListing(c *gin.Context, _ openapi_types.UUID)   { c.Status(http.StatusNotFound) }
func (wired) GetPublicCalendar(c *gin.Context, _ openapi_types.UUID, _ api.GetPublicCalendarParams) {
	c.Status(http.StatusNotFound)
}
func (wired) ListMineListings(c *gin.Context)                     { c.Status(http.StatusNotFound) }
func (wired) CreateListingDraft(c *gin.Context)                   { c.Status(http.StatusNotFound) }
func (wired) GetMyListing(c *gin.Context, _ openapi_types.UUID)   { c.Status(http.StatusNotFound) }
func (wired) UpdateListing(c *gin.Context, _ openapi_types.UUID)  { c.Status(http.StatusNotFound) }
func (wired) ListBlocks(c *gin.Context, _ openapi_types.UUID)     { c.Status(http.StatusNotFound) }
func (wired) AddBlock(c *gin.Context, _ openapi_types.UUID)       { c.Status(http.StatusNotFound) }
func (wired) RemoveBlock(c *gin.Context, _, _ openapi_types.UUID) { c.Status(http.StatusNotFound) }
func (wired) PauseListing(c *gin.Context, _ openapi_types.UUID)   { c.Status(http.StatusNotFound) }
func (wired) PublishListing(c *gin.Context, _ openapi_types.UUID) { c.Status(http.StatusNotFound) }
func (wired) GetOwnerOnboarding(c *gin.Context)                   { c.Status(http.StatusNotFound) }
func (wired) UpdateOwnerOnboarding(c *gin.Context)                { c.Status(http.StatusNotFound) }

func do(r http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func mustAPI() *API {
	a, _ := testAPI()
	return a
}

func loadI18n(t *testing.T) error {
	t.Helper()
	_, err := i18n.Load()
	return err
}

func cookieValue(t *testing.T, w *httptest.ResponseRecorder, name string) string {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	t.Fatalf("missing cookie %s", name)
	return ""
}

func accountReq(r http.Handler, method, path, cookie, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "canteiro_session", Value: cookie})
	}
	return do(r, req)
}

func asMap(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}
