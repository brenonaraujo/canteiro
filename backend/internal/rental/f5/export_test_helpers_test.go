package f5

import (
	"context"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// WirePilar3OnClaimResolvedForTest exposes the unexported
// wirePilar3OnClaimResolved for tests in other packages (the
// integration-style pillar3_test.go lives in package f5_test). Used to
// exercise the residual-debt branch when v1's API cap would otherwise
// prevent agreed > deposit. The test-only hook is intentional and
// minimal — it does not bypass any production logic, it just exposes
// the same helper the production damage flow invokes.
func (s *Service) WirePilar3OnClaimResolvedForTest(ctx context.Context, claim rental.DamageClaim, agreed int64) error {
	return s.wirePilar3OnClaimResolved(ctx, claim, agreed)
}
