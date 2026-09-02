export type ListingCategory
  = | 'manual'
    | 'electric'
    | 'light_construction'
    | 'agricultural'
    | 'heavy'

export type ListingSize = 'light' | 'heavy'

export type OperatorMode = 'none' | 'optional' | 'required'

export type PriceUnit = 'hour' | 'day'

export type ListingState = 'draft' | 'published' | 'paused'

export type ListingRules = {
  document_required?: boolean
  min_age?: number
  experience_required?: boolean
  travel_restricted?: boolean
}

export type ListingDelivery = {
  enabled: boolean
  coverage?: string
}

export type OperatorIdentity = {
  name: string
  phone: string
  is_owner: boolean
}

export type ListingOperator = {
  mode: OperatorMode
  hourly_rate_cents?: number
  min_hours?: number
  identity?: OperatorIdentity
}

export type ListingPhoto = string

export type Listing = {
  id: string
  owner_account_id: string
  state: ListingState
  title: string
  description: string
  category: ListingCategory
  size?: ListingSize
  pickup_city: string
  pickup_neighborhood: string
  delivery: ListingDelivery
  price_unit: PriceUnit
  price_amount_cents: number
  deposit_cents: number
  min_lead_time_hours: number
  photos: ListingPhoto[]
  rules: ListingRules
  operator: ListingOperator
  heavy_legal_cession?: boolean
  created_at?: string
  updated_at?: string
}

export type PublicListing = {
  id: string
  title: string
  description: string
  category: ListingCategory
  size?: ListingSize
  pickup_city: string
  pickup_neighborhood: string
  delivery_enabled?: boolean
  price_unit: PriceUnit
  price_amount_cents: number
  deposit_cents: number
  min_lead_time_hours: number
  photos: ListingPhoto[]
  rules: ListingRules
  operator_mode: OperatorMode
  operator_hourly_rate_cents?: number
  created_at: string
}

export type ListingPage = {
  page: number
  page_size: number
  total: number
  items: PublicListing[]
}

export type CategoryConfig = {
  category: ListingCategory
  size: ListingSize
  deposit_min_cents: number
}

export type AvailabilityBlock = {
  id: string
  listing_id: string
  starts_at: string
  ends_at: string
  reason?: string
  created_at: string
}

export type PublicCalendar = {
  listing_id: string
  min_lead_time_hours: number
  blocks: Array<{ starts_at: string, ends_at: string }>
}

export type OwnerOnboarding = {
  payout_set: boolean
  terms_accepted: boolean
  terms_version: string
  terms_accepted_at?: string
  payout_kind?: string
  payout_last4?: string
}

export type CreateListingInput = {
  title: string
  description: string
  category: ListingCategory
  pickup_city: string
  pickup_neighborhood?: string
  delivery?: ListingDelivery
  price_unit: PriceUnit
  price_amount_cents: number
  deposit_cents: number
  min_lead_time_hours: number
  photos?: ListingPhoto[]
  rules?: ListingRules
  operator: ListingOperator
  heavy_legal_cession?: boolean
}

export type UpdateListingInput = Partial<CreateListingInput>

export type ListingSearchFilters = {
  category?: ListingCategory
  city?: string
  from?: string
  to?: string
  operator_mode?: OperatorMode
  size?: ListingSize
  min_price_cents?: number
  max_price_cents?: number
  page?: number
}

export type ListingDraftErrors = {
  title?: string
  description?: string
  pickup_city?: string
  pickup_neighborhood?: string
  price_amount_cents?: string
  deposit_cents?: string
  min_lead_time_hours?: string
  photos?: string
  operator_mode?: string
  operator_identity?: string
  operator_hourly_rate_cents?: string
  delivery_coverage?: string
  heavy_legal_cession?: string
}

export type ListingFormState = {
  title: string
  description: string
  category: ListingCategory
  size: ListingSize
  pickup_city: string
  pickup_neighborhood: string
  delivery_enabled: boolean
  delivery_coverage: string
  price_unit: PriceUnit
  price_amount_cents: number
  deposit_cents: number
  min_lead_time_hours: number
  photos: ListingPhoto[]
  rules_document_required: boolean
  rules_min_age: number
  rules_experience_required: boolean
  rules_travel_restricted: boolean
  operator_mode: OperatorMode
  operator_hourly_rate_cents: number
  operator_min_hours: number
  operator_name: string
  operator_phone: string
  operator_is_owner: boolean
  heavy_legal_cession: boolean
}

export type ListingFormResult = {
  ok: boolean
  value?: CreateListingInput
  errors?: ListingDraftErrors
}

export function isListingCategory(value: string): value is ListingCategory {
  return [
    'manual',
    'electric',
    'light_construction',
    'agricultural',
    'heavy'
  ].includes(value)
}

export function isListingSize(value: string): value is ListingSize {
  return ['light', 'heavy'].includes(value)
}

export function isOperatorMode(value: string): value is OperatorMode {
  return ['none', 'optional', 'required'].includes(value)
}

export function isPriceUnit(value: string): value is PriceUnit {
  return ['hour', 'day'].includes(value)
}

export function categorySize(category: ListingCategory): ListingSize {
  return category === 'heavy' ? 'heavy' : 'light'
}

export function depositMinimumFor(
  configs: CategoryConfig[],
  category: ListingCategory
): number {
  const size = categorySize(category)
  const match = configs.find(c => c.category === category && c.size === size)
  return match?.deposit_min_cents ?? 0
}
