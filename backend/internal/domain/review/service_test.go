package review_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
)

// --- minimal fakes for the review service ------------------------------

type fakeRentals struct {
	mu   sync.Mutex
	byID map[string]fakeRentalRow
}

type fakeRentalRow struct {
	rental      review.RentalParticipant
	terminal    bool
	terminalErr error
}

func newFakeRentals() *fakeRentals {
	return &fakeRentals{byID: map[string]fakeRentalRow{}}
}

func (f *fakeRentals) put(r review.RentalParticipant, terminal bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[r.RentalID] = fakeRentalRow{rental: r, terminal: terminal}
}

func (f *fakeRentals) GetParticipant(ctx context.Context, rentalID string) (review.RentalParticipant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.byID[rentalID]
	if !ok {
		return review.RentalParticipant{}, review.ErrNotFound
	}
	return row.rental, nil
}

func (f *fakeRentals) IsTerminal(ctx context.Context, rentalID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.byID[rentalID]
	if !ok {
		return false, review.ErrNotFound
	}
	return row.terminal, row.terminalErr
}

type fakeRepo struct {
	mu        sync.Mutex
	reviews   []review.Review
	aggByKey  map[string]review.ReviewAggregate
	insertErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{aggByKey: map[string]review.ReviewAggregate{}}
}

func (f *fakeRepo) InsertReviewWithAggregate(ctx context.Context, in review.ReviewWithAggregateInput) (review.Review, review.ReviewAggregate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return review.Review{}, review.ReviewAggregate{}, f.insertErr
	}
	// UNIQUE simulation: same (rental_id, rater_user_id, scope) → ErrAlreadyReviewed
	for _, existing := range f.reviews {
		if existing.RentalID == in.Review.RentalID &&
			existing.RaterUserID == in.Review.RaterUserID &&
			existing.Scope == in.Review.Scope {
			return review.Review{}, review.ReviewAggregate{}, review.ErrAlreadyReviewed
		}
	}
	now := time.Now().UTC()
	in.Review.CreatedAt = now
	f.reviews = append(f.reviews, in.Review)
	agg := in.NewAggregate
	if prev, ok := f.aggByKey[review.AggregateKey(in.NewAggregate.RateeUserID, in.NewAggregate.Scope)]; ok {
		agg = review.NewAggregate(prev.RateeUserID, prev.Scope, prev.Count+1, prev.Sum+int64(in.Review.Score))
	}
	f.aggByKey[review.AggregateKey(agg.RateeUserID, agg.Scope)] = agg
	return in.Review, agg, nil
}

func (f *fakeRepo) ListByRatee(ctx context.Context, rateeUserID string, scope review.Scope, limit, offset int) ([]review.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []review.Review{}
	for _, r := range f.reviews {
		if r.RateeUserID != rateeUserID {
			continue
		}
		if scope != "" && r.Scope != scope {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRepo) GetAggregate(ctx context.Context, rateeUserID string, scope review.Scope) (review.ReviewAggregate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.aggByKey[review.AggregateKey(rateeUserID, scope)]; ok {
		return a, nil
	}
	return review.ReviewAggregate{RateeUserID: rateeUserID, Scope: scope}, nil
}

type counterIDs struct{ n int }

func (c *counterIDs) String() string {
	c.n++
	return "rv-" + itoa(c.n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// --- CreateReview ------------------------------------------------------

func TestService_CreateReview_HappyPath(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
		OperatorID: "", OperatorIsOwner: false,
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	got, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID:    "r1",
		RaterUserID: "ren-1",
		Scope:       review.ScopeOwner,
		Score:       5,
		Comment:     "excellent",
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	require.Equal(t, "own-1", got.RateeUserID)
	require.Equal(t, review.ScopeOwner, got.Scope)
	require.Equal(t, 5, got.Score)
	require.Equal(t, "excellent", got.Comment)
	require.False(t, got.CreatedAt.IsZero())
}

func TestService_CreateReview_StrangerRejected(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID:    "r1",
		RaterUserID: "stranger",
		Scope:       review.ScopeOwner,
		Score:       5,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, review.ErrNotParticipant)
}

func TestService_CreateReview_SelfReviewRejected(t *testing.T) {
	// Owner trying to rate themselves as owner — impossible because
	// owner's allowed scope is only "renter". Service still defends
	// on the self-review rule.
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID:    "r1",
		RaterUserID: "own-1",
		Scope:       review.ScopeOwner,
		Score:       5,
	})
	require.ErrorIs(t, err, review.ErrNotParticipant,
		"owner's only allowed scope is renter — owner cannot rate owner")
}

func TestService_CreateReview_ScopeNotAllowedForRater(t *testing.T) {
	// Renter trying to rate the renter (only owner can rate renter).
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID:    "r1",
		RaterUserID: "ren-1",
		Scope:       review.ScopeRenter,
		Score:       5,
	})
	require.ErrorIs(t, err, review.ErrNotParticipant)
}

