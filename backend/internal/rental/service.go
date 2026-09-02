package rental

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// Repository persists rentals, payment intents, webhook events and receipts.
type Repository interface {
	CreateIntent(ctx context.Context, r rental.Rental, snapBytes []byte) (rental.Rental, error)
	GetByID(ctx context.Context, id string) (rental.Rental, error)
	GetByIntentKey(ctx context.Context, tenantID, listingID, intentKey string) (rental.Rental, bool, error)
	ListForOwner(ctx context.Context, ownerID string, states []rental.State) ([]rental.Rental, error)
	ListForTenant(ctx context.Context, tenantID string, states []rental.State) ([]rental.Rental, error)
	UpdateState(ctx context.Context, id string, from, to rental.State, mutate func(r *rental.Rental)) (rental.Rental, error)
	ListActiveOverlapping(ctx context.Context, listingID string, start, end time.Time) ([]rental.Rental, error)
	ListOwnerBlocks(ctx context.Context, listingID string, start, end time.Time) ([]Block, error)
	SaveReceipt(ctx context.Context, rec rental.Receipt) (rental.Receipt, error)
	GetReceipt(ctx context.Context, rentalID string) (rental.Receipt, bool, error)
	UpsertPaymentIntent(ctx context.Context, intent PaymentIntent) (PaymentIntent, error)
	GetPaymentIntent(ctx context.Context, rentalID string) (PaymentIntent, bool, error)
	RecordWebhookEvent(ctx context.Context, ev WebhookEvent) (WebhookEvent, error)
}

// PaymentIntent is the persisted representation of a PSP intent.
type PaymentIntent struct {
	ID                  string
	RentalID            string
	Provider            string
	ProviderPaymentID   string
	IdempotencyKey      string
	Attempt             int
	AmountCents         int64
	DepositCents        int64
	ExpectedTotalCents  int64
	Status              string
	FailureCode         string
	FailureMessage      string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// WebhookEvent is the persisted record of a PSP event.
type WebhookEvent struct {
	ID              string
	Provider        string
	ProviderEventID string
	EventType       string
	RentalID        string
	PaymentIntentID string
	Payload         []byte
	SignatureValid  bool
	ReceivedAt      time.Time
	ProcessedAt     *time.Time
}

// Block is a thin wrapper around listing.Block.
type Block struct {
	ID        string
	ListingID string
	StartsAt  time.Time
	EndsAt    time.Time
}

// ListingLookup is the slice of listing.Service this service needs.
type ListingLookup interface {
	GetByID(ctx context.Context, id string) (listing.Listing, error)
}

// AccountLookup is the slice of account.Service this service needs.
type AccountLookup interface {
	GetByID(ctx context.Context, id string) (account.Account, error)
}

// IDGenerator produces rental + receipt + intent + webhook ids.
type IDGenerator interface {
	String() string
}

// Config holds the knobs the service reads from the platform env.
type Config struct {
	AcceptanceWindow   time.Duration
	CommissionBPS      int64
	MinLeadTime        time.Duration
	DefaultCurrency    string
	ProviderName       string
	ProviderWebhookKey string
	Now                func() time.Time
	IDGen              IDGenerator
}

// Defaults fills the zero-valued fields with the documented defaults.
func (c *Config) Defaults() {
	if c.AcceptanceWindow == 0 {
		c.AcceptanceWindow = 12 * time.Hour
	}
	if c.CommissionBPS == 0 {
		c.CommissionBPS = rental.DefaultCommissionBPS
	}
	if c.DefaultCurrency == "" {
		c.DefaultCurrency = "BRL"
	}
	if c.ProviderName == "" {
		c.ProviderName = "noop"
	}
	if c.Now == nil {
		c.Now = func() time.Time { return time.Now().UTC() }
	}
	if c.IDGen == nil {
		c.IDGen = defaultIDGen{}
	}
}

// Service orchestrates the rental lifecycle.
type Service struct {
	repo     Repository
	listing  ListingLookup
	accounts AccountLookup
	payment  PaymentProvider
	cfg      Config
}

// PaymentProvider is the adapter boundary.
type PaymentProvider interface {
	CreateIntent(ctx context.Context, req CreateIntentRequest) (CreateIntentResponse, error)
	VerifyWebhookSignature(ctx context.Context, rawBody []byte, signature string) (ProviderWebhookEvent, error)
}

// CreateIntentRequest is the input to the PSP.
type CreateIntentRequest struct {
	RentalID         string
	IdempotencyKey   string
	AmountCents      int64
	DepositCents     int64
	Currency         string
	AcceptanceWindow time.Duration
	Metadata         map[string]string
}

// CreateIntentResponse is the PSP's response.
type CreateIntentResponse struct {
	Provider          string
	ProviderPaymentID string
	Status            string
	FailureCode       string
	FailureMessage    string
}

// ProviderWebhookEvent is the (already-verified) PSP event.
type ProviderWebhookEvent struct {
	Provider         string
	ProviderEventID  string
	EventType        string
	RentalID         string
	PaymentIntentID  string
	AmountCents      int64
	DepositCents     int64
	FailureCode      string
	FailureMessage   string
	Authorized       bool
}

// NewService builds the service with default config filled in.
func NewService(repo Repository, listing ListingLookup, accounts AccountLookup, payment PaymentProvider, cfg Config) *Service {
	cfg.Defaults()
	return &Service{
		repo:     repo,
		listing:  listing,
		accounts: accounts,
		payment:  payment,
		cfg:      cfg,
	}
}

type defaultIDGen struct{}

func (defaultIDGen) String() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func intentKeyFromWindow(listingID string, start, end time.Time) string {
	return listingID + "|" + start.UTC().Format(time.RFC3339) + "|" + end.UTC().Format(time.RFC3339)
}

func tenantHasOpenDebt(r rental.Rental) bool {
	return r.TenantClaimDebt == "open"
}

func (s *Service) requireActiveTenant(ctx context.Context, tenantID string) error {
	acc, err := s.accounts.GetByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, account.ErrNotFound) {
			return rental.ErrForbidden
		}
		return err
	}
	if acc.Status == account.StatusDeactivated {
		return rental.ErrAccountDeactivated
	}
	if !acc.ProfileComplete() {
		return rental.ErrProfileIncomplete
	}
	return nil
}

