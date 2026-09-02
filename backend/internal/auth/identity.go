package auth

import "context"

// Identity is the Google subject only. Email is never stored.
type Identity struct {
	Subject string
}

// Exchanger is the Google identity backing service.
type Exchanger interface {
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (Identity, error)
}
