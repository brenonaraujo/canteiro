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
		name                       string
		aStart, aEnd, bStart, bEnd time.Time
		want                       bool
	}{
		{"identical windows overlap", base, base.Add(2 * time.Hour), base, base.Add(2 * time.Hour), true},
		{"a strictly before b", base, base.Add(time.Hour), base.Add(2 * time.Hour), base.Add(3 * time.Hour), false},
		{"b strictly before a", base.Add(2 * time.Hour), base.Add(3 * time.Hour), base, base.Add(time.Hour), false},
		{"touching at start (half-open)", base, base.Add(time.Hour), base.Add(time.Hour), base.Add(2 * time.Hour), false},
		{"touching at end (half-open)", base, base.Add(time.Hour), base.Add(-time.Hour), base, false},
		{"partial inner overlap", base, base.Add(3 * time.Hour), base.Add(time.Hour), base.Add(2 * time.Hour), true},
		{"b fully inside a", base, base.Add(3 * time.Hour), base.Add(time.Hour), base.Add(2 * time.Hour), true},
		{"degenerate a returns false", base, base, base, base.Add(time.Hour), false},
		{"degenerate b returns false", base, base.Add(time.Hour), base, base, false},
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
