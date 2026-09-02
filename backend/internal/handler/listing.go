package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
)

// ListingAPI wires the F2 listing endpoints (owner + public catalog).
type ListingAPI struct {
	svc          *listing.Service
	current      CurrentAccountFn
	defaultTerms string
}

// CurrentAccountFn returns the authenticated account id; "" when no session.
// We accept a func instead of importing auth to avoid a cycle.
type CurrentAccountFn func(c *gin.Context) (string, bool)

// NewListingAPI builds the adapter.
func NewListingAPI(svc *listing.Service, current CurrentAccountFn) *ListingAPI {
	if current == nil {
		current = noSession
	}
	return &ListingAPI{svc: svc, current: current, defaultTerms: "v1"}
}

func noSession(_ *gin.Context) (string, bool) { return "", false }

// --- helpers ---------------------------------------------------------------

func (l *ListingAPI) requireSession(c *gin.Context) (string, bool) {
	id, ok := l.current(c)
	if !ok {
		l.writeErr(c, http.StatusUnauthorized, "unauthorized", "listing.unauthorized")
		return "", false
	}
	return id, true
}

func (l *ListingAPI) writeErr(c *gin.Context, status int, code, key string) {
	c.JSON(status, api.Error{
		Code:       code,
		Message:    i18n.T(c.Request.Context(), key),
		MessageKey: key,
	})
}

func (l *ListingAPI) writeServiceErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, listing.ErrForbidden):
		l.writeErr(c, http.StatusForbidden, "forbidden", "listing.forbidden")
	case errors.Is(err, listing.ErrNotFound):
		l.writeErr(c, http.StatusNotFound, "not_found", "listing.not_found")
	case errors.Is(err, listing.ErrInvalidInput):
		l.writeErr(c, http.StatusUnprocessableEntity, "invalid_input", "listing.invalid_input")
	case errors.Is(err, listing.ErrAlreadyPublished):
		l.writeErr(c, http.StatusConflict, "already_published", "listing.already_published")
	case errors.Is(err, listing.ErrNotPublished):
		l.writeErr(c, http.StatusConflict, "not_published", "listing.not_published")
	case errors.Is(err, listing.ErrDeactivated):
		l.writeErr(c, http.StatusForbidden, "account_deactivated", "listing.account_deactivated")
	case errors.Is(err, listing.ErrProfileIncomplete):
		l.writeErr(c, http.StatusForbidden, "profile_incomplete", "listing.profile_incomplete")
	case errors.Is(err, listing.ErrOwnerOnboardingRequired):
		l.writeErr(c, http.StatusUnprocessableEntity, "owner_onboarding_required", "listing.owner_onboarding_required")
	case errors.Is(err, listing.ErrPublishGates):
		l.writeErr(c, http.StatusUnprocessableEntity, "publish_gates_unsatisfied", "listing.publish_gates")
	case errors.Is(err, listing.ErrBlockOverlap):
		l.writeErr(c, http.StatusConflict, "block_overlap", "listing.block_overlap")
	case errors.Is(err, listing.ErrBlockWindow):
		l.writeErr(c, http.StatusUnprocessableEntity, "invalid_window", "listing.block_window")
	default:
		l.writeErr(c, http.StatusInternalServerError, "internal_error", "error.internal")
	}
	return true
}

// toUUID parses a UUID-shaped string. Returns uuid.Nil on failure (the
// upstream validation in the codegen layer guarantees well-formed ids).
func toUUID(s string) openapi_types.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		return openapi_types.UUID{}
	}
	return openapi_types.UUID(u)
}

// --- conversions ----------------------------------------------------------

func listingToAPI(l listing.Listing) api.Listing {
	out := api.Listing{
		Id:                 toUUID(l.ID),
		OwnerAccountId:     toUUID(l.OwnerAccountID),
		State:              api.ListingState(l.State),
		Title:              l.Title,
		Description:        l.Description,
		Category:           api.ListingCategory(l.Category),
		PickupCity:         l.PickupCity,
		PickupNeighborhood: l.PickupNeighborhood,
		Delivery: api.ListingDelivery{
			Enabled:  l.Delivery.Enabled,
			Coverage: l.Delivery.Coverage,
		},
		PriceUnit:        api.PriceUnit(l.PriceUnit),
		PriceAmountCents: int(l.PriceAmountCents),
		DepositCents:     int(l.DepositCents),
		MinLeadTimeHours: l.MinLeadTimeHours,
		Operator:         operatorToAPI(l.Operator),
		Rules: api.ListingRules{
			DocumentRequired:   &l.Rules.DocumentRequired,
			ExperienceRequired: &l.Rules.ExperienceRequired,
			MinAge:             &l.Rules.MinAge,
			TravelRestricted:   &l.Rules.TravelRestricted,
		},
		Photos:            l.Photos,
		HeavyLegalCession: &l.HeavyLegalCession,
	}
	if !l.CreatedAt.IsZero() {
		t := l.CreatedAt
		out.CreatedAt = &t
	}
	if !l.UpdatedAt.IsZero() {
		t := l.UpdatedAt
		out.UpdatedAt = &t
	}
	return out
}

