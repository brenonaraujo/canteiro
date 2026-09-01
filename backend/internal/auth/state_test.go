package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/auth"
)

func TestState_RoundTripAndRejectsTamper(t *testing.T) {
	t.Parallel()
	st := auth.NewState("0123456789abcdef0123456789abcdef")
	raw, err := st.Issue()
	require.NoError(t, err)
	require.NoError(t, st.Verify(raw))
	require.Error(t, st.Verify(raw+"x"))
	require.Error(t, st.Verify(""))
	st2 := auth.NewState("ffffffffffffffffffffffffffffffff")
	require.Error(t, st2.Verify(raw))
}

func TestState_Expired(t *testing.T) {
	t.Parallel()
	st := auth.NewState("0123456789abcdef0123456789abcdef")
	st.SetNow(func() time.Time { return time.Unix(1_700_000_000, 0) })
	raw, err := st.Issue()
	require.NoError(t, err)
	st.SetNow(func() time.Time { return time.Unix(1_700_000_000, 0).Add(11 * time.Minute) })
	require.Error(t, st.Verify(raw))
}

func TestState_NotReady(t *testing.T) {
	t.Parallel()
	st := auth.NewState("short")
	require.False(t, st.Ready())
	_, err := st.Issue()
	require.Error(t, err)
}
