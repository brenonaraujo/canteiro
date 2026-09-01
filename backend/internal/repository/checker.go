package repository

import "context"

// Checker is a readiness probe against a backing service.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}
