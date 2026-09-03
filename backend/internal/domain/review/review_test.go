package review_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
)

// Review.Validate and scope helpers. Pure-function coverage — no IO.

func TestReview_Validate_RejectsBadScore(t *testing.T) {
	for _, score := range []int{0, -1, 6, 100} {
		r := review.Review{
			RentalID:    "r1",
			RaterUserID: "u1",
			Scope:       review.ScopeOwner,
			RateeUserID: "u2",
			Score:       score,
		}
		err := r.Validate()
		require.Error(t, err)
		require.ErrorIs(t, err, review.ErrInvalidInput, "score=%d", score)
	}
}

func TestReview_Validate_AcceptsScores1Through5(t *testing.T) {
	for _, score := range []int{1, 2, 3, 4, 5} {
		r := review.Review{
			RentalID:    "r1",
			RaterUserID: "u1",
			Scope:       review.ScopeOwner,
			RateeUserID: "u2",
			Score:       score,
		}
		require.NoError(t, r.Validate(), "score=%d", score)
	}
}

func TestReview_Validate_RejectsEmptyRentalOrRater(t *testing.T) {
	r := review.Review{
		RaterUserID: "u1",
		Scope:       review.ScopeOwner,
		RateeUserID: "u2",
		Score:       5,
	}
	require.ErrorIs(t, r.Validate(), review.ErrInvalidInput)

	r = review.Review{
		RentalID: "r1",
		Scope:    review.ScopeOwner,
		Score:    5,
	}
	require.ErrorIs(t, r.Validate(), review.ErrInvalidInput)
}

func TestReview_Validate_RejectsUnknownScope(t *testing.T) {
	r := review.Review{
		RentalID:    "r1",
		RaterUserID: "u1",
		Scope:       review.Scope("bogus"),
		Score:       5,
	}
	err := r.Validate()
	require.ErrorIs(t, err, review.ErrScopeInvalid)
}

func TestReview_Validate_RequiresRateeForUserScopes(t *testing.T) {
	r := review.Review{
		RentalID:    "r1",
		RaterUserID: "u1",
		Scope:       review.ScopeOwner,
		// RateeUserID empty
		Score: 5,
	}
	require.ErrorIs(t, r.Validate(), review.ErrInvalidInput)
}

func TestReview_Validate_AllowsEmptyRateeForListingScope(t *testing.T) {
	r := review.Review{
		RentalID:    "r1",
		RaterUserID: "u1",
		Scope:       review.ScopeListing,
		Score:       5,
	}
	require.NoError(t, r.Validate())
}

func TestReview_Validate_RejectsSelfReview(t *testing.T) {
	r := review.Review{
		RentalID:    "r1",
		RaterUserID: "u1",
		RateeUserID: "u1", // same as rater
		Scope:       review.ScopeOwner,
		Score:       5,
	}
	err := r.Validate()
	require.ErrorIs(t, err, review.ErrSelfReview)
}

func TestReview_Validate_StripsControlChars(t *testing.T) {
	r := review.Review{
		RentalID:    "r1",
		RaterUserID: "u1",
		RateeUserID: "u2",
		Scope:       review.ScopeOwner,
		Score:       5,
		Comment:     "Bom\x00bem\x07atend\x0c.",
	}
	require.NoError(t, r.Validate())
	require.Equal(t, "Bombematend.", r.Comment, "control bytes stripped")
}

func TestReview_Validate_PreservesNewlinesAndTabs(t *testing.T) {
	r := review.Review{
		ID:          "rv-1",
		RentalID:    "r1",
		RaterUserID: "u1",
		RateeUserID: "u2",
		Scope:       review.ScopeOwner,
		Score:       5,
		Comment:     "line1\nline2\tcol2",
	}
	require.NoError(t, r.Validate())
	require.Equal(t, "line1\nline2\tcol2", r.Comment)
}

