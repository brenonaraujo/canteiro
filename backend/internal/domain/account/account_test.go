package account_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
)

func TestCapabilities_Table(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantReserve error
		wantPublish error
		acc         account.Account
		name        string
	}{
		{
			name:        "new google account cannot reserve or publish",
			acc:         account.Account{Status: account.StatusIncomplete},
			wantReserve: account.ErrProfileIncomplete,
			wantPublish: account.ErrProfileIncomplete,
		},
		{
			name: "empty name still incomplete",
			acc: account.Account{
				Status:      account.StatusIncomplete,
				DisplayName: "",
				Phone:       "+5511999999999",
			},
			wantReserve: account.ErrProfileIncomplete,
			wantPublish: account.ErrProfileIncomplete,
		},
		{
			name: "empty phone still incomplete",
			acc: account.Account{
				Status:      account.StatusIncomplete,
				DisplayName: "Ana",
				Phone:       "  ",
			},
			wantReserve: account.ErrProfileIncomplete,
			wantPublish: account.ErrProfileIncomplete,
		},
		{
			name: "complete profile may reserve but never publish in F1",
			acc: account.Account{
				Status:      account.StatusActive,
				DisplayName: "Ana",
				Phone:       "+5511999999999",
			},
			wantReserve: nil,
			wantPublish: account.ErrOwnerOnboardingRequired,
		},
		{
			name: "deactivated cannot start reserve or publish",
			acc: account.Account{
				Status:      account.StatusDeactivated,
				DisplayName: "Ana",
				Phone:       "+5511999999999",
			},
			wantReserve: account.ErrDeactivated,
			wantPublish: account.ErrDeactivated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, tt.acc.CanReserve(), tt.wantReserve)
			require.ErrorIs(t, tt.acc.CanPublish(), tt.wantPublish)
		})
	}
}

func TestProfileComplete(t *testing.T) {
	t.Parallel()
	require.False(t, account.Account{}.ProfileComplete())
	require.True(t, account.Account{DisplayName: "Ana", Phone: "1199"}.ProfileComplete())
}

func TestValidateProfile_RejectsEmpty(t *testing.T) {
	t.Parallel()
	_, err := account.ValidateProfile("  ", "1199")
	require.ErrorIs(t, err, account.ErrInvalidProfile)
	_, err = account.ValidateProfile("Ana", "")
	require.ErrorIs(t, err, account.ErrInvalidProfile)
}

func TestValidateProfile_Trims(t *testing.T) {
	t.Parallel()
	name, err := account.ValidateProfile("  Ana  ", " 1199 ")
	require.NoError(t, err)
	assert.Equal(t, "Ana", name.DisplayName)
	assert.Equal(t, "1199", name.Phone)
}
