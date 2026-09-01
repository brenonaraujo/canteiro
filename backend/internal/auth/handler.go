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
	base := strings.TrimRight(a.deps.WebAppURL, "/")
	if base == "" {
		base = "/"
	}
	u, err := url.Parse(base)
	if err != nil {
		c.Status(http.StatusFound)
		return
	}
	q := u.Query()
	q.Set("auth", result)
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
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