func (s *Service) requirePublishedListingSnapshot(ctx context.Context, listingID string) (listing.Listing, rental.ListingSnapshot, error) {
	l, err := s.listing.GetByID(ctx, listingID)
	if err != nil {
		if errors.Is(err, listing.ErrNotFound) {
			return listing.Listing{}, rental.ListingSnapshot{}, rental.ErrNotFound
		}
		return listing.Listing{}, rental.ListingSnapshot{}, err
	}
	if l.State != listing.StatePublished {
		return listing.Listing{}, rental.ListingSnapshot{}, rental.ErrListingUnavailable
	}
	snap := rental.ListingSnapshot{
		OwnerID:            l.OwnerAccountID,
		Title:              l.Title,
		Category:           string(l.Category),
		PriceUnit:          string(l.PriceUnit),
		PriceAmountCents:   l.PriceAmountCents,
		DepositCents:       l.DepositCents,
		MinLeadTimeHours:   l.MinLeadTimeHours,
		PickupCity:         l.PickupCity,
		HeavyLegalCession:  l.HeavyLegalCession,
		Operator: rental.OperatorSnapshot{
			Mode:            string(l.Operator.Mode),
			HourlyRateCents: l.Operator.HourlyRateCents,
			MinHours:        l.Operator.MinHours,
			IsOwner:         l.Operator.Identity.IsOwner,
		},
	}
	if l.Operator.Identity.Name != "" {
		snap.Operator.Name = l.Operator.Identity.Name
	}
	if l.Operator.Identity.Phone != "" {
		snap.Operator.Phone = l.Operator.Identity.Phone
	}
	return l, snap, nil
}

func (s *Service) requireNoOverlap(ctx context.Context, listingID string, start, end time.Time) error {
	active, err := s.repo.ListActiveOverlapping(ctx, listingID, start, end)
	if err != nil {
		return err
	}
	if len(active) > 0 {
		return rental.ErrCalendarOverlap
	}
	blocks, err := s.repo.ListOwnerBlocks(ctx, listingID, start, end)
	if err != nil {
		return err
	}
	for _, b := range blocks {
		if rental.HasOverlap(start, end, b.StartsAt, b.EndsAt) {
			return rental.ErrCalendarOverlap
		}
	}
	return nil
}
