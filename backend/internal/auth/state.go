package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"
)

const stateTTL = 10 * time.Minute

// ErrInvalidState is a missing, expired, or tampered OAuth state.
var ErrInvalidState = errors.New("invalid oauth state")

// State signs CSRF tokens without process memory.
type State struct {
	now    func() time.Time
	secret []byte
}

// NewState requires a secret of at least 16 bytes.
func NewState(secret string) *State {
	return &State{secret: []byte(secret), now: time.Now}
}

// Ready reports whether signing is configured.
func (s *State) Ready() bool {
	return s != nil && len(s.secret) >= 16
}

// SetNow replaces the clock (tests).
func (s *State) SetNow(now func() time.Time) {
	if s != nil && now != nil {
		s.now = now
	}
}

// Issue returns a signed, time-limited state value.
func (s *State) Issue() (string, error) {
	if !s.Ready() {
		return "", ErrInvalidState
	}
	exp := s.now().Add(stateTTL).Unix()
	var payload [8]byte
	binary.BigEndian.PutUint64(payload[:], uint64(exp)) //nolint:gosec // G115 unix seconds
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(payload[:])
	out := append(payload[:], mac.Sum(nil)...)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

// Verify rejects missing, expired, or forged state.
func (s *State) Verify(raw string) error {
	if !s.Ready() || raw == "" {
		return ErrInvalidState
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) != 8+sha256.Size {
		return ErrInvalidState
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(b[:8])
	if !hmac.Equal(mac.Sum(nil), b[8:]) {
		return ErrInvalidState
	}
	exp := int64(binary.BigEndian.Uint64(b[:8])) //nolint:gosec // G115 unix seconds
	if s.now().Unix() > exp {
		return ErrInvalidState
	}
	return nil
}
