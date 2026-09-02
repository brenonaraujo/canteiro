package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// Service is the F1 account use-cases. Skill: pre-implementation-design
// (atomic Ensure/Update/Deactivate; no helper split).
type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() string
}

// NewService wires a repository. Clock and ids are replaceable in tests.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, newID: newID}
}

// EnsureFromGoogle creates an incomplete account or reopens the existing one.
func (s *Service) EnsureFromGoogle(ctx context.Context, subject string) (Account, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return Account{}, ErrNotFound
	}
	acc, err := s.repo.GetByGoogleSubject(ctx, subject)
	if err == nil {
		return acc, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Account{}, err
	}
	acc = Account{ID: s.newID(), GoogleSubject: subject, Status: StatusIncomplete}
	if err := s.repo.Create(ctx, acc); err != nil {
		if errors.Is(err, ErrDuplicateGoogle) {
			return s.repo.GetByGoogleSubject(ctx, subject)
		}
		return Account{}, err
	}
	return acc, nil
}

// UpdateProfile sets name and phone. Empty values are rejected.
func (s *Service) UpdateProfile(ctx context.Context, id, name, phone string) (Account, error) {
	acc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Account{}, err
	}
	if acc.Status == StatusDeactivated {
		return Account{}, ErrDeactivated
	}
	p, err := ValidateProfile(name, phone)
	if err != nil {
		return Account{}, err
	}
	acc = acc.ApplyProfile(p)
	if err := s.repo.Update(ctx, acc); err != nil {
		return Account{}, err
	}
	return acc, nil
}

// GetByID loads an account. Unknown ids return ErrNotFound.
func (s *Service) GetByID(ctx context.Context, id string) (Account, error) {
	return s.repo.GetByID(ctx, id)
}

// Deactivate marks the account deactivated. Rentals are not cancelled.
func (s *Service) Deactivate(ctx context.Context, id string) (Account, error) {
	acc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Account{}, err
	}
	acc = acc.Deactivate(s.now())
	if err := s.repo.Update(ctx, acc); err != nil {
		return Account{}, err
	}
	return acc, nil
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
