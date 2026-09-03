package handler

import (
	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
)

// F5Stubs is a partial implementation of api.ServerInterface that
// returns 404 for every F5 endpoint. Test files in this package embed
// F5Stubs into their per-test *Server types so the api.RegisterHandlers
// dispatch table stays satisfied without bringing in real F5
// dependencies (which require a DB).
type F5Stubs struct{}

func (F5Stubs) RegisterRentalPickup(c *gin.Context, _ openapi_types.UUID) {
	c.Status(404)
}
func (F5Stubs) RegisterRentalReturn(c *gin.Context, _ openapi_types.UUID) {
	c.Status(404)
}
func (F5Stubs) OpenDamageClaim(c *gin.Context, _ openapi_types.UUID) { c.Status(404) }
func (F5Stubs) RespondDamageClaim(c *gin.Context, _ openapi_types.UUID) {
	c.Status(404)
}
func (F5Stubs) StaffResolveDamage(c *gin.Context, _ openapi_types.UUID) {
	c.Status(404)
}
func (F5Stubs) SettleDebt(c *gin.Context, _ openapi_types.UUID)  { c.Status(404) }
func (F5Stubs) ForgiveDebt(c *gin.Context, _ openapi_types.UUID) { c.Status(404) }

// F6 review stubs — parallel to F5Stubs above. The per-test *Server
// types (listingServer / rentalServer / paymentServer) embed
// F5Stubs, which transitively satisfies api.ServerInterface for the
// F6 review methods too. F6-specific tests build their own harness.
func (F5Stubs) CreateRentalReview(c *gin.Context, _ openapi_types.UUID) { c.Status(404) }
func (F5Stubs) ListUserReviews(c *gin.Context, _ openapi_types.UUID, _ api.ListUserReviewsParams) {
	c.Status(404)
}
func (F5Stubs) GetUserReviewAggregate(c *gin.Context, _ openapi_types.UUID, _ api.GetUserReviewAggregateParams) {
	c.Status(404)
}
