// Package listingpg is the Postgres implementation of the listing
// domain Repository. It uses GORM (consistent with the rest of the
// backend) and exposes the interface the listing.Service depends on.
package listingpg

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
)

// Repo persists listings, photos, blocks and owner onboarding.
type Repo struct {
	DB *gorm.DB
}

// New returns the listing repository.
func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

// --- row mappings ---------------------------------------------------------

type listingRow struct {
	ID                    string    `gorm:"column:id;primaryKey"`
	OwnerAccountID        string    `gorm:"column:owner_account_id"`
	State                 string    `gorm:"column:state"`
	Title                 string    `gorm:"column:title"`
	Description           string    `gorm:"column:description"`
	Category              string    `gorm:"column:category"`
	PickupCity            string    `gorm:"column:pickup_city"`
	PickupNeighborhood    string    `gorm:"column:pickup_neighborhood"`
	DeliveryEnabled       bool      `gorm:"column:delivery_enabled"`
	DeliveryCoverage      string    `gorm:"column:delivery_coverage"`
	PriceUnit             string    `gorm:"column:price_unit"`
	PriceAmountCents      int64     `gorm:"column:price_amount_cents"`
	DepositCents          int64     `gorm:"column:deposit_cents"`
	MinLeadTimeHours      int       `gorm:"column:min_lead_time_hours"`
	OperatorMode          string    `gorm:"column:operator_mode"`
	OperatorHourlyRate    int64     `gorm:"column:operator_hourly_rate_cents"`
	OperatorMinHours      int       `gorm:"column:operator_min_hours"`
	OperatorName          string    `gorm:"column:operator_name"`
	OperatorPhone         string    `gorm:"column:operator_phone"`
	OperatorIsOwner       bool      `gorm:"column:operator_is_owner"`
	RuleDocumentRequired  bool      `gorm:"column:rule_document_required"`
	RuleMinAge            int       `gorm:"column:rule_min_age"`
	RuleExperienceReq     bool      `gorm:"column:rule_experience_required"`
	RuleTravelRestricted  bool      `gorm:"column:rule_travel_restricted"`
	HeavyLegalCession     bool      `gorm:"column:heavy_legal_cession"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (listingRow) TableName() string { return "listings" }

type photoRow struct {
	ListingID string `gorm:"column:listing_id;primaryKey"`
	Position  int    `gorm:"column:position;primaryKey"`
	URL       string `gorm:"column:url"`
}

func (photoRow) TableName() string { return "listing_photos" }

type blockRow struct {
	ID        string    `gorm:"column:id;primaryKey"`
	ListingID string    `gorm:"column:listing_id"`
	StartsAt  time.Time `gorm:"column:starts_at"`
	EndsAt    time.Time `gorm:"column:ends_at"`
	Reason    string    `gorm:"column:reason"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (blockRow) TableName() string { return "listing_blocks" }

type ownerOnboardingRow struct {
	AccountID      string     `gorm:"column:account_id;primaryKey"`
	PayoutKind     string     `gorm:"column:payout_kind"`
	PayoutLast4    string     `gorm:"column:payout_last4"`
	TermsAcceptedAt *time.Time `gorm:"column:terms_accepted_at"`
	TermsVersion   string     `gorm:"column:terms_version"`
}

func (ownerOnboardingRow) TableName() string { return "owner_onboarding" }

type categoryRow struct {
	Category       string `gorm:"column:category;primaryKey"`
	Size           string `gorm:"column:size"`
	DepositMinCents int64 `gorm:"column:deposit_min_cents"`
}

func (categoryRow) TableName() string { return "listing_categories" }

// --- row → domain ----------------------------------------------------------

func toListing(r listingRow, photos []string) listing.Listing {
	return listing.Listing{
		ID:                 r.ID,
		OwnerAccountID:     r.OwnerAccountID,
		State:              listing.State(r.State),
		Title:              r.Title,
		Description:        r.Description,
		Category:           listing.Category(r.Category),
		PickupCity:         r.PickupCity,
		PickupNeighborhood: r.PickupNeighborhood,
		Delivery: listing.Delivery{
			Enabled:  r.DeliveryEnabled,
			Coverage: r.DeliveryCoverage,
		},
		PriceUnit:         listing.PriceUnit(r.PriceUnit),
		PriceAmountCents:  r.PriceAmountCents,
		DepositCents:      r.DepositCents,
		MinLeadTimeHours:  r.MinLeadTimeHours,
		Photos:            photos,
		Rules: listing.Rules{
			DocumentRequired:   r.RuleDocumentRequired,
			MinAge:             r.RuleMinAge,
			ExperienceRequired: r.RuleExperienceReq,
			TravelRestricted:   r.RuleTravelRestricted,
		},
		Operator: listing.Operator{
			Mode:            listing.OperatorMode(r.OperatorMode),
			HourlyRateCents: r.OperatorHourlyRate,
			MinHours:        r.OperatorMinHours,
			Identity: listing.OperatorIdentity{
				Name:    r.OperatorName,
				Phone:   r.OperatorPhone,
				IsOwner: r.OperatorIsOwner,
			},
		},
		HeavyLegalCession: r.HeavyLegalCession,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

func toRow(l listing.Listing) listingRow {
	return listingRow{
		ID:                    l.ID,
		OwnerAccountID:        l.OwnerAccountID,
		State:                 string(l.State),
		Title:                 l.Title,
		Description:           l.Description,
		Category:              string(l.Category),
		PickupCity:            l.PickupCity,
		PickupNeighborhood:    l.PickupNeighborhood,
		DeliveryEnabled:       l.Delivery.Enabled,
		DeliveryCoverage:      l.Delivery.Coverage,
		PriceUnit:             string(l.PriceUnit),
		PriceAmountCents:      l.PriceAmountCents,
		DepositCents:          l.DepositCents,
		MinLeadTimeHours:      l.MinLeadTimeHours,
		OperatorMode:          string(l.Operator.Mode),
		OperatorHourlyRate:    l.Operator.HourlyRateCents,
		OperatorMinHours:      l.Operator.MinHours,
		OperatorName:          l.Operator.Identity.Name,
		OperatorPhone:         l.Operator.Identity.Phone,
		OperatorIsOwner:       l.Operator.Identity.IsOwner,
		RuleDocumentRequired:  l.Rules.DocumentRequired,
		RuleMinAge:            l.Rules.MinAge,
		RuleExperienceReq:     l.Rules.ExperienceRequired,
		RuleTravelRestricted:  l.Rules.TravelRestricted,
		HeavyLegalCession:     l.HeavyLegalCession,
		CreatedAt:             l.CreatedAt,
		UpdatedAt:             l.UpdatedAt,
	}
}

func mapListing(row listingRow, photos []string, err error) (listing.Listing, error) {
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return listing.Listing{}, listing.ErrNotFound
		}
		return listing.Listing{}, err
	}
	return toListing(row, photos), nil
}

func (r *Repo) loadPhotos(ctx context.Context, listingID string) ([]string, error) {
	var rows []photoRow
	if err := r.DB.WithContext(ctx).
		Where("listing_id = ?", listingID).
		Order("position ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, p := range rows {
		out[i] = p.URL
	}
	return out, nil
}

// --- listing CRUD ---------------------------------------------------------

// Create inserts a listing and its photos in a single transaction.
func (r *Repo) Create(ctx context.Context, l listing.Listing) (listing.Listing, error) {
	row := toRow(l)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return r.replacePhotosTx(tx, row.ID, l.Photos)
	})
	if err != nil {
		return listing.Listing{}, err
	}
	stored, err := r.GetByID(ctx, row.ID)
	return stored, err
}

