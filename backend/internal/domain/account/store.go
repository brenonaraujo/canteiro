package account

import "context"

// Repository persists accounts. Google subject is unique.
type Repository interface {
	GetByID(ctx context.Context, id string) (Account, error)
	GetByGoogleSubject(ctx context.Context, subject string) (Account, error)
	Create(ctx context.Context, acc Account) error
	Update(ctx context.Context, acc Account) error
}
