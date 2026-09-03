// Package f5pg implements the F5 repository contracts (ReturnRepository,
// DamageRepository, DebtRepository) on top of PostgreSQL via GORM. The
// schema lives in migrations/000007_devolucao.{up,down}.sql; the row types
// here mirror that schema 1:1.
//
// The three repository types are kept separate because the underlying
// interfaces share method names (Create, GetByID, UpdateState) — bundling
// them on a single receiver would force awkward disambiguation. Each type
// is a thin wrapper around the same *gorm.DB; the caller picks the right
// one based on the domain object being persisted.
package f5pg

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// DB is the embedded Postgres handle shared by the three repositories.
type DB = gorm.DB

// --- row types -----------------------------------------------------------

type returnRow struct {
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	ReturnedAt           *time.Time `gorm:"column:returned_at"`
	ID                   string     `gorm:"column:id;primaryKey"`
	RentalID             string     `gorm:"column:rental_id;unique"`
	State                string     `gorm:"column:state"`
	PickupEvidence       []byte     `gorm:"column:pickup_evidence"`
	ReturnEvidence       []byte     `gorm:"column:return_evidence"`
	DepositReleasedCents int64      `gorm:"column:deposit_released_cents"`
	DepositCapturedCents int64      `gorm:"column:deposit_captured_cents"`
}

func (returnRow) TableName() string { return "devolucoes" }

