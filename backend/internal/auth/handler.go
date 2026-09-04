package auth

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
)

// Deps is HTTP wiring for F1 account endpoints.
type Deps struct {
	Accounts  *account.Service
	Sessions  SessionStore
	Google    Exchanger
	State     *State
	WebAppURL string
	Cookie    CookieSettings
}

// API implements the OpenAPI auth/account operations.
type API struct {
	deps Deps
}

// NewAPI returns the F1 HTTP adapter.
func NewAPI(d Deps) *API {
	return &API{deps: d}
}

func (a *API) configured() bool {
	return a != nil && a.deps.Google != nil && a.deps.State.Ready() && a.deps.Accounts != nil && a.deps.Sessions != nil
}

func (a *API) writeErr(c *gin.Context, status int, code, key string) {
	c.JSON(status, api.Error{
		Code:       code,
		Message:    i18n.T(c.Request.Context(), key),
		MessageKey: key,
	})
}

func (a *API) redirect(c *gin.Context, result string) {
	c.Redirect(http.StatusFound, postAuthLocation(a.deps.WebAppURL, c.Request, result))
}

func postAuthLocation(webApp string, r *http.Request, result string) string {
	q := "auth=" + url.QueryEscape(result)
	if samePublicHost(webApp, r) {
		return "/?" + q
	}
	return strings.TrimRight(webApp, "/") + "/?" + q
}

func samePublicHost(webApp string, r *http.Request) bool {
	if r == nil || strings.TrimSpace(webApp) == "" {
		return true
	}
	u, err := url.Parse(webApp)
	if err != nil || u.Host == "" {
		return true
	}
	reqHost := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		reqHost = h
	}
	return strings.EqualFold(u.Host, reqHost)
}

func toAPI(acc account.Account) api.Account {
	return api.Account{
		Id:          acc.ID,
		DisplayName: acc.DisplayName,
		Phone:       acc.Phone,
		Status:      api.AccountStatus(acc.Status),
		Capabilities: api.AccountCapabilities{
			Reserve: acc.CanReserve() == nil,
			Publish: acc.CanPublish() == nil,
		},
	}
}

func (a *API) current(c *gin.Context) (account.Account, error) {
	raw, err := rawCookie(c.Request, a.deps.Cookie)
	if err != nil {
		return account.Account{}, ErrNoSession
	}
	sess, err := a.deps.Sessions.GetByTokenHash(HashToken(raw))
	if err != nil {
		return account.Account{}, ErrNoSession
	}
	acc, err := a.deps.Accounts.GetByID(c.Request.Context(), sess.AccountID)
	if err != nil {
		if errors.Is(err, account.ErrNotFound) {
			return account.Account{}, ErrNoSession
		}
		return account.Account{}, err
	}
	return acc, nil
}

// CurrentAccountID returns the authenticated account id, or ("", false) when
// there is no session or the session cannot be resolved. Exposed for
// handlers (e.g. F2 listing) that need only the id and don't want to import
// the full account struct.
func (a *API) CurrentAccountID(c *gin.Context) (string, bool) {
	acc, err := a.current(c)
	if err != nil {
		return "", false
	}
	return acc.ID, true
}

// Accounts returns the account.Service this API was wired with. Other domain
// services (e.g. F2 listing) depend on a slice of the account service to
// resolve owners; exposing it here keeps the wiring in one place.
func (a *API) Accounts() *account.Service { return a.deps.Accounts }
