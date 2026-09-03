package rental_test

import (
	"testing"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// F5 state machine — table-driven coverage for CanReturnTransition,
// CanDamageTransition and CanDebtTransition. These are the heart of Pilar 1
// (state machine for the devolução) and Pilar 4 (defense as a gate); any
// invalid transition leaking past these checks will let a wrong state
// persist and break the F5 invariants.
func TestF5StateMachines(t *testing.T) {
	t.Run("ReturnState: happy path", func(t *testing.T) {
		cases := []struct {
			from, to rental.ReturnState
			ok       bool
		}{
			{rental.ReturnAwaitingPickup, rental.ReturnInProgress, true},
			{rental.ReturnInProgress, rental.ReturnAwaitingConfirmation, true},
			{rental.ReturnInProgress, rental.ReturnClosed, true},
			{rental.ReturnAwaitingConfirmation, rental.ReturnClosed, true},
			{rental.ReturnAwaitingConfirmation, rental.ReturnContested, true},
			{rental.ReturnContested, rental.ReturnClosed, true},
		}
		for _, c := range cases {
			if got := rental.CanReturnTransition(c.from, c.to); got != c.ok {
				t.Errorf("CanReturnTransition(%q,%q)=%v want %v", c.from, c.to, got, c.ok)
			}
		}
	})

	t.Run("ReturnState: forbidden transitions", func(t *testing.T) {
		cases := []struct {
			from, to rental.ReturnState
		}{
			{rental.ReturnAwaitingPickup, rental.ReturnClosed},          // cannot skip pickup
			{rental.ReturnAwaitingPickup, rental.ReturnContested},       // cannot contest before pickup
			{rental.ReturnClosed, rental.ReturnInProgress},              // terminal
			{rental.ReturnClosed, rental.ReturnAwaitingConfirmation},    // terminal
			{rental.ReturnContested, rental.ReturnAwaitingConfirmation}, // cannot un-contest
			{rental.ReturnContested, rental.ReturnInProgress},           // cannot un-contest
		}
		for _, c := range cases {
			if rental.CanReturnTransition(c.from, c.to) {
				t.Errorf("CanReturnTransition(%q,%q) should be false", c.from, c.to)
			}
		}
	})

	t.Run("DamageState: happy path and defenses", func(t *testing.T) {
		cases := []struct {
			from, to rental.DamageState
			ok       bool
		}{
			{rental.DamageOpen, rental.DamageRenterAgreed, true},
			{rental.DamageOpen, rental.DamageContested, true},
			{rental.DamageOpen, rental.DamageExpired, true},
			{rental.DamageOpen, rental.DamageCancelled, true},
			{rental.DamageContested, rental.DamageStaffResolved, true},
			{rental.DamageContested, rental.DamageRenterAgreed, true},
			{rental.DamageContested, rental.DamageCancelled, true},
		}
		for _, c := range cases {
			if got := rental.CanDamageTransition(c.from, c.to); got != c.ok {
				t.Errorf("CanDamageTransition(%q,%q)=%v want %v", c.from, c.to, got, c.ok)
			}
		}
	})

	t.Run("DamageState: forbidden transitions", func(t *testing.T) {
		cases := []struct {
			from, to rental.DamageState
		}{
			{rental.DamageOpen, rental.DamageStaffResolved},      // staff cannot resolve without contest
			{rental.DamageRenterAgreed, rental.DamageContested},  // already agreed
			{rental.DamageExpired, rental.DamageContested},       // expired is terminal
			{rental.DamageStaffResolved, rental.DamageContested}, // terminal
			{rental.DamageCancelled, rental.DamageRenterAgreed},  // terminal
		}
		for _, c := range cases {
			if rental.CanDamageTransition(c.from, c.to) {
				t.Errorf("CanDamageTransition(%q,%q) should be false", c.from, c.to)
			}
		}
	})

	t.Run("DebtState: only open transitions out", func(t *testing.T) {
		if !rental.CanDebtTransition(rental.DebtOpen, rental.DebtSettled) {
			t.Error("open -> settled must be allowed")
		}
		if !rental.CanDebtTransition(rental.DebtOpen, rental.DebtForgiven) {
			t.Error("open -> forgiven must be allowed")
		}
		if rental.CanDebtTransition(rental.DebtSettled, rental.DebtForgiven) {
			t.Error("settled -> forgiven should be forbidden")
		}
		if rental.CanDebtTransition(rental.DebtForgiven, rental.DebtSettled) {
			t.Error("forgiven -> settled should be forbidden")
		}
	})
}

func TestDamageNature_Valid(t *testing.T) {
	for _, n := range rental.AllDamageNatures {
		if !n.IsValid() {
			t.Errorf("nature %q from AllDamageNatures must be valid", n)
		}
	}
	bad := rental.DamageNature("??")
	if bad.IsValid() {
		t.Error("unknown nature must be invalid")
	}
}