func TestReview_Validate_RejectsOversizedComment(t *testing.T) {
	big := make([]byte, review.MaxCommentBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	r := review.Review{
		RentalID:    "r1",
		RaterUserID: "u1",
		RateeUserID: "u2",
		Scope:       review.ScopeOwner,
		Score:       5,
		Comment:     string(big),
	}
	require.ErrorIs(t, r.Validate(), review.ErrInvalidInput)
}

// --- Scope / participant helpers ---------------------------------------

func TestScope_IsValid(t *testing.T) {
	require.True(t, review.ScopeListing.IsValid())
	require.True(t, review.ScopeOwner.IsValid())
	require.True(t, review.ScopeOperator.IsValid())
	require.True(t, review.ScopeRenter.IsValid())
	require.False(t, review.Scope("").IsValid())
	require.False(t, review.Scope("widget").IsValid())
}

func TestParticipantRoles_AllowedRaterScopes_RenterWithOperator(t *testing.T) {
	p := review.ParticipantRoles{
		RenterID: "ren-1", OwnerID: "own-1", OperatorID: "op-1",
	}
	scopes := p.AllowedRaterScopes("ren-1")
	require.ElementsMatch(t, []review.Scope{
		review.ScopeListing, review.ScopeOwner, review.ScopeOperator,
	}, scopes)
}

func TestParticipantRoles_AllowedRaterScopes_RenterNoOperator(t *testing.T) {
	p := review.ParticipantRoles{
		RenterID: "ren-1", OwnerID: "own-1",
		// OperatorID empty: no operator slot
	}
	scopes := p.AllowedRaterScopes("ren-1")
	require.ElementsMatch(t, []review.Scope{review.ScopeListing, review.ScopeOwner}, scopes)
}

func TestParticipantRoles_AllowedRaterScopes_RenterOperatorIsOwner(t *testing.T) {
	// EC-5: operator == owner → no separate operator scope.
	p := review.ParticipantRoles{
		RenterID: "ren-1", OwnerID: "own-1", OperatorID: "own-1",
	}
	scopes := p.AllowedRaterScopes("ren-1")
	require.ElementsMatch(t, []review.Scope{review.ScopeListing, review.ScopeOwner}, scopes)
}

func TestParticipantRoles_AllowedRaterScopes_OwnerRates(t *testing.T) {
	p := review.ParticipantRoles{
		RenterID: "ren-1", OwnerID: "own-1",
	}
	scopes := p.AllowedRaterScopes("own-1")
	require.ElementsMatch(t, []review.Scope{review.ScopeRenter}, scopes)
}

func TestParticipantRoles_AllowedRaterScopes_Stranger(t *testing.T) {
	p := review.ParticipantRoles{
		RenterID: "ren-1", OwnerID: "own-1",
	}
	require.Nil(t, p.AllowedRaterScopes("stranger"))
}

func TestParticipantRoles_RateeID(t *testing.T) {
	p := review.ParticipantRoles{
		RenterID: "ren-1", OwnerID: "own-1", OperatorID: "op-1",
	}
	require.Equal(t, "own-1", p.RateeID(review.ScopeOwner))
	require.Equal(t, "ren-1", p.RateeID(review.ScopeRenter))
	require.Equal(t, "op-1", p.RateeID(review.ScopeOperator))
	require.Empty(t, p.RateeID(review.ScopeListing), "listing scope has no user ratee")
}

// --- Aggregate math ----------------------------------------------------

func TestCompute_Averages(t *testing.T) {
	// 5 + 4 + 3 + 5 + 1 = 18 / 5 = 3.6
	require.InDelta(t, 3.6, review.Compute(5, 18), 0.001)
	require.InDelta(t, 0, review.Compute(0, 0), 0.001)
	require.InDelta(t, 0, review.Compute(-1, 5), 0.001, "negative count returns 0")
}

func TestNewAggregate_ShapeAndRoundedAvg(t *testing.T) {
	a := review.NewAggregate("u-1", review.ScopeOwner, 5, 18)
	require.Equal(t, "u-1", a.RateeUserID)
	require.Equal(t, review.ScopeOwner, a.Scope)
	require.Equal(t, int64(5), a.Count)
	require.Equal(t, int64(18), a.Sum)
	require.InDelta(t, 3.6, a.Avg, 0.001)
	require.False(t, a.Zero())
}

func TestNewAggregate_ZeroShape(t *testing.T) {
	a := review.NewAggregate("u-2", review.ScopeOperator, 0, 0)
	require.True(t, a.Zero())
	require.InDelta(t, 0, a.Avg, 0.001)
}

func TestAggregateKey(t *testing.T) {
	require.Equal(t, "u-1|owner", review.AggregateKey("u-1", review.ScopeOwner))
}