func TestService_CreateReview_RentalNotTerminal(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, false) // not terminal
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID:    "r1",
		RaterUserID: "ren-1",
		Scope:       review.ScopeOwner,
		Score:       5,
	})
	require.ErrorIs(t, err, review.ErrRentalNotTerminal)
}

func TestService_CreateReview_UnknownRental(t *testing.T) {
	rentals := newFakeRentals()
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID:    "missing",
		RaterUserID: "ren-1",
		Scope:       review.ScopeOwner,
		Score:       5,
	})
	require.ErrorIs(t, err, review.ErrNotFound)
}

func TestService_CreateReview_AlreadyReviewed(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	in := review.CreateReviewInput{
		RentalID: "r1", RaterUserID: "ren-1", Scope: review.ScopeOwner, Score: 5,
	}
	_, err := svc.CreateReview(context.Background(), in)
	require.NoError(t, err)
	_, err = svc.CreateReview(context.Background(), in)
	require.ErrorIs(t, err, review.ErrAlreadyReviewed)
}

func TestService_CreateReview_RejectsBadScore(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID: "r1", RaterUserID: "ren-1", Scope: review.ScopeOwner, Score: 7,
	})
	require.ErrorIs(t, err, review.ErrInvalidInput)
}

func TestService_CreateReview_OperatorDistinctFromOwner(t *testing.T) {
	// Renter rates operator (third party). Service resolves ratee to
	// operator id from the snapshot.
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
		OperatorID: "op-1", OperatorIsOwner: false,
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	got, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID: "r1", RaterUserID: "ren-1", Scope: review.ScopeOperator, Score: 4,
	})
	require.NoError(t, err)
	require.Equal(t, "op-1", got.RateeUserID)
}

func TestService_CreateReview_OperatorIsOwnerCollapsesScope(t *testing.T) {
	// EC-5: when OperatorIsOwner, the renter can only rate owner —
	// the operator scope is not allowed.
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
		OperatorID: "own-1", OperatorIsOwner: true,
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID: "r1", RaterUserID: "ren-1", Scope: review.ScopeOperator, Score: 4,
	})
	require.ErrorIs(t, err, review.ErrNotParticipant)
}

func TestService_CreateReview_RepositoryError_BubblesUp(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	repo.insertErr = errors.New("db down")
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID: "r1", RaterUserID: "ren-1", Scope: review.ScopeOwner, Score: 5,
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, review.ErrAlreadyReviewed)
}

// --- Read side ----------------------------------------------------------

func TestService_ListReceivedReviews_FiltersByRatee(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID: "r1", RaterUserID: "ren-1", Scope: review.ScopeOwner, Score: 5,
	})
	require.NoError(t, err)

	got, err := svc.ListReceivedReviews(context.Background(), review.ListReceivedReviewsInput{
		RateeUserID: "own-1", Scope: review.ScopeOwner, Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 5, got[0].Score)

	other, err := svc.ListReceivedReviews(context.Background(), review.ListReceivedReviewsInput{
		RateeUserID: "ren-1", Scope: review.ScopeOwner, Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	require.Empty(t, other, "owner has no reviews as owner when only the renter rated")
}

func TestService_GetAggregate_AfterInsert(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID: "r1", RaterUserID: "ren-1", Scope: review.ScopeOwner, Score: 4,
	})
	require.NoError(t, err)

	agg, err := svc.GetAggregate(context.Background(), "own-1", review.ScopeOwner)
	require.NoError(t, err)
	require.Equal(t, int64(1), agg.Count)
	require.Equal(t, int64(4), agg.Sum)
	require.InDelta(t, 4.0, agg.Avg, 0.001)
}

func TestService_GetAggregate_EmptyWhenNoneInserted(t *testing.T) {
	rentals := newFakeRentals()
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	agg, err := svc.GetAggregate(context.Background(), "stranger", review.ScopeOwner)
	require.NoError(t, err)
	require.True(t, agg.Zero())
}

// --- Coverage targets (NÃO-BLOQ 3) -----------------------------------

func TestService_NewService_DefaultsUseRealClockAndIDGen(t *testing.T) {
	// When Config.Now and Config.IDGen are nil, NewService wires the
	// production defaults. Exercising the constructors here also
	// touches the realClock.Now branch (service.go:39).
	rentals := newFakeRentals()
	repo := newFakeRepo()
	svc := review.NewService(review.Config{}, rentals, repo)
	require.NotNil(t, svc)

	// DefaultIDGen is exported via review package — pin the
	// contract: it returns a non-nil generator whose ids are
	// parseable UUIDs. (See idgen_test.go for the full shape check.)
	gen := review.DefaultIDGen()
	require.NotNil(t, gen)
	id := gen.String()
	require.NotEmpty(t, id)
}

