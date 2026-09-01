package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
)

func TestMapAccount_NotFound(t *testing.T) {
	t.Parallel()
	_, err := mapAccount(accountRow{}, gorm.ErrRecordNotFound)
	require.ErrorIs(t, err, account.ErrNotFound)
}

func TestToRow_RoundTripStatus(t *testing.T) {
	t.Parallel()
	acc := account.Account{ID: "id-1", GoogleSubject: "sub", Status: account.StatusActive, DisplayName: "Ana", Phone: "1"}
	row := toRow(acc)
	got, err := mapAccount(row, nil)
	require.NoError(t, err)
	require.Equal(t, acc, got)
}

func TestIsUnique(t *testing.T) {
	t.Parallel()
	require.False(t, isUnique(nil))
	require.True(t, isUnique(gorm.ErrDuplicatedKey))
}
