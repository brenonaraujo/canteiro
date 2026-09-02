package rentalpg

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

// Repo is the Postgres-backed rental repository.
type Repo struct {
	DB *gorm.DB
}

// New returns the rental repository.
func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

type rentalRow struct {
	ID                    string     `gorm:"column:id;primaryKey"`
	ListingID             string     `gorm:"column:listing_id"`
	TenantAccountID       string     `gorm:"column:tenant_account_id"`
	ListingSnapshot       []byte     `gorm:"column:listing_snapshot"`
	RentCents             int64      `gorm:"column:rent_cents"`
	OperatorCents         int64      `gorm:"column:operator_cents"`
	DepositCents          int64      `gorm:"column:deposit_cents"`
	CommissionCents       int64      `gorm:"column:commission_cents"`
	OwnerPayoutCents      int64      `gorm:"column:owner_payout_cents"`
	OperatorPayoutCents   int64      `gorm:"column:operator_payout_cents"`
	StartsAt              time.Time  `gorm:"column:starts_at"`
	EndsAt                time.Time  `gorm:"column:ends_at"`
	WithOperator          bool       `gorm:"column:with_operator"`
	OperatorTermsAccepted bool       `gorm:"column:operator_terms_accepted"`
	State                 string     `gorm:"column:state"`
	AcceptanceDeadlineAt  *time.Time `gorm:"column:acceptance_deadline_at"`
	ConfirmedAt           *time.Time `gorm:"column:confirmed_at"`
	DeclinedAt            *time.Time `gorm:"column:declined_at"`
	DeclineReason         string     `gorm:"column:decline_reason"`
	IntentKey             string     `gorm:"column:intent_key"`
	TenantClaimDebt       string     `gorm:"column:tenant_claim_debt"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (rentalRow) TableName() string { return "rentals" }

type paymentIntentRow struct {
	ID                 string    `gorm:"column:id;primaryKey"`
	RentalID           string    `gorm:"column:rental_id"`
	Provider           string    `gorm:"column:provider"`
	ProviderPaymentID  string    `gorm:"column:provider_payment_id"`
	IdempotencyKey     string    `gorm:"column:idempotency_key"`
	Attempt            int       `gorm:"column:attempt"`
	AmountCents        int64     `gorm:"column:amount_cents"`
	DepositCents       int64     `gorm:"column:deposit_cents"`
	ExpectedTotalCents int64     `gorm:"column:expected_total_cents"`
	Status             string    `gorm:"column:status"`
	FailureCode        string    `gorm:"column:failure_code"`
	FailureMessage     string    `gorm:"column:failure_message"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (paymentIntentRow) TableName() string { return "payment_intents" }

type webhookEventRow struct {
	ID              string     `gorm:"column:id;primaryKey"`
	Provider        string     `gorm:"column:provider"`
	ProviderEventID string     `gorm:"column:provider_event_id"`
	EventType       string     `gorm:"column:event_type"`
	RentalID        *string    `gorm:"column:rental_id"`
	PaymentIntentID *string    `gorm:"column:payment_intent_id"`
	Payload         []byte     `gorm:"column:payload"`
	SignatureValid  bool       `gorm:"column:signature_valid"`
	ReceivedAt      time.Time  `gorm:"column:received_at"`
	ProcessedAt     *time.Time `gorm:"column:processed_at"`
}

func (webhookEventRow) TableName() string { return "payment_webhook_events" }

type receiptRow struct {
	ID                  string    `gorm:"column:id;primaryKey"`
	RentalID            string    `gorm:"column:rental_id;unique"`
	TenantAccountID     string    `gorm:"column:tenant_account_id"`
	RentCents           int64     `gorm:"column:rent_cents"`
	OperatorCents       int64     `gorm:"column:operator_cents"`
	DepositCents        int64     `gorm:"column:deposit_cents"`
	TotalCents          int64     `gorm:"column:total_cents"`
	CommissionBaseCents int64     `gorm:"column:commission_base_cents"`
	CommissionCents     int64     `gorm:"column:commission_cents"`
	OwnerPayoutCents    int64     `gorm:"column:owner_payout_cents"`
	OperatorPayoutCents int64     `gorm:"column:operator_payout_cents"`
	ListingSnapshot     []byte    `gorm:"column:listing_snapshot"`
	WindowStartsAt      time.Time `gorm:"column:window_starts_at"`
	WindowEndsAt        time.Time `gorm:"column:window_ends_at"`
	IssuedAt            time.Time `gorm:"column:issued_at"`
}

func (receiptRow) TableName() string { return "rental_receipts" }

func toRental(r rentalRow, snap []byte) (rental.Rental, error) {
	out := rental.Rental{
		ID:                    r.ID,
		ListingID:             r.ListingID,
		TenantAccountID:       r.TenantAccountID,
		RentCents:             r.RentCents,
		OperatorCents:         r.OperatorCents,
		DepositCents:          r.DepositCents,
		CommissionCents:       r.CommissionCents,
		OwnerPayoutCents:      r.OwnerPayoutCents,
		OperatorPayoutCents:   r.OperatorPayoutCents,
		StartsAt:              r.StartsAt,
		EndsAt:                r.EndsAt,
		WithOperator:          r.WithOperator,
		OperatorTermsAccepted: r.OperatorTermsAccepted,
		State:                 rental.State(r.State),
		AcceptanceDeadlineAt:  r.AcceptanceDeadlineAt,
		ConfirmedAt:           r.ConfirmedAt,
		DeclinedAt:            r.DeclinedAt,
		DeclineReason:         r.DeclineReason,
		IntentKey:             r.IntentKey,
		TenantClaimDebt:       r.TenantClaimDebt,
		CreatedAt:             r.CreatedAt,
		UpdatedAt:             r.UpdatedAt,
	}
	s, err := rental.UnmarshalSnapshot(snap)
	if err != nil {
		return rental.Rental{}, err
	}
	out.ListingSnapshot = s
	return out, nil
}

func marshalSnapshot(s rental.ListingSnapshot) ([]byte, error) {
	return json.Marshal(s)
}

func unmarshalSnapshot(b []byte) (rental.ListingSnapshot, error) {
	return rental.UnmarshalSnapshot(b)
}

// CreateIntent inserts a `pending` rental. UNIQUE on intent_key makes duplicate calls idempotent.
func (r *Repo) CreateIntent(ctx context.Context, val rental.Rental, snapBytes []byte) (rental.Rental, error) {
	if snapBytes == nil {
		var err error
		snapBytes, err = marshalSnapshot(val.ListingSnapshot)
		if err != nil {
			return rental.Rental{}, err
		}
	}
	row := rentalRow{
		ID:                    val.ID,
		ListingID:             val.ListingID,
		TenantAccountID:       val.TenantAccountID,
		ListingSnapshot:       snapBytes,
		RentCents:             val.RentCents,
		OperatorCents:         val.OperatorCents,
		DepositCents:          val.DepositCents,
		CommissionCents:       val.CommissionCents,
		OwnerPayoutCents:      val.OwnerPayoutCents,
		OperatorPayoutCents:   val.OperatorPayoutCents,
		StartsAt:              val.StartsAt,
		EndsAt:                val.EndsAt,
		WithOperator:          val.WithOperator,
		OperatorTermsAccepted: val.OperatorTermsAccepted,
		State:                 string(val.State),
		AcceptanceDeadlineAt:  val.AcceptanceDeadlineAt,
		ConfirmedAt:           val.ConfirmedAt,
		DeclinedAt:            val.DeclinedAt,
		DeclineReason:         val.DeclineReason,
		IntentKey:             val.IntentKey,
		TenantClaimDebt:       val.TenantClaimDebt,
		CreatedAt:             val.CreatedAt,
		UpdatedAt:             val.UpdatedAt,
	}
	if err := r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row).Error; err != nil {
		return rental.Rental{}, err
	}
	loaded, _, err := r.GetByIntentKey(ctx, val.TenantAccountID, val.ListingID, val.IntentKey)
	return loaded, err
}

