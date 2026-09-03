package rental

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHasOverlap(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		aStart, aEnd, bStart, bEnd time.Time
		name                       string
		want                       bool
	}{
		{name: "identical windows overlap", aStart: base, aEnd: base.Add(2 * time.Hour), bStart: base, bEnd: base.Add(2 * time.Hour), want: true},
		{name: "a strictly before b", aStart: base, aEnd: base.Add(time.Hour), bStart: base.Add(2 * time.Hour), bEnd: base.Add(3 * time.Hour), want: false},
		{name: "b strictly before a", aStart: base.Add(2 * time.Hour), aEnd: base.Add(3 * time.Hour), bStart: base, bEnd: base.Add(time.Hour), want: false},
		{name: "touching at start (half-open)", aStart: base, aEnd: base.Add(time.Hour), bStart: base.Add(time.Hour), bEnd: base.Add(2 * time.Hour), want: false},
		{name: "touching at end (half-open)", aStart: base, aEnd: base.Add(time.Hour), bStart: base.Add(-time.Hour), bEnd: base, want: false},
		{name: "partial inner overlap", aStart: base, aEnd: base.Add(3 * time.Hour), bStart: base.Add(time.Hour), bEnd: base.Add(2 * time.Hour), want: true},
		{name: "b fully inside a", aStart: base, aEnd: base.Add(3 * time.Hour), bStart: base.Add(time.Hour), bEnd: base.Add(2 * time.Hour), want: true},
		{name: "degenerate a returns false", aStart: base, aEnd: base, bStart: base, bEnd: base.Add(time.Hour), want: false},
		{name: "degenerate b returns false", aStart: base, aEnd: base.Add(time.Hour), bStart: base, bEnd: base, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HasOverlap(tc.aStart, tc.aEnd, tc.bStart, tc.bEnd)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCanTransition(t *testing.T) {
	t.Parallel()
	require.True(t, CanTransition(StatePending, StateAuthorized))
	require.True(t, CanTransition(StatePending, StateCancelled))
	require.True(t, CanTransition(StateAuthorized, StateConfirmed))
	require.True(t, CanTransition(StateAuthorized, StateDeclined))
	require.True(t, CanTransition(StateAuthorized, StateExpired))
	require.True(t, CanTransition(StateAuthorized, StateRefunded))
	require.True(t, CanTransition(StateConfirmed, StateRefunded))
	require.False(t, CanTransition(StatePending, StateConfirmed))
	require.False(t, CanTransition(StatePending, StateExpired))
	require.False(t, CanTransition(StateAuthorized, StatePending))
	require.False(t, CanTransition(StateConfirmed, StateDeclined))
	require.False(t, CanTransition(StateDeclined, StateConfirmed))
	require.False(t, CanTransition(StateExpired, StateAuthorized))
	require.False(t, CanTransition(StateRefunded, StateConfirmed))
	require.False(t, CanTransition(State("garbage"), StateAuthorized))
}

func TestStateOccupiesCalendar(t *testing.T) {
	t.Parallel()
	require.True(t, StateAuthorized.OccupiesCalendar())
	require.True(t, StateConfirmed.OccupiesCalendar())
	require.False(t, StatePending.OccupiesCalendar())
	require.False(t, StateDeclined.OccupiesCalendar())
	require.False(t, StateExpired.OccupiesCalendar())
	require.False(t, StateCancelled.OccupiesCalendar())
	require.False(t, StateRefunded.OccupiesCalendar())
}

func TestStateTerminal(t *testing.T) {
	t.Parallel()
	require.False(t, StatePending.Terminal())
	require.False(t, StateAuthorized.Terminal())
	require.True(t, StateConfirmed.Terminal())
	require.True(t, StateDeclined.Terminal())
	require.True(t, StateExpired.Terminal())
	require.True(t, StateCancelled.Terminal())
	require.True(t, StateRefunded.Terminal())
}
