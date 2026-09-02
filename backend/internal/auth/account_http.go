package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
)

// GetAccount returns the caller profile. Google subject is never included.
func (a *API) GetAccount(c *gin.Context) {
	acc, ok := a.requireAccount(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, toAPI(acc))
}

// UpdateAccount sets visible name and phone.
func (a *API) UpdateAccount(c *gin.Context) {
	acc, ok := a.requireAccount(c)
	if !ok {
		return
	}
	var req api.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		a.writeErr(c, http.StatusUnprocessableEntity, "invalid_profile", "account.invalid_profile")
		return
	}
	got, err := a.deps.Accounts.UpdateProfile(c.Request.Context(), acc.ID, req.DisplayName, req.Phone)
	if a.writeAccountErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, toAPI(got))
}

// DeactivateAccount is irreversible in v1 and does not cancel rentals.
func (a *API) DeactivateAccount(c *gin.Context) {
	acc, ok := a.requireAccount(c)
	if !ok {
		return
	}
	var req api.DeactivateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		a.writeErr(c, http.StatusUnprocessableEntity, "confirm_required", "account.confirm_required")
		return
	}
	got, err := a.deps.Accounts.Deactivate(c.Request.Context(), acc.ID)
	if a.writeAccountErr(c, err) {
		return
	}
	incDeactivate()
	c.JSON(http.StatusOK, toAPI(got))
}

func (a *API) requireAccount(c *gin.Context) (account.Account, bool) {
	acc, err := a.current(c)
	if err != nil {
		if errors.Is(err, ErrNoSession) {
			a.writeErr(c, http.StatusUnauthorized, "unauthorized", "auth.unauthorized")
			return account.Account{}, false
		}
		a.writeErr(c, http.StatusInternalServerError, "internal_error", "error.internal")
		return account.Account{}, false
	}
	return acc, true
}

func (a *API) writeAccountErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, account.ErrInvalidProfile):
		a.writeErr(c, http.StatusUnprocessableEntity, "invalid_profile", "account.invalid_profile")
	case errors.Is(err, account.ErrDeactivated):
		a.writeErr(c, http.StatusForbidden, "deactivated", "account.deactivated")
	case errors.Is(err, account.ErrNotFound):
		a.writeErr(c, http.StatusUnauthorized, "unauthorized", "auth.unauthorized")
	default:
		a.writeErr(c, http.StatusInternalServerError, "internal_error", "error.internal")
	}
	return true
}