// Update replaces the mutable fields of an existing listing.
func (r *Repo) Update(ctx context.Context, l listing.Listing) (listing.Listing, error) {
	row := toRow(l)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&listingRow{}).Where("id = ?", row.ID).Updates(map[string]any{
			"title":                      row.Title,
			"description":                row.Description,
			"category":                   row.Category,
			"pickup_city":                row.PickupCity,
			"pickup_neighborhood":        row.PickupNeighborhood,
			"delivery_enabled":           row.DeliveryEnabled,
			"delivery_coverage":          row.DeliveryCoverage,
			"price_unit":                 row.PriceUnit,
			"price_amount_cents":         row.PriceAmountCents,
			"deposit_cents":              row.DepositCents,
			"min_lead_time_hours":        row.MinLeadTimeHours,
			"operator_mode":              row.OperatorMode,
			"operator_hourly_rate_cents": row.OperatorHourlyRate,
			"operator_min_hours":         row.OperatorMinHours,
			"operator_name":              row.OperatorName,
			"operator_phone":             row.OperatorPhone,
			"operator_is_owner":          row.OperatorIsOwner,
			"rule_document_required":     row.RuleDocumentRequired,
			"rule_min_age":               row.RuleMinAge,
			"rule_experience_required":   row.RuleExperienceReq,
			"rule_travel_restricted":     row.RuleTravelRestricted,
			"heavy_legal_cession":        row.HeavyLegalCession,
			"updated_at":                 row.UpdatedAt,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return listing.ErrNotFound
		}
		return r.replacePhotosTx(tx, row.ID, l.Photos)
	})
	if err != nil {
		return listing.Listing{}, err
	}
	return r.GetByID(ctx, row.ID)
}

