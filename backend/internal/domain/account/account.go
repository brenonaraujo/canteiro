package account

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxName  = 80
	maxPhone = 32
)

// Status is the explicit account state consumed by F2+.
type Status string

const (
	// StatusIncomplete is a Google account without visible name or phone.
	StatusIncomplete Status = "incomplete"
	// StatusActive may reserve (F3) once profile is complete.
	StatusActive Status = "active"
	// StatusDeactivated cannot start a new reserve or listing.
	StatusDeactivated Status = "deactivated"
)

// Account is a Google-linked marketplace identity. No email is stored.
type Account struct {
	DeactivatedAt *time.Time
	ID            string
	GoogleSubject string
	DisplayName   string
	Phone         string
	Status        Status
}

// Profile is a validated visible name and phone pair.
type Profile struct {
	DisplayName string
	Phone       string
}

// ProfileComplete is true when both visible name and phone are set.
func (a Account) ProfileComplete() bool {
	return strings.TrimSpace(a.DisplayName) != "" && strings.TrimSpace(a.Phone) != ""
}

// CanReserve is the F3 gate. F1 owns the rule; F3 must call it.
func (a Account) CanReserve() error {
	if a.Status == StatusDeactivated {
		return ErrDeactivated
	}
	if a.Status != StatusActive || !a.ProfileComplete() {
		return ErrProfileIncomplete
	}
	return nil
}

// CanPublish is the F2 gate. F1 always denies (owner onboarding is F2).
func (a Account) CanPublish() error {
	if err := a.CanReserve(); err != nil {
		return err
	}
	return ErrOwnerOnboardingRequired
}

// ValidateProfile trims and rejects empty or oversized fields.
func ValidateProfile(name, phone string) (Profile, error) {
	n := strings.TrimSpace(name)
	p := strings.TrimSpace(phone)
	if n == "" || p == "" || utf8.RuneCountInString(n) > maxName || utf8.RuneCountInString(p) > maxPhone {
		return Profile{}, ErrInvalidProfile
	}
	return Profile{DisplayName: n, Phone: p}, nil
}

// ApplyProfile sets name/phone and promotes incomplete → active.
func (a Account) ApplyProfile(p Profile) Account {
	a.DisplayName = p.DisplayName
	a.Phone = p.Phone
	if a.Status == StatusIncomplete && a.ProfileComplete() {
		a.Status = StatusActive
	}
	return a
}

// Deactivate is irreversible in v1. It does not cancel rentals.
func (a Account) Deactivate(now time.Time) Account {
	if a.Status == StatusDeactivated {
		return a
	}
	a.Status = StatusDeactivated
	t := now.UTC()
	a.DeactivatedAt = &t
	return a
}
