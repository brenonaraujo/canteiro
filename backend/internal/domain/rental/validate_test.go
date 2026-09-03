package rental

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateWindow_RejectsZeroTimestamps(t *testing.T) {
	t.Parallel()
	err := ValidateWindow(time.Time{}, time.Time{}, time.Time{}, 0)
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestValidateWindow_RejectsEndsBeforeStart(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	err := ValidateWindow(start, start.Add(-time.Hour), time.Time{}, 0)
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestValidateWindow_EnforcesLeadTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	start := now.Add(time.Hour) // less than the 12h lead
	end := start.Add(time.Hour)
	err := ValidateWindow(start, end, now, 12*time.Hour)
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestValidateWindow_AllowsSufficientLeadTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	start := now.Add(13 * time.Hour)
	end := start.Add(time.Hour)
	require.NoError(t, ValidateWindow(start, end, now, 12*time.Hour))
}

func TestRentalValidate_RequiresID(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	r.ID = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidInput)
}

func TestRentalValidate_RequiresListingID(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	r.ListingID = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidInput)
}

func TestRentalValidate_RequiresTenant(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	r.TenantAccountID = ""
	require.ErrorIs(t, r.Validate(), ErrInvalidInput)
}

func TestRentalValidate_RejectsNegativeCents(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	r.RentCents = -1
	require.ErrorIs(t, r.Validate(), ErrInvalidInput)
}

func TestRentalValidate_RejectsZeroTotal(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	r.RentCents = 0
	r.OperatorCents = 0
	r.DepositCents = 0
	require.ErrorIs(t, r.Validate(), ErrInvalidInput)
}

func TestRentalValidate_RequiresOperatorTermsWhenRequired(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	r.ListingSnapshot.Operator.Mode = "required"
	r.OperatorTermsAccepted = false
	require.ErrorIs(t, r.Validate(), ErrOperatorTermsRequired)
}

func TestRentalValidate_RejectsOperatorWhenModeNone(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	r.ListingSnapshot.Operator.Mode = "none"
	r.WithOperator = true
	require.ErrorIs(t, r.Validate(), ErrOperatorNotAvailable)
}

func TestRentalValidate_RejectsInvalidState(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	r.State = StateConfirmed
	require.ErrorIs(t, r.Validate(), ErrInvalidInput)
}

func TestRentalValidate_HappyPath(t *testing.T) {
	t.Parallel()
	require.NoError(t, rentalStub().Validate())
}

func TestMarshalSnapshot_RoundTrip(t *testing.T) {
	t.Parallel()
	orig := rentalStub().ListingSnapshot
	b, err := MarshalSnapshot(orig)
	require.NoError(t, err)
	require.NotEmpty(t, b)
	got, err := UnmarshalSnapshot(b)
	require.NoError(t, err)
	require.Equal(t, orig, got)
}

func TestMarshalSnapshot_EmptyIsOK(t *testing.T) {
	t.Parallel()
	b, err := MarshalSnapshot(ListingSnapshot{})
	require.NoError(t, err)
	got, err := UnmarshalSnapshot(b)
	require.NoError(t, err)
	require.Equal(t, ListingSnapshot{}, got)
}

func TestUnmarshalSnapshot_RejectsGarbage(t *testing.T) {
	t.Parallel()
	_, err := UnmarshalSnapshot([]byte("not json"))
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestUnmarshalSnapshot_HandlesEmptyBytes(t *testing.T) {
	t.Parallel()
	got, err := UnmarshalSnapshot(nil)
	require.NoError(t, err)
	require.Equal(t, ListingSnapshot{}, got)
}

func TestMarshalSnapshot_ProducesValidJSON(t *testing.T) {
	t.Parallel()
	snap := rentalStub().ListingSnapshot
	b, err := MarshalSnapshot(snap)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(b, &raw))
	require.Equal(t, snap.OwnerID, raw["owner_id"])
	require.Equal(t, snap.Title, raw["title"])
}

func TestRental_IsOwnerAndIsTenant(t *testing.T) {
	t.Parallel()
	r := rentalStub()
	require.True(t, r.IsOwner("owner-1"))
	require.False(t, r.IsOwner("nobody"))
	require.False(t, r.IsTenant("owner-1"))
	require.True(t, r.IsTenant("tenant-1"))
}

// rentalStub returns a minimal valid Rental suitable for Validate().
func rentalStub() *Rental {
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	return &Rental{
		ID:                    "rental-1",
		ListingID:             "listing-1",
		TenantAccountID:       "tenant-1",
		RentCents:             1000,
		OperatorCents:         0,
		DepositCents:          5000,
		StartsAt:              start,
		EndsAt:                start.Add(2 * time.Hour),
		State:                 StatePending,
		ListingSnapshot: ListingSnapshot{
			OwnerID:  "owner-1",
			Title:    "Furadeira",
			Category: "electric",
			Operator: OperatorSnapshot{Mode: "none"},
		},
	}
}
