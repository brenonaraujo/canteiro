package listing

import (
	"context"
	"time"
)

// Repository persists listings and their supporting rows. F2 owns the
// pgx/GORM implementation in package listingpg; this interface is the
// boundary the service depends on.
type Repository interface {
	Create(ctx context.Context, l Listing) (Listing, error)
	Update(ctx context.Context, l Listing) (Listing, error)
	GetByID(ctx context.Context, id string) (Listing, error)
	ListByOwner(ctx context.Context, ownerID string) ([]Listing, error)
	GetPublic(ctx context.Context, id string) (Listing, error)
	UpdateState(ctx context.Context, id string, state State) error
	ReplacePhotos(ctx context.Context, listingID string, photos []string) error

	AddBlock(ctx context.Context, b Block) (Block, error)
	ListBlocks(ctx context.Context, listingID string) ([]Block, error)
	ListBlocksInWindow(ctx context.Context, listingID string, from, to time.Time) ([]Block, error)
	RemoveBlock(ctx context.Context, listingID, blockID string) error

	GetOwnerOnboarding(ctx context.Context, accountID string) (OwnerOnboarding, error)
	UpsertOwnerOnboarding(ctx context.Context, o OwnerOnboarding) (OwnerOnboarding, error)

	CategoryConfig(ctx context.Context) ([]CategoryConfig, error)
	CategoryByName(ctx context.Context, c Category) (CategoryConfig, bool, error)

	// SearchCatalog is the public-facing discovery query. `availability`
	// may be a zero `from`/`to` to skip the window filter. The
	// implementation joins with listing_blocks when needed.
	SearchCatalog(ctx context.Context, f SearchFilters) ([]Listing, int, error)
}

// SearchFilters mirrors the public catalog query parameters.
type SearchFilters struct {
	Category       Category
	City           string
	From           time.Time
	To             time.Time
	OperatorMode   OperatorMode
	Size           Size
	MinPriceCents  int64
	MaxPriceCents  int64
	Page           int
	PageSize       int
}
