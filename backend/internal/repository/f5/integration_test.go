//go:build integration

// Integration tests for the F5 repository layer — required because the
// gate for repository/f5 is the integration suite (ADR-0021). Run with:
//
//	DATABASE_URL=postgres://canteiro:canteiro@localhost:5432/canteiro?sslmode=disable \
//	  go test -tags=integration ./internal/repository/f5/...
//
// These tests assume migrations 000001..000007 are applied (the dev
// compose does it on `up`). The B1 regression lives here: the
// pickup_evidence / return_evidence / evidence columns are JSONB NOT NULL
// DEFAULT '{}'::jsonb — GORM sends nil for empty []byte and that violates
// the NOT NULL constraint. The defaultJSON helper in f5pg.go fills in
// "{}" before INSERT so the INSERT succeeds. Without the fix, the test
// below fails on the first INSERT.
package f5pg_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	f5pg "github.com/brenonaraujo/canteiro/backend/internal/repository/f5"
)

func openIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// seedRental inserts a minimal confirmed rental so the FK targets in
// devolucoes / avaria_pedidos / dividas are satisfied. Returns the
// rental id (and tenant account id). Best-effort cleanup on test exit.
func seedRental(t *testing.T, db *gorm.DB) (rentalID, tenantID, ownerID string) {
	t.Helper()
	tenantID = uuid.NewString()
	ownerID = uuid.NewString()
	listingID := uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO accounts (id, email, name, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?),
		       (?, ?, ?, ?, ?, ?, ?)`,
		tenantID, "tenant-"+tenantID+"@test", "T", "tenant", "active", now, now,
		ownerID, "owner-"+ownerID+"@test", "O", "owner", "active", now, now,
	).Error)
	require.NoError(t, db.Exec(`INSERT INTO listings (id, owner_id, title, category, price_unit, price_amount_cents, deposit_cents, status, created_at, updated_at)
		VALUES (?, ?, 'Furadeira', 'electric', 'hour', 5000, 10000, 'published', ?, ?)`,
		listingID, ownerID, now, now,
	).Error)
	rentalID = uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO rentals (id, listing_id, tenant_account_id, owner_id, starts_at, ends_at, state, rent_cents, deposit_cents, intent_key, snapshot, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'confirmed', 5000, 10000, ?, '{}'::jsonb, ?, ?)`,
		rentalID, listingID, tenantID, ownerID,
		now.Add(-2*time.Hour), now.Add(time.Hour),
		"int-"+uuid.NewString(), now, now,
	).Error)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM rentals WHERE id = ?", rentalID).Error
		_ = db.Exec("DELETE FROM listings WHERE id = ?", listingID).Error
		_ = db.Exec("DELETE FROM accounts WHERE id IN (?, ?)", tenantID, ownerID).Error
	})
	return rentalID, tenantID, ownerID
}

// TestIntegration_ReturnRepo_NilDefaultDoesNotViolateNotNull is the B1
// RED-first regression. The QA repro showed that calling Create with
// ReturnEvidence=nil produced
//   ERROR: null value in column "return_evidence" of relation
//   "devolucoes" violates not-null constraint
// This test reproduces that: we pass an empty Return (no evidence set)
// and assert the INSERT succeeds and the row round-trips.
func TestIntegration_ReturnRepo_NilDefaultDoesNotViolateNotNull(t *testing.T) {
	db := openIntegrationDB(t)
	ctx := context.Background()
	repo := f5pg.NewReturnRepo(db)
	rentalID, _, _ := seedRental(t, db)

	id := uuid.NewString()
	created, err := repo.Create(ctx, rental.Return{
		ID:       id,
		RentalID: rentalID,
		State:    rental.ReturnInProgress,
		// PickupEvidence / ReturnEvidence both nil — pre-fix this 500s.
	})
	require.NoError(t, err)
	require.Equal(t, id, created.ID)

	got, ok, err := repo.GetByRental(ctx, rentalID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, id, got.ID)
	// defaultJSON keeps the column non-null even when caller passed nothing.
	require.NotNil(t, got.PickupEvidence)
	require.NotNil(t, got.ReturnEvidence)
	require.Equal(t, []byte("{}"), got.PickupEvidence)
	require.Equal(t, []byte("{}"), got.ReturnEvidence)

	t.Cleanup(func() { _ = db.Exec("DELETE FROM devolucoes WHERE id = ?", id).Error() })
}

// TestIntegration_DamageRepo_NilEvidenceDoesNotViolateNotNull is the
// "bug latente irmão" called out in the QA report: same class as B1
// for the avaria_pedidos.evidence column.
func TestIntegration_DamageRepo_NilEvidenceDoesNotViolateNotNull(t *testing.T) {
	db := openIntegrationDB(t)
	ctx := context.Background()
	repo := f5pg.NewDamageRepo(db)
	rentalID, tenantID, ownerID := seedRental(t, db)

	id := uuid.NewString()
	created, err := repo.Create(ctx, rental.DamageClaim{
		ID:           id,
		RentalID:     rentalID,
		OwnerID:      ownerID,
		RenterID:     tenantID,
		State:        rental.DamageOpen,
		Nature:       rental.DamageCosmetic,
		Description:  "scratch",
		ProposedCents: 1000,
		// Evidence nil — pre-fix this 500s on NOT NULL evidence.
	})
	require.NoError(t, err)
	require.Equal(t, id, created.ID)

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, []byte("{}"), got.Evidence)

	t.Cleanup(func() { _ = db.Exec("DELETE FROM avaria_pedidos WHERE id = ?", id).Error() })
}