func operatorToAPI(op listing.Operator) api.ListingOperator {
	out := api.ListingOperator{Mode: api.OperatorMode(op.Mode)}
	if op.HourlyRateCents > 0 || op.Mode == listing.OperatorRequired || op.Mode == listing.OperatorOptional {
		h := int(op.HourlyRateCents)
		out.HourlyRateCents = &h
	}
	if op.MinHours > 0 {
		m := op.MinHours
		out.MinHours = &m
	}
	name := op.Identity.Name
	phone := op.Identity.Phone
	if name != "" || phone != "" || op.Identity.IsOwner {
		out.Identity = &api.OperatorIdentity{
			Name: name, Phone: phone, IsOwner: op.Identity.IsOwner,
		}
	}
	return out
}

func requestToListing(r api.CreateListingRequest) listing.Listing {
	del := listing.Delivery{}
	if r.Delivery != nil {
		del = listing.Delivery{Enabled: r.Delivery.Enabled, Coverage: r.Delivery.Coverage}
	}
	rules := listing.Rules{}
	if r.Rules != nil {
		rules = listing.Rules{
			DocumentRequired:   boolDeref(r.Rules.DocumentRequired),
			MinAge:             intDeref(r.Rules.MinAge),
			ExperienceRequired: boolDeref(r.Rules.ExperienceRequired),
			TravelRestricted:   boolDeref(r.Rules.TravelRestricted),
		}
	}
	op := listing.Operator{Mode: listing.OperatorMode(r.Operator.Mode)}
	if r.Operator.HourlyRateCents != nil {
		op.HourlyRateCents = int64(*r.Operator.HourlyRateCents)
	}
	if r.Operator.MinHours != nil {
		op.MinHours = *r.Operator.MinHours
	}
	if r.Operator.Identity != nil {
		op.Identity = listing.OperatorIdentity{
			Name:    r.Operator.Identity.Name,
			Phone:   r.Operator.Identity.Phone,
			IsOwner: r.Operator.Identity.IsOwner,
		}
	}
	var heavy bool
	if r.HeavyLegalCession != nil {
		heavy = *r.HeavyLegalCession
	}
	var photos []string
	if r.Photos != nil {
		photos = *r.Photos
	}
	return listing.Listing{
		Title:              r.Title,
		Description:        r.Description,
		Category:           listing.Category(r.Category),
		PickupCity:         r.PickupCity,
		PickupNeighborhood: r.PickupNeighborhood,
		Delivery:           del,
		PriceUnit:          listing.PriceUnit(r.PriceUnit),
		PriceAmountCents:   int64(r.PriceAmountCents),
		DepositCents:       int64(r.DepositCents),
		MinLeadTimeHours:   r.MinLeadTimeHours,
		Photos:             photos,
		Rules:              rules,
		Operator:           op,
		HeavyLegalCession:  heavy,
	}
}