func (r *Repo) replacePhotosTx(tx *gorm.DB, listingID string, photos []string) error {
	if err := tx.Where("listing_id = ?", listingID).Delete(&photoRow{}).Error; err != nil {
		return err
	}
	if len(photos) == 0 {
		return nil
	}
	rows := make([]photoRow, len(photos))
	for i, u := range photos {
		rows[i] = photoRow{ListingID: listingID, Position: i, URL: u}
	}
	return tx.Create(&rows).Error
}

// GetByID returns the listing including photos.
func (r *Repo) GetByID(ctx context.Context, id string) (listing.Listing, error) {
	var row listingRow
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&row).Error
	photos, perr := r.loadPhotos(ctx, id)
	if perr != nil {
		return listing.Listing{}, perr
	}
	return mapListing(row, photos, err)
}

// ListByOwner returns every listing owned by the caller.
func (r *Repo) ListByOwner(ctx context.Context, ownerID string) ([]listing.Listing, error) {
	var rows []listingRow
	if err := r.DB.WithContext(ctx).
		Where("owner_account_id = ?", ownerID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]listing.Listing, 0, len(rows))
	for _, row := range rows {
		photos, err := r.loadPhotos(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toListing(row, photos))
	}
	return out, nil
}

// GetPublic returns the listing only when it's published.
func (r *Repo) GetPublic(ctx context.Context, id string) (listing.Listing, error) {
	var row listingRow
	err := r.DB.WithContext(ctx).
		Where("id = ? AND state = ?", id, string(listing.StatePublished)).
		First(&row).Error
	photos, perr := r.loadPhotos(ctx, id)
	if perr != nil {
		return listing.Listing{}, perr
	}
	return mapListing(row, photos, err)
}

// UpdateState transitions draft ⇄ published ⇄ paused.
func (r *Repo) UpdateState(ctx context.Context, id string, state listing.State) error {
	res := r.DB.WithContext(ctx).Model(&listingRow{}).
		Where("id = ?", id).
		Updates(map[string]any{"state": string(state), "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return listing.ErrNotFound
	}
	return nil
}

// ReplacePhotos is a no-op wrapper kept for the interface; the underlying
// Update already persists photos in the same transaction.
func (r *Repo) ReplacePhotos(ctx context.Context, listingID string, photos []string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.replacePhotosTx(tx, listingID, photos)
	})
}

// --- blocks ---------------------------------------------------------------

// AddBlock persists a block; overlap is rejected by the service before call.
func (r *Repo) AddBlock(ctx context.Context, b listing.Block) (listing.Block, error) {
	row := blockRow{
		ID:        b.ID,
		ListingID: b.ListingID,
		StartsAt:  b.StartsAt,
		EndsAt:    b.EndsAt,
		Reason:    b.Reason,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return listing.Block{}, err
	}
	return toBlock(row), nil
}

func toBlock(r blockRow) listing.Block {
	return listing.Block{
		ID:        r.ID,
		ListingID: r.ListingID,
		StartsAt:  r.StartsAt,
		EndsAt:    r.EndsAt,
		Reason:    r.Reason,
		CreatedAt: r.CreatedAt,
	}
}

// ListBlocks returns every block of a listing.
func (r *Repo) ListBlocks(ctx context.Context, listingID string) ([]listing.Block, error) {
	var rows []blockRow
	if err := r.DB.WithContext(ctx).
		Where("listing_id = ?", listingID).
		Order("starts_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]listing.Block, len(rows))
	for i, b := range rows {
		out[i] = toBlock(b)
	}
	return out, nil
}

// ListBlocksInWindow filters by [from, to). Adjacency does not overlap.
func (r *Repo) ListBlocksInWindow(ctx context.Context, listingID string, from, to time.Time) ([]listing.Block, error) {
	var rows []blockRow
	if err := r.DB.WithContext(ctx).
		Where("listing_id = ? AND starts_at < ? AND ends_at > ?", listingID, to, from).
		Order("starts_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]listing.Block, len(rows))
	for i, b := range rows {
		out[i] = toBlock(b)
	}
	return out, nil
}

// RemoveBlock deletes a block by id. Unknown id → ErrNotFound.
func (r *Repo) RemoveBlock(ctx context.Context, listingID, blockID string) error {
	res := r.DB.WithContext(ctx).
		Where("listing_id = ? AND id = ?", listingID, blockID).
		Delete(&blockRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return listing.ErrNotFound
	}
	return nil
}

// --- owner onboarding ------------------------------------------------------

// GetOwnerOnboarding returns the onboarding state. Missing row → empty.
func (r *Repo) GetOwnerOnboarding(ctx context.Context, accountID string) (listing.OwnerOnboarding, error) {
	var row ownerOnboardingRow
	err := r.DB.WithContext(ctx).Where("account_id = ?", accountID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return listing.OwnerOnboarding{AccountID: accountID}, nil
		}
		return listing.OwnerOnboarding{}, err
	}
	return listing.OwnerOnboarding{
		AccountID:       row.AccountID,
		PayoutKind:      row.PayoutKind,
		PayoutLast4:     row.PayoutLast4,
		TermsAcceptedAt: derefTime(row.TermsAcceptedAt),
		TermsVersion:    row.TermsVersion,
	}, nil
}

// UpsertOwnerOnboarding writes the onboarding row (Postgres ON CONFLICT).
func (r *Repo) UpsertOwnerOnboarding(ctx context.Context, o listing.OwnerOnboarding) (listing.OwnerOnboarding, error) {
	row := ownerOnboardingRow{
		AccountID:       o.AccountID,
		PayoutKind:      strings.TrimSpace(o.PayoutKind),
		PayoutLast4:     strings.TrimSpace(o.PayoutLast4),
		TermsAcceptedAt: timePtr(o.TermsAcceptedAt),
		TermsVersion:    o.TermsVersion,
	}
	err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"payout_kind":      row.PayoutKind,
			"payout_last4":     row.PayoutLast4,
			"terms_accepted_at": row.TermsAcceptedAt,
			"terms_version":    row.TermsVersion,
			"updated_at":       time.Now().UTC(),
		}),
	}).Create(&row).Error
	if err != nil {
		return listing.OwnerOnboarding{}, err
	}
	return r.GetOwnerOnboarding(ctx, o.AccountID)
}