// GetByID loads a rental by id.
func (r *Repo) GetByID(ctx context.Context, id string) (rental.Rental, error) {
	var row rentalRow
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.Rental{}, rental.ErrNotFound
		}
		return rental.Rental{}, err
	}
	return toRental(row, row.ListingSnapshot)
}

// GetByIntentKey returns the rental matching the unique tuple.
func (r *Repo) GetByIntentKey(ctx context.Context, tenantID, listingID, key string) (rental.Rental, bool, error) {
	var row rentalRow
	err := r.DB.WithContext(ctx).
		Where("tenant_account_id = ? AND listing_id = ? AND intent_key = ?", tenantID, listingID, key).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.Rental{}, false, nil
		}
		return rental.Rental{}, false, err
	}
	out, err := toRental(row, row.ListingSnapshot)
	if err != nil {
		return rental.Rental{}, false, err
	}
	return out, true, nil
}

// ListForOwner returns rentals over the owner's listings.
func (r *Repo) ListForOwner(ctx context.Context, ownerID string, states []rental.State) ([]rental.Rental, error) {
	q := r.DB.WithContext(ctx).Model(&rentalRow{})
	if ownerID != "" {
		q = q.Where("listing_snapshot->>'owner_id' = ?", ownerID)
	}
	if len(states) > 0 {
		ss := make([]string, len(states))
		for i, s := range states {
			ss[i] = string(s)
		}
		q = q.Where("state IN ?", ss)
	}
	var rows []rentalRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]rental.Rental, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		val, err := toRental(row, row.ListingSnapshot)
		if err != nil {
			return nil, err
		}
		out = append(out, val)
	}
	return out, nil
}