func applyPatch(cur listing.Listing, p api.UpdateListingRequest) listing.Listing {
	if p.Title != nil {
		cur.Title = *p.Title
	}
	if p.Description != nil {
		cur.Description = *p.Description
	}
	if p.Category != nil {
		cur.Category = listing.Category(*p.Category)
	}
	if p.PickupCity != nil {
		cur.PickupCity = *p.PickupCity
	}
	if p.PickupNeighborhood != nil {
		cur.PickupNeighborhood = *p.PickupNeighborhood
	}
	if p.Delivery != nil {
		cur.Delivery = listing.Delivery{Enabled: p.Delivery.Enabled, Coverage: p.Delivery.Coverage}
	}
	if p.PriceUnit != nil {
		cur.PriceUnit = listing.PriceUnit(*p.PriceUnit)
	}
	if p.PriceAmountCents != nil {
		cur.PriceAmountCents = int64(*p.PriceAmountCents)
	}
	if p.DepositCents != nil {
		cur.DepositCents = int64(*p.DepositCents)
	}
	if p.MinLeadTimeHours != nil {
		cur.MinLeadTimeHours = *p.MinLeadTimeHours
	}
	if p.Operator != nil {
		op := listing.Operator{Mode: listing.OperatorMode(p.Operator.Mode)}
		if p.Operator.HourlyRateCents != nil {
			op.HourlyRateCents = int64(*p.Operator.HourlyRateCents)
		}
		if p.Operator.MinHours != nil {
			op.MinHours = *p.Operator.MinHours
		}
		if p.Operator.Identity != nil {
			op.Identity = listing.OperatorIdentity{
				Name:    p.Operator.Identity.Name,
				Phone:   p.Operator.Identity.Phone,
				IsOwner: p.Operator.Identity.IsOwner,
			}
		}
		cur.Operator = op
	}
	if p.Rules != nil {
		cur.Rules = listing.Rules{
			DocumentRequired:   boolDeref(p.Rules.DocumentRequired),
			MinAge:             intDeref(p.Rules.MinAge),
			ExperienceRequired: boolDeref(p.Rules.ExperienceRequired),
			TravelRestricted:   boolDeref(p.Rules.TravelRestricted),
		}
	}
	if p.HeavyLegalCession != nil {
		cur.HeavyLegalCession = *p.HeavyLegalCession
	}
	if p.Photos != nil {
		cur.Photos = *p.Photos
	}
	return cur
}

func onboardingToAPI(o listing.OwnerOnboarding) api.OwnerOnboarding {
	out := api.OwnerOnboarding{
		PayoutSet:     o.PayoutSet(),
		TermsAccepted: o.TermsAccepted(defaultTerms),
		TermsVersion:  defaultTerms,
	}
	if o.PayoutKind != "" {
		s := o.PayoutKind
		out.PayoutKind = &s
	}
	if o.PayoutLast4 != "" {
		s := o.PayoutLast4
		out.PayoutLast4 = &s
	}
	if !o.TermsAcceptedAt.IsZero() {
		t := o.TermsAcceptedAt
		out.TermsAcceptedAt = &t
	}
	return out
}

func onboardingFromPatch(p api.UpdateOwnerOnboardingRequest) listing.OwnerOnboarding {
	out := listing.OwnerOnboarding{}
	if p.PayoutKind != nil {
		out.PayoutKind = *p.PayoutKind
	}
	if p.PayoutLast4 != nil {
		out.PayoutLast4 = *p.PayoutLast4
	}
	if p.AcceptTerms != nil && *p.AcceptTerms {
		out.TermsAcceptedAt = time.Now().UTC()
		out.TermsVersion = defaultTerms
	}
	return out
}

func blockToAPI(b listing.Block, listingID string) api.AvailabilityBlock {
	return api.AvailabilityBlock{
		Id:        toUUID(b.ID),
		ListingId: toUUID(listingID),
		StartsAt:  b.StartsAt,
		EndsAt:    b.EndsAt,
		Reason:    b.Reason,
		CreatedAt: b.CreatedAt,
	}
}

func publicBlockToAPI(b listing.Block) struct {
	EndsAt   time.Time `json:"ends_at"`
	StartsAt time.Time `json:"starts_at"`
} {
	return struct {
		EndsAt   time.Time `json:"ends_at"`
		StartsAt time.Time `json:"starts_at"`
	}{EndsAt: b.EndsAt, StartsAt: b.StartsAt}
}

func categoryConfigToAPI(c listing.CategoryConfig) api.CategoryConfig {
	return api.CategoryConfig{
		Category:        api.ListingCategory(c.Category),
		DepositMinCents: int(c.DepositMinCents),
		Size:            api.ListingSize(c.Size),
	}
}

func publicListingToAPI(l listing.Listing) api.PublicListing {
	out := api.PublicListing{
		Id:                 toUUID(l.ID),
		Category:           api.ListingCategory(l.Category),
		Size:               api.ListingSize(l.Category.Size()),
		Title:              l.Title,
		Description:        l.Description,
		PickupCity:         l.PickupCity,
		PickupNeighborhood: l.PickupNeighborhood,
		PriceUnit:          api.PriceUnit(l.PriceUnit),
		PriceAmountCents:   int(l.PriceAmountCents),
		DepositCents:       int(l.DepositCents),
		MinLeadTimeHours:   l.MinLeadTimeHours,
		Photos:             l.Photos,
		Rules: api.ListingRules{
			DocumentRequired:   &l.Rules.DocumentRequired,
			ExperienceRequired: &l.Rules.ExperienceRequired,
			MinAge:             &l.Rules.MinAge,
			TravelRestricted:   &l.Rules.TravelRestricted,
		},
		CreatedAt: l.CreatedAt,
	}
	mode := api.OperatorMode(l.Operator.Mode)
	out.OperatorMode = &mode
	if l.Operator.HourlyRateCents > 0 {
		h := int(l.Operator.HourlyRateCents)
		out.OperatorHourlyRateCents = &h
	}
	if l.Delivery.Enabled {
		v := true
		out.DeliveryEnabled = &v
	}
	return out
}