// table; alignment is micro-optimization.
//
//nolint:govet // fieldalignment: 18-field row mirror of the damage_pedidos
type damageRow struct {
	OpenedAt           time.Time  `gorm:"column:opened_at"`
	DecidedAt          *time.Time `gorm:"column:decided_at"`
	ResolvedAt         *time.Time `gorm:"column:resolved_at"`
	RespondedAt        *time.Time `gorm:"column:responded_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	ID                 string     `gorm:"column:id;primaryKey"`
	RentalID           string     `gorm:"column:rental_id;unique"`
	State              string     `gorm:"column:state"`
	Nature             string     `gorm:"column:nature"`
	OwnerID            string     `gorm:"column:owner_id"`
	RenterID           string     `gorm:"column:renter_id"`
	Description        string     `gorm:"column:description"`
	Evidence           []byte     `gorm:"column:evidence"`
	ProposedCents      int64      `gorm:"column:proposed_cents"`
	AgreedCents        int64      `gorm:"column:agreed_cents"`
	RenterResponseKind string     `gorm:"column:renter_response_kind"`
	RenterResponseNote string     `gorm:"column:renter_response_note"`
	StaffDecisionNote  string     `gorm:"column:staff_decision_note"`
}

func (damageRow) TableName() string { return "avaria_pedidos" }

// table; alignment is micro-optimization.
//
//nolint:govet // fieldalignment: 13-field row mirror of the dividas
type debtRow struct {
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	DueAt          time.Time  `gorm:"column:due_at"`
	SettledAt      *time.Time `gorm:"column:settled_at"`
	ForgivenAt     *time.Time `gorm:"column:forgiven_at"`
	ID             string     `gorm:"column:id;primaryKey"`
	RentalID       string     `gorm:"column:rental_id;unique"`
	DamageID       string     `gorm:"column:damage_id;unique"`
	RenterID       string     `gorm:"column:renter_id"`
	State          string     `gorm:"column:state"`
	OriginalCents  int64      `gorm:"column:original_cents"`
	ForgivenCents  int64      `gorm:"column:forgiven_cents"`
	SettledCents   int64      `gorm:"column:settled_cents"`
	ForgivenReason string     `gorm:"column:forgiven_reason"`
}

func (debtRow) TableName() string { return "dividas" }

// --- conversions ---------------------------------------------------------

func toReturn(r returnRow) rental.Return {
	return rental.Return{
		ID:                   r.ID,
		RentalID:             r.RentalID,
		State:                rental.ReturnState(r.State),
		PickupEvidence:       r.PickupEvidence,
		ReturnEvidence:       r.ReturnEvidence,
		DepositReleasedCents: r.DepositReleasedCents,
		DepositCapturedCents: r.DepositCapturedCents,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
		ReturnedAt:           r.ReturnedAt,
	}
}

func toDamage(r damageRow) rental.DamageClaim {
	return rental.DamageClaim{
		ID:                 r.ID,
		RentalID:           r.RentalID,
		State:              rental.DamageState(r.State),
		Nature:             rental.DamageNature(r.Nature),
		OwnerID:            r.OwnerID,
		RenterID:           r.RenterID,
		Description:        r.Description,
		Evidence:           r.Evidence,
		ProposedCents:      r.ProposedCents,
		AgreedCents:        r.AgreedCents,
		RenterResponseKind: r.RenterResponseKind,
		RenterResponseNote: r.RenterResponseNote,
		StaffDecisionNote:  r.StaffDecisionNote,
		OpenedAt:           r.OpenedAt,
		RespondedAt:        r.RespondedAt,
		DecidedAt:          r.DecidedAt,
		ResolvedAt:         r.ResolvedAt,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

func toDebt(r debtRow) rental.Debt {
	return rental.Debt{
		ID:             r.ID,
		RentalID:       r.RentalID,
		DamageID:       r.DamageID,
		RenterID:       r.RenterID,
		State:          rental.DebtState(r.State),
		OriginalCents:  r.OriginalCents,
		ForgivenCents:  r.ForgivenCents,
		SettledCents:   r.SettledCents,
		ForgivenReason: r.ForgivenReason,
		DueAt:          r.DueAt,
		SettledAt:      r.SettledAt,
		ForgivenAt:     r.ForgivenAt,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// --- return --------------------------------------------------------------

// ReturnRepo implements the f5.ReturnRepository on top of Postgres.
type ReturnRepo struct{ DB *gorm.DB }

// NewReturnRepo returns a return repository.
func NewReturnRepo(db *gorm.DB) *ReturnRepo { return &ReturnRepo{DB: db} }

// Create persists a new return row.
func (r *ReturnRepo) Create(ctx context.Context, ret rental.Return) (rental.Return, error) {
	if ret.ID == "" {
		ret.ID = uuid.NewString()
	}
	row := returnRow{
		ID:                   ret.ID,
		RentalID:             ret.RentalID,
		State:                string(ret.State),
		PickupEvidence:       defaultJSON(ret.PickupEvidence),
		ReturnEvidence:       defaultJSON(ret.ReturnEvidence),
		DepositReleasedCents: ret.DepositReleasedCents,
		DepositCapturedCents: ret.DepositCapturedCents,
		ReturnedAt:           ret.ReturnedAt,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return rental.Return{}, err
	}
	loaded, _, err := r.GetByRental(ctx, ret.RentalID)
	if err != nil {
		return rental.Return{}, err
	}
	return loaded, nil
}

// GetByRental loads the return for a rental.
func (r *ReturnRepo) GetByRental(ctx context.Context, rentalID string) (rental.Return, bool, error) {
	var row returnRow
	err := r.DB.WithContext(ctx).Where("rental_id = ?", rentalID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.Return{}, false, nil
		}
		return rental.Return{}, false, err
	}
	return toReturn(row), true, nil
}

// UpdateState runs a state transition.
func (r *ReturnRepo) UpdateState(ctx context.Context, id string, from, to rental.ReturnState, mutate func(ret *rental.Return)) (rental.Return, error) {
	if !rental.CanReturnTransition(from, to) {
		return rental.Return{}, rental.ErrF5ReturnInvalidState
	}
	cur, err := r.byID(ctx, id)
	if err != nil {
		return rental.Return{}, err
	}
	if cur.State != from {
		return rental.Return{}, rental.ErrF5ReturnInvalidState
	}
	cur.State = to
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	res := r.DB.WithContext(ctx).Model(&returnRow{}).
		Where("id = ? AND state = ?", id, string(from)).
		Updates(map[string]any{
			"state":                  string(to),
			"return_evidence":        cur.ReturnEvidence,
			"deposit_released_cents": cur.DepositReleasedCents,
			"deposit_captured_cents": cur.DepositCapturedCents,
			"returned_at":            cur.ReturnedAt,
			"updated_at":             cur.UpdatedAt,
		})
	if res.Error != nil {
		return rental.Return{}, res.Error
	}
	if res.RowsAffected == 0 {
		return rental.Return{}, rental.ErrF5ReturnInvalidState
	}
	return cur, nil
}

func (r *ReturnRepo) byID(ctx context.Context, id string) (rental.Return, error) {
	var row returnRow
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.Return{}, rental.ErrF5ReturnNotFound
		}
		return rental.Return{}, err
	}
	return toReturn(row), nil
}

// Mutate runs a non-state-changing update on the return row. Used by
// the Pilar 3 wire: capturing the deposit + releasing the balance and
// setting returned_at happen outside the state machine. Mirrors
// DebtRepo.Mutate. The mutate callback receives the loaded row and may
// set DepositCapturedCents / DepositReleasedCents / ReturnedAt freely.
// The repository bumps updated_at and writes all three columns (NULL
// values are passed through; non-null preservation is the caller's job).
func (r *ReturnRepo) Mutate(ctx context.Context, id string, mutate func(ret *rental.Return)) (rental.Return, error) {
	cur, err := r.byID(ctx, id)
	if err != nil {
		return rental.Return{}, err
	}
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	res := r.DB.WithContext(ctx).Model(&returnRow{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"deposit_captured_cents": cur.DepositCapturedCents,
			"deposit_released_cents": cur.DepositReleasedCents,
			"returned_at":            cur.ReturnedAt,
			"updated_at":             cur.UpdatedAt,
		})
	if res.Error != nil {
		return rental.Return{}, res.Error
	}
	if res.RowsAffected == 0 {
		return rental.Return{}, rental.ErrF5ReturnNotFound
	}
	return cur, nil
}

// --- damage --------------------------------------------------------------

// DamageRepo implements the f5.DamageRepository on top of Postgres.
type DamageRepo struct{ DB *gorm.DB }

// NewDamageRepo returns a damage repository.
func NewDamageRepo(db *gorm.DB) *DamageRepo { return &DamageRepo{DB: db} }

// Create persists a new damage claim.
func (r *DamageRepo) Create(ctx context.Context, c rental.DamageClaim) (rental.DamageClaim, error) {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	row := damageRow{
		ID:                 c.ID,
		RentalID:           c.RentalID,
		State:              string(c.State),
		Nature:             string(c.Nature),
		OwnerID:            c.OwnerID,
		RenterID:           c.RenterID,
		Description:        c.Description,
		Evidence:           defaultJSON(c.Evidence),
		ProposedCents:      c.ProposedCents,
		AgreedCents:        c.AgreedCents,
		RenterResponseKind: c.RenterResponseKind,
		RenterResponseNote: c.RenterResponseNote,
		StaffDecisionNote:  c.StaffDecisionNote,
		OpenedAt:           c.OpenedAt,
		RespondedAt:        c.RespondedAt,
		DecidedAt:          c.DecidedAt,
		ResolvedAt:         c.ResolvedAt,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if row.OpenedAt.IsZero() {
		row.OpenedAt = row.CreatedAt
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return rental.DamageClaim{}, err
	}
	return r.byID(ctx, row.ID)
}

// GetByID loads a damage claim by id.
func (r *DamageRepo) GetByID(ctx context.Context, id string) (rental.DamageClaim, error) {
	return r.byID(ctx, id)
}

// GetByRental loads the damage claim for a rental (1 per rental, v1).
func (r *DamageRepo) GetByRental(ctx context.Context, rentalID string) (rental.DamageClaim, bool, error) {
	var row damageRow
	err := r.DB.WithContext(ctx).Where("rental_id = ?", rentalID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.DamageClaim{}, false, nil
		}
		return rental.DamageClaim{}, false, err
	}
	return toDamage(row), true, nil
}

// UpdateState runs a damage state transition.
func (r *DamageRepo) UpdateState(ctx context.Context, id string, from, to rental.DamageState, mutate func(c *rental.DamageClaim)) (rental.DamageClaim, error) {
	if !rental.CanDamageTransition(from, to) {
		return rental.DamageClaim{}, rental.ErrF5DamageInvalidState
	}
	cur, err := r.byID(ctx, id)
	if err != nil {
		return rental.DamageClaim{}, err
	}
	if cur.State != from {
		return rental.DamageClaim{}, rental.ErrF5DamageInvalidState
	}
	cur.State = to
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	res := r.DB.WithContext(ctx).Model(&damageRow{}).
		Where("id = ? AND state = ?", id, string(from)).
		Updates(map[string]any{
			"state":                string(to),
			"responded_at":         cur.RespondedAt,
			"decided_at":           cur.DecidedAt,
			"resolved_at":          cur.ResolvedAt,
			"renter_response_kind": cur.RenterResponseKind,
			"renter_response_note": cur.RenterResponseNote,
			"staff_decision_note":  cur.StaffDecisionNote,
			"agreed_cents":         cur.AgreedCents,
			"updated_at":           cur.UpdatedAt,
		})
	if res.Error != nil {
		return rental.DamageClaim{}, res.Error
	}
	if res.RowsAffected == 0 {
		return rental.DamageClaim{}, rental.ErrF5DamageInvalidState
	}
	return cur, nil
}

// ListExpiring returns claims in Open or Contested with opened_at < before.
func (r *DamageRepo) ListExpiring(ctx context.Context, before time.Time) ([]rental.DamageClaim, error) {
	var rows []damageRow
	if err := r.DB.WithContext(ctx).
		Where("state IN ?", []string{string(rental.DamageOpen), string(rental.DamageContested)}).
		Where("opened_at < ?", before).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]rental.DamageClaim, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDamage(row))
	}
	return out, nil
}

func (r *DamageRepo) byID(ctx context.Context, id string) (rental.DamageClaim, error) {
	var row damageRow
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.DamageClaim{}, rental.ErrF5DamageNotFound
		}
		return rental.DamageClaim{}, err
	}
	return toDamage(row), nil
}

// --- debt ----------------------------------------------------------------

// DebtRepo implements the f5.DebtRepository on top of Postgres.
type DebtRepo struct{ DB *gorm.DB }

// NewDebtRepo returns a debt repository.
func NewDebtRepo(db *gorm.DB) *DebtRepo { return &DebtRepo{DB: db} }

// Create persists a new debt.
func (r *DebtRepo) Create(ctx context.Context, d rental.Debt) (rental.Debt, error) {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	row := debtRow{
		ID:             d.ID,
		RentalID:       d.RentalID,
		DamageID:       d.DamageID,
		RenterID:       d.RenterID,
		State:          string(d.State),
		OriginalCents:  d.OriginalCents,
		ForgivenCents:  d.ForgivenCents,
		SettledCents:   d.SettledCents,
		ForgivenReason: d.ForgivenReason,
		DueAt:          d.DueAt,
		SettledAt:      d.SettledAt,
		ForgivenAt:     d.ForgivenAt,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return rental.Debt{}, err
	}
	return r.byID(ctx, row.ID)
}

// GetByID loads a debt by id.
func (r *DebtRepo) GetByID(ctx context.Context, id string) (rental.Debt, error) {
	return r.byID(ctx, id)
}

// GetByDamage loads the debt associated with a damage claim (1 per claim).
func (r *DebtRepo) GetByDamage(ctx context.Context, damageID string) (rental.Debt, bool, error) {
	var row debtRow
	err := r.DB.WithContext(ctx).Where("damage_id = ?", damageID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.Debt{}, false, nil
		}
		return rental.Debt{}, false, err
	}
	return toDebt(row), true, nil
}

// UpdateState runs a debt state transition.
func (r *DebtRepo) UpdateState(ctx context.Context, id string, from, to rental.DebtState, mutate func(d *rental.Debt)) (rental.Debt, error) {
	if !rental.CanDebtTransition(from, to) {
		return rental.Debt{}, rental.ErrF5DebtInvalidState
	}
	cur, err := r.byID(ctx, id)
	if err != nil {
		return rental.Debt{}, err
	}
	if cur.State != from {
		return rental.Debt{}, rental.ErrF5DebtInvalidState
	}
	cur.State = to
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	res := r.DB.WithContext(ctx).Model(&debtRow{}).
		Where("id = ? AND state = ?", id, string(from)).
		Updates(map[string]any{
			"state":           string(to),
			"settled_cents":   cur.SettledCents,
			"settled_at":      cur.SettledAt,
			"forgiven_cents":  cur.ForgivenCents,
			"forgiven_reason": cur.ForgivenReason,
			"forgiven_at":     cur.ForgivenAt,
			"updated_at":      cur.UpdatedAt,
		})
	if res.Error != nil {
		return rental.Debt{}, res.Error
	}
	if res.RowsAffected == 0 {
		return rental.Debt{}, rental.ErrF5DebtInvalidState
	}
	return cur, nil
}

// Mutate runs a state-less update (used for partial forgiveness).
func (r *DebtRepo) Mutate(ctx context.Context, id string, mutate func(d *rental.Debt)) (rental.Debt, error) {
	cur, err := r.byID(ctx, id)
	if err != nil {
		return rental.Debt{}, err
	}
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	res := r.DB.WithContext(ctx).Model(&debtRow{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"forgiven_cents":  cur.ForgivenCents,
			"forgiven_reason": cur.ForgivenReason,
			"updated_at":      cur.UpdatedAt,
		})
	if res.Error != nil {
		return rental.Debt{}, res.Error
	}
	return cur, nil
}

// ListOpenForRenter returns the renter's open debts.
func (r *DebtRepo) ListOpenForRenter(ctx context.Context, renterID string) ([]rental.Debt, error) {
	var rows []debtRow
	if err := r.DB.WithContext(ctx).Where("renter_id = ? AND state = ?", renterID, string(rental.DebtOpen)).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]rental.Debt, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDebt(row))
	}
	return out, nil
}

// ListDueBy returns open debts past their due_at.
func (r *DebtRepo) ListDueBy(ctx context.Context, before time.Time) ([]rental.Debt, error) {
	var rows []debtRow
	if err := r.DB.WithContext(ctx).Where("state = ? AND due_at <= ?", string(rental.DebtOpen), before).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]rental.Debt, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDebt(row))
	}
	return out, nil
}

func (r *DebtRepo) byID(ctx context.Context, id string) (rental.Debt, error) {
	var row debtRow
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.Debt{}, rental.ErrF5DebtNotFound
		}
		return rental.Debt{}, err
	}
	return toDebt(row), nil
}

// defaultJSON mirrors the schema's `DEFAULT '{}'::jsonb` for the evidence
// JSONB columns. GORM sends nil as SQL NULL for `[]byte` and that violates
// the NOT NULL constraint even when the schema would have applied the
// default — so the repository fills in "{}" before INSERT. Mirrors the
// JSON shape the service marshals (EvidencePayload -> {"photos":..,
// "description":.., "checklist":..}). Used by ReturnRepo.Create,
// DamageRepo.Create (and any future Create path touching a JSONB NOT NULL
// column without DB-side DEFAULT).
func defaultJSON(in []byte) []byte {
	if len(in) == 0 {
		return []byte("{}")
	}
	return in
}
