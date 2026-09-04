package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// StartGoogleAuth redirects to Google or 503 if the provider is unset.
func (a *API) StartGoogleAuth(c *gin.Context) {
	if !a.configured() {
		a.writeErr(c, http.StatusServiceUnavailable, "not_configured", "auth.not_configured")
		return
	}
	state, err := a.deps.State.Issue()
	if err != nil {
		a.writeErr(c, http.StatusServiceUnavailable, "not_configured", "auth.not_configured")
		return
	}
	incLogin("start")
	c.Redirect(http.StatusFound, a.deps.Google.AuthCodeURL(state))
}

// GoogleCallback finishes Google sign-in and sets the session cookie.
func (a *API) GoogleCallback(c *gin.Context) {
	if c.Query("error") != "" {
		incLogin("denied")
		a.redirect(c, "denied")
		return
	}
	if !a.configured() || a.deps.State.Verify(c.Query("state")) != nil {
		incLogin("error")
		a.redirect(c, "error")
		return
	}
	a.finishGoogle(c)
}

func (a *API) finishGoogle(c *gin.Context) {
	ident, err := a.deps.Google.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil || ident.Subject == "" {
		incLogin("error")
		a.redirect(c, "error")
		return
	}
	acc, err := a.deps.Accounts.EnsureFromGoogle(c.Request.Context(), ident.Subject)
	if err != nil {
		incLogin("error")
		a.redirect(c, "error")
		return
	}
	if err := a.issueSession(c, acc.ID); err != nil {
		incLogin("error")
		a.redirect(c, "error")
		return
	}
	incLogin("ok")
	a.redirect(c, "ok")
}

func (a *API) issueSession(c *gin.Context, accountID string) error {
	raw, hash, err := NewToken()
	if err != nil {
		return err
	}
	sess := Session{
		ID:        newSessionID(),
		AccountID: accountID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(a.deps.Cookie.ttl()),
	}
	if err := a.deps.Sessions.Create(sess); err != nil {
		return err
	}
	setSessionCookie(c.Writer, a.deps.Cookie, raw, c.Request)
	return nil
}

func newSessionID() string {
	raw, _, err := NewToken()
	if err != nil {
		return ""
	}
	if len(raw) < 32 {
		return raw
	}
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32]
}

// Logout ends the session. It does not deactivate the account.
func (a *API) Logout(c *gin.Context) {
	raw, err := rawCookie(c.Request, a.deps.Cookie)
	if err != nil {
		a.writeErr(c, http.StatusUnauthorized, "unauthorized", "auth.unauthorized")
		return
	}
	_ = a.deps.Sessions.DeleteByTokenHash(HashToken(raw))
	clearSessionCookie(c.Writer, a.deps.Cookie, c.Request)
	incLogout()
	c.Status(http.StatusNoContent)
}