// ListForTenant returns the tenant's rentals.
func (r *Repo) ListForTenant(ctx context.Context, tenantID string, states []rental.State) ([]rental.Rental, error) {
	q := r.DB.WithContext(ctx).Model(&rentalRow{}).Where("tenant_account_id = ?", tenantID)
	if len(states) > 0 {
		ss := make([]string, len(states))
		for i, s := range states {
			ss[i] = string(s)
		}
		q = q.Where("state IN ?", ss)
	}
	var rows []rentalRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]rental.Rental, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		val, err := toRental(row, row.ListingSnapshot)
		if err != nil {
			return nil, err
		}
		out = append(out, val)
	}
	return out, nil
}

// UpdateState runs the state transition.
func (r *Repo) UpdateState(ctx context.Context, id string, from, to rental.State, mutate func(r *rental.Rental)) (rental.Rental, error) {
	if !rental.CanTransition(from, to) {
		return rental.Rental{}, rental.ErrInvalidTransition
	}
	cur, err := r.GetByID(ctx, id)
	if err != nil {
		return rental.Rental{}, err
	}
	if cur.State != from {
		return rental.Rental{}, rental.ErrInvalidTransition
	}
	cur.State = to
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	res := r.DB.WithContext(ctx).Model(&rentalRow{}).
		Where("id = ? AND state = ?", id, string(from)).
		Updates(map[string]any{
			"state":                  string(to),
			"acceptance_deadline_at": cur.AcceptanceDeadlineAt,
			"confirmed_at":           cur.ConfirmedAt,
			"declined_at":            cur.DeclinedAt,
			"decline_reason":         cur.DeclineReason,
			"updated_at":             cur.UpdatedAt,
		})
	if res.Error != nil {
		return rental.Rental{}, res.Error
	}
	if res.RowsAffected == 0 {
		return rental.Rental{}, rental.ErrInvalidTransition
	}
	return cur, nil
}

// ListActiveOverlapping returns active rentals overlapping [start, end).
func (r *Repo) ListActiveOverlapping(ctx context.Context, listingID string, start, end time.Time) ([]rental.Rental, error) {
	var rows []rentalRow
	err := r.DB.WithContext(ctx).
		Where("listing_id = ? AND state IN ('authorized', 'confirmed')", listingID).
		Where("starts_at < ? AND ends_at > ?", end, start).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]rental.Rental, 0, len(rows))
	for _, row := range rows {
		val, err := toRental(row, row.ListingSnapshot)
		if err != nil {
			return nil, err
		}
		out = append(out, val)
	}
	return out, nil
}