func listingPageToAPI(items []listing.Listing, page, size, total int) api.ListingPage {
	out := api.ListingPage{
		Page:     page,
		PageSize: size,
		Total:    total,
		Items:    make([]api.PublicListing, len(items)),
	}
	for i, l := range items {
		out.Items[i] = publicListingToAPI(l)
	}
	return out
}

func boolDeref(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func intDeref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func parseSearchFilters(p api.SearchCatalogParams) listing.SearchFilters {
	f := listing.SearchFilters{
		PageSize: 20,
	}
	if p.Category != nil {
		f.Category = listing.Category(string(*p.Category))
	}
	if p.City != nil {
		f.City = *p.City
	}
	if p.OperatorMode != nil {
		f.OperatorMode = listing.OperatorMode(string(*p.OperatorMode))
	}
	if p.Size != nil {
		f.Size = listing.Size(string(*p.Size))
	}
	if p.MinPriceCents != nil {
		f.MinPriceCents = int64(*p.MinPriceCents)
	}
	if p.MaxPriceCents != nil {
		f.MaxPriceCents = int64(*p.MaxPriceCents)
	}
	if p.Page != nil {
		f.Page = *p.Page
	}
	if p.From != nil {
		f.From = p.From.Time
	}
	if p.To != nil {
		f.To = p.To.Time
	}
	return f
}

// defaultTerms is the owner terms version this adapter expects. Centralised
// so tests can override it via the domain layer if needed.
const defaultTerms = "v1"

// --- owner endpoints -------------------------------------------------------

// ListMineListings returns every listing owned by the caller.
func (l *ListingAPI) ListMineListings(c *gin.Context) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	items, err := l.svc.ListMine(c.Request.Context(), ownerID)
	if err != nil {
		l.writeServiceErr(c, err)
		return
	}
	out := make([]api.Listing, len(items))
	for i, item := range items {
		out[i] = listingToAPI(item)
	}
	c.JSON(http.StatusOK, out)
}

// CreateListingDraft persists a new draft listing.
func (l *ListingAPI) CreateListingDraft(c *gin.Context) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	var req api.CreateListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		l.writeErr(c, http.StatusBadRequest, "invalid_payload", "listing.invalid_payload")
		return
	}
	draft := requestToListing(req)
	got, err := l.svc.CreateDraft(c.Request.Context(), ownerID, draft)
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusCreated, listingToAPI(got))
}

// GetMyListing returns one of the caller's listings (private view).
func (l *ListingAPI) GetMyListing(c *gin.Context, id openapi_types.UUID) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	got, err := l.svc.GetMine(c.Request.Context(), ownerID, id.String())
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, listingToAPI(got))
}

// UpdateListing edits a draft or paused listing.
func (l *ListingAPI) UpdateListing(c *gin.Context, id openapi_types.UUID) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	var req api.UpdateListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		l.writeErr(c, http.StatusBadRequest, "invalid_payload", "listing.invalid_payload")
		return
	}
	cur, err := l.svc.GetMine(c.Request.Context(), ownerID, id.String())
	if l.writeServiceErr(c, err) {
		return
	}
	cur = applyPatch(cur, req)
	got, err := l.svc.Update(c.Request.Context(), ownerID, cur)
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, listingToAPI(got))
}

// PublishListing transitions to published after gate check.
func (l *ListingAPI) PublishListing(c *gin.Context, id openapi_types.UUID) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	got, err := l.svc.Publish(c.Request.Context(), ownerID, id.String())
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, listingToAPI(got))
}

// PauseListing reverses publish without losing data.
func (l *ListingAPI) PauseListing(c *gin.Context, id openapi_types.UUID) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	got, err := l.svc.Pause(c.Request.Context(), ownerID, id.String())
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, listingToAPI(got))
}