// --- categories -----------------------------------------------------------

// CategoryConfig returns every category with its size and deposit min.
func (r *Repo) CategoryConfig(ctx context.Context) ([]listing.CategoryConfig, error) {
	var rows []categoryRow
	if err := r.DB.WithContext(ctx).
		Order("display_order ASC, category ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]listing.CategoryConfig, len(rows))
	for i, c := range rows {
		out[i] = listing.CategoryConfig{
			Category:        listing.Category(c.Category),
			Size:            listing.Size(c.Size),
			DepositMinCents: c.DepositMinCents,
		}
	}
	return out, nil
}

// CategoryByName looks up a category; ok=false when missing.
func (r *Repo) CategoryByName(ctx context.Context, c listing.Category) (listing.CategoryConfig, bool, error) {
	var row categoryRow
	err := r.DB.WithContext(ctx).Where("category = ?", string(c)).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return listing.CategoryConfig{}, false, nil
		}
		return listing.CategoryConfig{}, false, err
	}
	return listing.CategoryConfig{
		Category:        listing.Category(row.Category),
		Size:            listing.Size(row.Size),
		DepositMinCents: row.DepositMinCents,
	}, true, nil
}

// --- search ---------------------------------------------------------------

// SearchCatalog returns listings that match the supplied filters. `from`/`to`
// are an availability window that excludes listings with overlapping blocks.
func (r *Repo) SearchCatalog(ctx context.Context, f listing.SearchFilters) ([]listing.Listing, int, error) {
	page := f.Page
	if page < 1 {
		page = 1
	}
	size := f.PageSize
	if size <= 0 || size > 100 {
		size = 20
	}
	q := r.DB.WithContext(ctx).Model(&listingRow{}).
		Where("state = ?", string(listing.StatePublished))
	if f.Category != "" {
		q = q.Where("category = ?", string(f.Category))
	}
	if f.City != "" {
		q = q.Where("pickup_city ILIKE ?", f.City)
	}
	if f.Size != "" {
		q = q.Where("category IN (?)", r.DB.Model(&categoryRow{}).
			Where("size = ?", string(f.Size)).
			Select("category"))
	}
	if f.OperatorMode != "" {
		q = q.Where("operator_mode = ?", string(f.OperatorMode))
	}
	if f.MinPriceCents > 0 {
		q = q.Where("price_amount_cents >= ?", f.MinPriceCents)
	}
	if f.MaxPriceCents > 0 {
		q = q.Where("price_amount_cents <= ?", f.MaxPriceCents)
	}
	if !f.From.IsZero() && !f.To.IsZero() && f.From.Before(f.To) {
		// Exclude listings with any block overlapping [from, to).
		sub := r.DB.Model(&blockRow{}).
			Select("listing_id").
			Where("starts_at < ? AND ends_at > ?", f.To, f.From)
		q = q.Where("id NOT IN (?)", sub)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []listingRow
	if err := q.Order("created_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]listing.Listing, 0, len(rows))
	for _, row := range rows {
		photos, err := r.loadPhotos(ctx, row.ID)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, toListing(row, photos))
	}
	return out, int(total), nil
}

// --- helpers --------------------------------------------------------------

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}