// ListOwnerBlocks returns blocks overlapping [start, end).
func (r *Repo) ListOwnerBlocks(ctx context.Context, listingID string, start, end time.Time) ([]rentsvc.Block, error) {
	type blockRow struct {
		ID        string    `gorm:"column:id"`
		ListingID string    `gorm:"column:listing_id"`
		StartsAt  time.Time `gorm:"column:starts_at"`
		EndsAt    time.Time `gorm:"column:ends_at"`
	}
	var rows []blockRow
	err := r.DB.WithContext(ctx).
		Table("listing_blocks").
		Where("listing_id = ?", listingID).
		Where("starts_at < ? AND ends_at > ?", end, start).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]rentsvc.Block, len(rows))
	for i, row := range rows {
		out[i] = rentsvc.Block{
			ID:        row.ID,
			ListingID: row.ListingID,
			StartsAt:  row.StartsAt,
			EndsAt:    row.EndsAt,
		}
	}
	return out, nil
}

// SaveReceipt inserts the write-once receipt.
func (r *Repo) SaveReceipt(ctx context.Context, rec rental.Receipt) (rental.Receipt, error) {
	snapBytes, err := json.Marshal(rec.ListingSnapshot)
	if err != nil {
		return rental.Receipt{}, err
	}
	row := receiptRow{
		ID:                  uuid.NewString(),
		RentalID:            rec.RentalID,
		TenantAccountID:     rec.TenantAccountID,
		RentCents:           rec.RentCents,
		OperatorCents:       rec.OperatorCents,
		DepositCents:        rec.DepositCents,
		TotalCents:          rec.TotalCents,
		CommissionBaseCents: rec.CommissionBaseCents,
		CommissionCents:     rec.CommissionCents,
		OwnerPayoutCents:    rec.OwnerPayoutCents,
		OperatorPayoutCents: rec.OperatorPayoutCents,
		ListingSnapshot:     snapBytes,
		WindowStartsAt:      rec.WindowStartsAt,
		WindowEndsAt:        rec.WindowEndsAt,
		IssuedAt:            rec.IssuedAt,
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return rental.Receipt{}, err
	}
	if row.RentalID != rec.RentalID {
		var existing receiptRow
		if err := r.DB.WithContext(ctx).Where("rental_id = ?", rec.RentalID).First(&existing).Error; err != nil {
			return rental.Receipt{}, err
		}
		row = existing
	}
	return rental.Receipt{
		RentalID:            row.RentalID,
		TenantAccountID:     row.TenantAccountID,
		RentCents:           row.RentCents,
		OperatorCents:       row.OperatorCents,
		DepositCents:        row.DepositCents,
		TotalCents:          row.TotalCents,
		CommissionBaseCents: row.CommissionBaseCents,
		CommissionCents:     row.CommissionCents,
		OwnerPayoutCents:    row.OwnerPayoutCents,
		OperatorPayoutCents: row.OperatorPayoutCents,
		WindowStartsAt:      row.WindowStartsAt,
		WindowEndsAt:        row.WindowEndsAt,
		IssuedAt:            row.IssuedAt,
	}, nil
}

// GetReceipt returns the receipt for a rental.
func (r *Repo) GetReceipt(ctx context.Context, rentalID string) (rental.Receipt, bool, error) {
	var row receiptRow
	err := r.DB.WithContext(ctx).Where("rental_id = ?", rentalID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rental.Receipt{}, false, nil
		}
		return rental.Receipt{}, false, err
	}
	snap, err := unmarshalSnapshot(row.ListingSnapshot)
	if err != nil {
		return rental.Receipt{}, false, err
	}
	return rental.Receipt{
		RentalID:            row.RentalID,
		TenantAccountID:     row.TenantAccountID,
		RentCents:           row.RentCents,
		OperatorCents:       row.OperatorCents,
		DepositCents:        row.DepositCents,
		TotalCents:          row.TotalCents,
		CommissionBaseCents: row.CommissionBaseCents,
		CommissionCents:     row.CommissionCents,
		OwnerPayoutCents:    row.OwnerPayoutCents,
		OperatorPayoutCents: row.OperatorPayoutCents,
		ListingSnapshot:     snap,
		WindowStartsAt:      row.WindowStartsAt,
		WindowEndsAt:        row.WindowEndsAt,
		IssuedAt:            row.IssuedAt,
	}, true, nil
}