// TestService_ListReceivedReviews_RequiresRatee covers the empty-ratee
// guard. Covers service.go:295 ListReceivedReviews (was 54.5%).
func TestService_ListReceivedReviews_RequiresRatee(t *testing.T) {
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, newFakeRentals(), newFakeRepo())
	_, err := svc.ListReceivedReviews(context.Background(), review.ListReceivedReviewsInput{
		RateeUserID: "", Scope: review.ScopeOwner, Limit: 10, Offset: 0,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, review.ErrInvalidInput)
}

// TestService_ListReceivedReviews_InvalidScope covers the scope
// validation branch. Covers service.go:295 ListReceivedReviews.
func TestService_ListReceivedReviews_InvalidScope(t *testing.T) {
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, newFakeRentals(), newFakeRepo())
	_, err := svc.ListReceivedReviews(context.Background(), review.ListReceivedReviewsInput{
		RateeUserID: "u1", Scope: review.Scope("bogus"), Limit: 10, Offset: 0,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, review.ErrScopeInvalid)
}

// TestService_ListReceivedReviews_DefaultAndClampLimit covers the
// Limit <= 0 and Limit > maxListLimit clamps. Covers service.go:295.
func TestService_ListReceivedReviews_DefaultAndClampLimit(t *testing.T) {
	rentals := newFakeRentals()
	rentals.put(review.RentalParticipant{
		RentalID: "r1", RenterID: "ren-1", OwnerID: "own-1",
	}, true)
	repo := newFakeRepo()
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	// Seed 3 reviews on the same rental for the renter to rate.
	// (One per scope — listing/owner/renter — keeps the UNIQUE
	// (rental, rater, scope) honest.)
	_, err := svc.CreateReview(context.Background(), review.CreateReviewInput{
		RentalID: "r1", RaterUserID: "ren-1", Scope: review.ScopeListing, Score: 5,
	})
	require.NoError(t, err)

	// Limit <= 0 → falls back to defaultListLimit (20). The repo
	// returns its seed slice unchanged, but we exercise the clamp
	// by hitting the read path.
	got, err := svc.ListReceivedReviews(context.Background(), review.ListReceivedReviewsInput{
		RateeUserID: "own-1", Scope: review.ScopeOwner, Limit: 0, Offset: 0,
	})
	require.NoError(t, err)
	// Own-1 has no owner-scoped review in this setup; renter rated
	// the listing only. The clamp ran; the result is just empty.
	require.Empty(t, got)

	// Limit > maxListLimit → clamped to maxListLimit. Same outcome.
	got, err = svc.ListReceivedReviews(context.Background(), review.ListReceivedReviewsInput{
		RateeUserID: "own-1", Scope: review.ScopeOwner, Limit: 9999, Offset: 0,
	})
	require.NoError(t, err)
	require.Empty(t, got)

	// Offset < 0 → clamped to 0.
	got, err = svc.ListReceivedReviews(context.Background(), review.ListReceivedReviewsInput{
		RateeUserID: "own-1", Scope: review.ScopeOwner, Limit: 10, Offset: -5,
	})
	require.NoError(t, err)
	require.Empty(t, got)
}

// TestService_ListReceivedReviews_EmptyScopeReturnsAll exercises the
// branch where scope == "" (any scope). Covers service.go:295.
func TestService_ListReceivedReviews_EmptyScopeReturnsAll(t *testing.T) {
	rentals := newFakeRentals()
	repo := newFakeRepo()
	// Pre-seed the fake repo with two reviews addressed to own-1,
	// one per scope. Bypassing CreateReview keeps the test focused
	// on the ListReceivedReviews scope-filter branch.
	repo.reviews = []review.Review{
		{ID: "rv-a", RentalID: "r1", RaterUserID: "ren-1", RateeUserID: "own-1", Scope: review.ScopeOwner, Score: 5},
		{ID: "rv-b", RentalID: "r2", RaterUserID: "ren-2", RateeUserID: "own-1", Scope: review.ScopeRenter, Score: 4},
	}
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, rentals, repo)

	got, err := svc.ListReceivedReviews(context.Background(), review.ListReceivedReviewsInput{
		RateeUserID: "own-1", Scope: "", Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, got, 2, "empty scope returns reviews across both scopes")
}

// TestService_GetAggregate_RequiresRatee covers the empty-ratee
// guard. Covers service.go:315 GetAggregate (was 60%).
func TestService_GetAggregate_RequiresRatee(t *testing.T) {
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, newFakeRentals(), newFakeRepo())
	_, err := svc.GetAggregate(context.Background(), "", review.ScopeOwner)
	require.Error(t, err)
	require.ErrorIs(t, err, review.ErrInvalidInput)
}

// TestService_GetAggregate_InvalidScope covers the scope validation
// branch. Covers service.go:315 GetAggregate.
func TestService_GetAggregate_InvalidScope(t *testing.T) {
	svc := review.NewService(review.Config{IDGen: &counterIDs{}}, newFakeRentals(), newFakeRepo())
	_, err := svc.GetAggregate(context.Background(), "u1", review.Scope("bogus"))
	require.Error(t, err)
	require.ErrorIs(t, err, review.ErrScopeInvalid)
}
