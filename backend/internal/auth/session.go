package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"
)

const defaultCookie = "canteiro_session"

// ErrNoSession is a missing or unknown session cookie.
var ErrNoSession = errors.New("no session")

// Session is a revocable login stored in the backing service.
type Session struct {
	ExpiresAt time.Time
	ID        string
	AccountID string
	TokenHash string
}

// SessionStore persists session hashes (never the raw token).
type SessionStore interface {
	Create(sess Session) error
	GetByTokenHash(hash string) (Session, error)
	DeleteByTokenHash(hash string) error
}

// CookieSettings controls the session cookie. Values come from env.
type CookieSettings struct { //nolint:govet // fieldalignment vs env mapping
	Name   string
	TTL    time.Duration
	Secure bool
}

func (c CookieSettings) name() string {
	if c.Name == "" {
		return defaultCookie
	}
	return c.Name
}

func (c CookieSettings) ttl() time.Duration {
	if c.TTL <= 0 {
		return 7 * 24 * time.Hour
	}
	return c.TTL
}

// NewToken returns a raw cookie value and its sha256 hex hash.
func NewToken() (raw, hash string, err error) {
	var b [32]byte
	if _, err = rand.Read(b[:]); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b[:])
	return raw, HashToken(raw), nil
}

// HashToken is the lookup key stored in Postgres.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func setSessionCookie(w http.ResponseWriter, cfg CookieSettings, raw string, r *http.Request) {
	writeSessionCookie(w, cfg, raw, int(cfg.ttl().Seconds()), r)
}

func clearSessionCookie(w http.ResponseWriter, cfg CookieSettings, r *http.Request) {
	writeSessionCookie(w, cfg, "", -1, r)
}

func writeSessionCookie(w http.ResponseWriter, cfg CookieSettings, raw string, maxAge int, r *http.Request) {
	//nolint:gosec // G124 Secure follows SESSION_COOKIE_SECURE or the request proto
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.name(),
		Value:    raw,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.secureFor(r),
	})
}

// secureFor is true when env forces Secure or the request arrived over HTTPS
// (direct TLS or X-Forwarded-Proto). The public host is HTTPS; local HTTP
// stays Secure=false so the cookie is stored.
func (c CookieSettings) secureFor(r *http.Request) bool {
	if c.Secure {
		return true
	}
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func rawCookie(r *http.Request, cfg CookieSettings) (string, error) {
	c, err := r.Cookie(cfg.name())
	if err != nil || c.Value == "" {
		return "", ErrNoSession
	}
	return c.Value, nil
}