// UpsertPaymentIntent persists (or returns) the payment intent.
func (r *Repo) UpsertPaymentIntent(ctx context.Context, intent rentsvc.PaymentIntent) (rentsvc.PaymentIntent, error) {
	row := paymentIntentRow{
		ID:                 intent.ID,
		RentalID:           intent.RentalID,
		Provider:           intent.Provider,
		ProviderPaymentID:  intent.ProviderPaymentID,
		IdempotencyKey:     intent.IdempotencyKey,
		Attempt:            intent.Attempt,
		AmountCents:        intent.AmountCents,
		DepositCents:       intent.DepositCents,
		ExpectedTotalCents: intent.ExpectedTotalCents,
		Status:             intent.Status,
		FailureCode:        intent.FailureCode,
		FailureMessage:     intent.FailureMessage,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return rentsvc.PaymentIntent{}, err
	}
	if row.RentalID == "" {
		var existing paymentIntentRow
		if err := r.DB.WithContext(ctx).Where("idempotency_key = ?", intent.IdempotencyKey).First(&existing).Error; err != nil {
			return rentsvc.PaymentIntent{}, err
		}
		row = existing
	}
	return toPaymentIntent(row), nil
}

// GetPaymentIntent returns the most recent intent for a rental.
func (r *Repo) GetPaymentIntent(ctx context.Context, rentalID string) (rentsvc.PaymentIntent, bool, error) {
	var row paymentIntentRow
	err := r.DB.WithContext(ctx).Where("rental_id = ?", rentalID).
		Order("created_at DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rentsvc.PaymentIntent{}, false, nil
		}
		return rentsvc.PaymentIntent{}, false, err
	}
	return toPaymentIntent(row), true, nil
}

func toPaymentIntent(row paymentIntentRow) rentsvc.PaymentIntent {
	return rentsvc.PaymentIntent{
		ID:                 row.ID,
		RentalID:           row.RentalID,
		Provider:           row.Provider,
		ProviderPaymentID:  row.ProviderPaymentID,
		IdempotencyKey:     row.IdempotencyKey,
		Attempt:            row.Attempt,
		AmountCents:        row.AmountCents,
		DepositCents:       row.DepositCents,
		ExpectedTotalCents: row.ExpectedTotalCents,
		Status:             row.Status,
		FailureCode:        row.FailureCode,
		FailureMessage:     row.FailureMessage,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

// RecordWebhookEvent inserts the event. UNIQUE on (provider, provider_event_id) backs EC-2 idempotency.
func (r *Repo) RecordWebhookEvent(ctx context.Context, ev rentsvc.WebhookEvent) (rentsvc.WebhookEvent, error) {
	id := uuid.NewString()
	if ev.ID != "" {
		id = ev.ID
	}
	var rentalIDPtr *string
	if ev.RentalID != "" {
		v := ev.RentalID
		rentalIDPtr = &v
	}
	row := webhookEventRow{
		ID:              id,
		Provider:        ev.Provider,
		ProviderEventID: ev.ProviderEventID,
		EventType:       ev.EventType,
		RentalID:        rentalIDPtr,
		Payload:         ev.Payload,
		SignatureValid:  ev.SignatureValid,
		ReceivedAt:      ev.ReceivedAt,
	}
	err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	if err != nil {
		return rentsvc.WebhookEvent{}, err
	}
	return rentsvc.WebhookEvent{
		ID:              row.ID,
		Provider:        row.Provider,
		ProviderEventID: row.ProviderEventID,
		EventType:       row.EventType,
		RentalID:        derefStr(row.RentalID),
		ReceivedAt:      row.ReceivedAt,
		SignatureValid:  row.SignatureValid,
	}, nil
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