// ListBlocks returns the owner's blocks.
func (l *ListingAPI) ListBlocks(c *gin.Context, id openapi_types.UUID) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	bs, err := l.svc.ListBlocks(c.Request.Context(), ownerID, id.String())
	if l.writeServiceErr(c, err) {
		return
	}
	out := make([]api.AvailabilityBlock, len(bs))
	for i, b := range bs {
		out[i] = blockToAPI(b, id.String())
	}
	c.JSON(http.StatusOK, out)
}

// AddBlock adds an availability block.
func (l *ListingAPI) AddBlock(c *gin.Context, id openapi_types.UUID) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	var req api.AddBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		l.writeErr(c, http.StatusBadRequest, "invalid_payload", "listing.invalid_payload")
		return
	}
	reason := ""
	if req.Reason != nil {
		reason = *req.Reason
	}
	b, err := l.svc.AddBlock(c.Request.Context(), ownerID, id.String(), listing.Block{
		StartsAt: req.StartsAt,
		EndsAt:   req.EndsAt,
		Reason:   reason,
	})
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusCreated, blockToAPI(b, id.String()))
}

// RemoveBlock deletes one block by id.
func (l *ListingAPI) RemoveBlock(c *gin.Context, id, blockId openapi_types.UUID) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	if err := l.svc.RemoveBlock(c.Request.Context(), ownerID, id.String(), blockId.String()); l.writeServiceErr(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// GetOwnerOnboarding returns the caller's onboarding state.
func (l *ListingAPI) GetOwnerOnboarding(c *gin.Context) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	o, err := l.svc.GetOwnerOnboarding(c.Request.Context(), ownerID)
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, onboardingToAPI(o))
}

// UpdateOwnerOnboarding updates payout details and/or accepts terms.
func (l *ListingAPI) UpdateOwnerOnboarding(c *gin.Context) {
	ownerID, ok := l.requireSession(c)
	if !ok {
		return
	}
	var req api.UpdateOwnerOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		l.writeErr(c, http.StatusBadRequest, "invalid_payload", "listing.invalid_payload")
		return
	}
	patch := onboardingFromPatch(req)
	o, err := l.svc.UpsertOwnerOnboarding(c.Request.Context(), ownerID, patch)
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, onboardingToAPI(o))
}

// --- public catalog --------------------------------------------------------

// ListCategories returns the platform-defined category config.
func (l *ListingAPI) ListCategories(c *gin.Context) {
	cats, err := l.svc.ListCategories(c.Request.Context())
	if l.writeServiceErr(c, err) {
		return
	}
	out := make([]api.CategoryConfig, len(cats))
	for i, c := range cats {
		out[i] = categoryConfigToAPI(c)
	}
	c.JSON(http.StatusOK, out)
}

// SearchCatalog returns published listings matching the supplied filters.
func (l *ListingAPI) SearchCatalog(c *gin.Context, params api.SearchCatalogParams) {
	f := parseSearchFilters(params)
	items, total, err := l.svc.SearchCatalog(c.Request.Context(), f)
	if l.writeServiceErr(c, err) {
		return
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	c.JSON(http.StatusOK, listingPageToAPI(items, page, f.PageSize, total))
}

// GetPublicListing returns the public ficha of a published listing.
func (l *ListingAPI) GetPublicListing(c *gin.Context, id openapi_types.UUID) {
	got, err := l.svc.GetPublic(c.Request.Context(), id.String())
	if l.writeServiceErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, publicListingToAPI(got))
}

// GetPublicCalendar returns the published listing's active blocks.
func (l *ListingAPI) GetPublicCalendar(c *gin.Context, id openapi_types.UUID, params api.GetPublicCalendarParams) {
	var from, to time.Time
	if params.From != nil {
		from = params.From.Time
	}
	if params.To != nil {
		to = params.To.Time
	}
	bs, err := l.svc.GetPublicCalendar(c.Request.Context(), id.String(), from, to)
	if l.writeServiceErr(c, err) {
		return
	}
	out := api.PublicCalendar{
		ListingId: id,
		Blocks:    make([]struct {
			EndsAt   time.Time `json:"ends_at"`
			StartsAt time.Time `json:"starts_at"`
		}, len(bs)),
	}
	for i, b := range bs {
		out.Blocks[i] = publicBlockToAPI(b)
	}
	if pub, err := l.svc.GetPublic(c.Request.Context(), id.String()); err == nil {
		out.MinLeadTimeHours = pub.MinLeadTimeHours
	}
	c.JSON(http.StatusOK, out)
}