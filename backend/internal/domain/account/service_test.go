package account_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
)

type memRepo struct {
	byID     map[string]account.Account
	byGoogle map[string]string
	fail     error
	mu       sync.Mutex
}

func newMem() *memRepo {
	return &memRepo{byID: map[string]account.Account{}, byGoogle: map[string]string{}}
}

func (m *memRepo) GetByID(_ context.Context, id string) (account.Account, error) {
	if m.fail != nil {
		return account.Account{}, m.fail
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.byID[id]
	if !ok {
		return account.Account{}, account.ErrNotFound
	}
	return a, nil
}

func (m *memRepo) GetByGoogleSubject(_ context.Context, subject string) (account.Account, error) {
	if m.fail != nil {
		return account.Account{}, m.fail
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byGoogle[subject]
	if !ok {
		return account.Account{}, account.ErrNotFound
	}
	return m.byID[id], nil
}

func (m *memRepo) Create(_ context.Context, acc account.Account) error {
	if m.fail != nil {
		return m.fail
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byGoogle[acc.GoogleSubject]; ok {
		return account.ErrDuplicateGoogle
	}
	m.byID[acc.ID] = acc
	m.byGoogle[acc.GoogleSubject] = acc.ID
	return nil
}

func (m *memRepo) Update(_ context.Context, acc account.Account) error {
	if m.fail != nil {
		return m.fail
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[acc.ID]; !ok {
		return account.ErrNotFound
	}
	m.byID[acc.ID] = acc
	return nil
}

func TestEnsureFromGoogle_CreatesThenReopens(t *testing.T) {
	t.Parallel()
	svc := account.NewService(newMem())
	ctx := context.Background()
	first, err := svc.EnsureFromGoogle(ctx, "sub-1")
	require.NoError(t, err)
	require.Equal(t, account.StatusIncomplete, first.Status)
	require.ErrorIs(t, first.CanReserve(), account.ErrProfileIncomplete)
	second, err := svc.EnsureFromGoogle(ctx, "sub-1")
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
}

func TestEnsureFromGoogle_EmptySubject(t *testing.T) {
	t.Parallel()
	_, err := account.NewService(newMem()).EnsureFromGoogle(context.Background(), "  ")
	require.ErrorIs(t, err, account.ErrNotFound)
}

func TestEnsureFromGoogle_RepoError(t *testing.T) {
	t.Parallel()
	repo := newMem()
	repo.fail = assert.AnError
	_, err := account.NewService(repo).EnsureFromGoogle(context.Background(), "sub")
	require.Error(t, err)
}

func TestUpdateProfile_ActivatesAndBlocksEmpty(t *testing.T) {
	t.Parallel()
	svc := account.NewService(newMem())
	ctx := context.Background()
	acc, err := svc.EnsureFromGoogle(ctx, "sub-2")
	require.NoError(t, err)
	_, err = svc.UpdateProfile(ctx, acc.ID, "", "1199")
	require.ErrorIs(t, err, account.ErrInvalidProfile)
	got, err := svc.UpdateProfile(ctx, acc.ID, "Ana", "1199")
	require.NoError(t, err)
	assert.Equal(t, account.StatusActive, got.Status)
	require.NoError(t, got.CanReserve())
	require.ErrorIs(t, got.CanPublish(), account.ErrOwnerOnboardingRequired)
}

func TestDeactivate_DoesNotReopenActive(t *testing.T) {
	t.Parallel()
	svc := account.NewService(newMem())
	ctx := context.Background()
	acc, err := svc.EnsureFromGoogle(ctx, "sub-3")
	require.NoError(t, err)
	acc, err = svc.UpdateProfile(ctx, acc.ID, "Ana", "1199")
	require.NoError(t, err)
	dead, err := svc.Deactivate(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, account.StatusDeactivated, dead.Status)
	require.ErrorIs(t, dead.CanReserve(), account.ErrDeactivated)
	again, err := svc.EnsureFromGoogle(ctx, "sub-3")
	require.NoError(t, err)
	assert.Equal(t, account.StatusDeactivated, again.Status)
	assert.Equal(t, acc.ID, again.ID)
	_, err = svc.UpdateProfile(ctx, acc.ID, "Bia", "1188")
	require.ErrorIs(t, err, account.ErrDeactivated)
}

func TestDeactivate_UnknownID(t *testing.T) {
	t.Parallel()
	_, err := account.NewService(newMem()).Deactivate(context.Background(), "missing")
	require.ErrorIs(t, err, account.ErrNotFound)
}
