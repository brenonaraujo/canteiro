package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL = "https://oauth2.googleapis.com/token"
	defaultInfoURL  = "https://oauth2.googleapis.com/tokeninfo"
	httpTimeout     = 10 * time.Second
	maxBody         = 1 << 20
)

// Google talks to Google OAuth over HTTP. Secrets stay in env/config.
type Google struct {
	HTTP         *http.Client
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	InfoURL      string
}

// Configured is true when client id, secret and redirect are set.
func (g *Google) Configured() bool {
	return g != nil && g.ClientID != "" && g.ClientSecret != "" && g.RedirectURL != ""
}

// AuthCodeURL builds the consent redirect. Scope is openid only.
func (g *Google) AuthCodeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("redirect_uri", g.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid")
	q.Set("state", state)
	base := g.AuthURL
	if base == "" {
		base = defaultAuthURL
	}
	return base + "?" + q.Encode()
}

// Exchange trades a code for a verified Google subject.
func (g *Google) Exchange(ctx context.Context, code string) (Identity, error) {
	if code == "" || !g.Configured() {
		return Identity{}, errors.New("google exchange failed")
	}
	idToken, err := g.token(ctx, code)
	if err != nil {
		return Identity{}, err
	}
	return g.subject(ctx, idToken)
}

func (g *Google) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: httpTimeout}
}

func (g *Google) token(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("redirect_uri", g.RedirectURL)
	form.Set("grant_type", "authorization_code")
	tokenURL := g.TokenURL
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("google exchange failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var body struct {
		IDToken string `json:"id_token"`
	}
	if err := decodeJSON(g.client(), req, &body); err != nil || body.IDToken == "" {
		return "", errors.New("google exchange failed")
	}
	return body.IDToken, nil
}

func (g *Google) subject(ctx context.Context, idToken string) (Identity, error) {
	infoURL := g.InfoURL
	if infoURL == "" {
		infoURL = defaultInfoURL
	}
	u, err := url.Parse(infoURL)
	if err != nil {
		return Identity{}, errors.New("google exchange failed")
	}
	q := u.Query()
	q.Set("id_token", idToken)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Identity{}, errors.New("google exchange failed")
	}
	var info struct {
		Aud string `json:"aud"`
		Sub string `json:"sub"`
	}
	if err := decodeJSON(g.client(), req, &info); err != nil {
		return Identity{}, errors.New("google exchange failed")
	}
	if info.Aud != g.ClientID || info.Sub == "" {
		return Identity{}, errors.New("google exchange failed")
	}
	return Identity{Subject: info.Sub}, nil
}

func decodeJSON(client *http.Client, req *http.Request, dest any) error {
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxBody))
		return errors.New("google exchange failed")
	}
	return json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(dest)
}
