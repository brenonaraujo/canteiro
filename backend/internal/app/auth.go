package app

import (
	"github.com/brenonaraujo/canteiro/backend/internal/auth"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"

	"gorm.io/gorm"
)

// NewAuthAPI wires Google, sessions and accounts. Missing env keeps auth unconfigured.
func NewAuthAPI(cfg *Config, db *gorm.DB) *auth.API {
	if cfg == nil {
		return auth.NewAPI(auth.Deps{})
	}
	deps := auth.Deps{
		Google: googleFrom(cfg),
		State:  auth.NewState(cfg.SessionSecret),
		Cookie: auth.CookieSettings{
			Name:   cfg.SessionCookieName,
			Secure: cfg.SessionCookieSecure,
			TTL:    cfg.SessionTTL,
		},
		WebAppURL: cfg.WebAppURL,
	}
	if db != nil {
		deps.Accounts = account.NewService(auth.PGAccounts{DB: db})
		deps.Sessions = auth.PGSessions{DB: db}
	}
	return auth.NewAPI(deps)
}

func googleFrom(cfg *Config) auth.Exchanger {
	g := &auth.Google{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
	}
	if !g.Configured() {
		return nil
	}
	return g
}
