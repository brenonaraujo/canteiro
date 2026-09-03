export type RentalState
  = | 'pending'
    | 'authorized'
    | 'confirmed'
    | 'declined'
    | 'expired'
    | 'cancelled'
    | 'cancellation_in_progress'
    | 'refunded'

export type OperatorMode = 'none' | 'optional' | 'required'

export type ListingCategory
  = | 'manual'
    | 'electric'
    | 'light_construction'
    | 'agricultural'
    | 'heavy'

export type PriceUnit = 'hour' | 'day'

export type SnapshotOperator = {
  mode: OperatorMode
  hourly_rate_cents: number
  min_hours: number
  is_owner: boolean
  name?: string
  phone?: string
}

export type ListingSnapshot = {
  owner_id: string
  title: string
  category: ListingCategory
  price_unit: PriceUnit
  price_amount_cents: number
  deposit_cents: number
  min_lead_time_hours?: number
  pickup_city?: string
  operator: SnapshotOperator
}

export type Rental = {
  id: string
  listing_id: string
  tenant_account_id: string
  state: RentalState
  starts_at: string
  ends_at: string
  with_operator: boolean
  operator_terms_accepted: boolean
  intent_key: string
  rent_cents: number
  operator_cents: number
  deposit_cents: number
  commission_cents: number
  owner_payout_cents: number
  operator_payout_cents: number
  listing_snapshot: ListingSnapshot
  acceptance_deadline_at?: string
  confirmed_at?: string
  declined_at?: string
  decline_reason?: string
  created_at: string
  updated_at: string
}

export type RentalQuoteOut = {
  rental_id: string
  rent_cents: number
  operator_cents: number
  deposit_cents: number
  total_cents: number
  commission_base_cents: number
  commission_cents: number
  owner_payout_cents: number
  operator_payout_cents: number
}

export type RentalReceipt = {
  rental_id: string
  tenant_account_id: string
  rent_cents: number
  operator_cents: number
  deposit_cents: number
  total_cents: number
  commission_base_cents: number
  commission_cents: number
  owner_payout_cents: number
  operator_payout_cents: number
  listing_snapshot: ListingSnapshot
  window_starts_at: string
  window_ends_at: string
  issued_at: string
}

export type RentalActor = 'tenant' | 'owner' | 'platform' | 'system'

export type CancellationReceipt = {
  rental_id: string
  cancelled_by: RentalActor
  cancelled_at: string
  window_label: string
  rent_cents: number
  operator_cents: number
  deposit_cents: number
  total_cents: number
  cancellation_fee_cents: number
  commission_cents: number
  owner_payout_cents: number
  operator_payout_cents: number
  deposit_status: 'held' | 'released' | 'partial' | 'forfe'
  processor_ref: string
}

export function isRentalState(value: string): value is RentalState {
  return [
    'pending',
    'authorized',
    'confirmed',
    'declined',
    'expired',
    'cancelled',
    'cancellation_in_progress',
    'refunded'
  ].includes(value)
}

export function isTerminalRentalState(state: RentalState): boolean {
  return state === 'cancelled'
    || state === 'refunded'
    || state === 'declined'
    || state === 'expired'
}

export function isCancellableByTenant(state: RentalState): boolean {
  return state === 'pending' || state === 'authorized' || state === 'confirmed'
}

export function isCancellableByOwner(state: RentalState): boolean {
  return state === 'pending' || state === 'authorized' || state === 'confirmed'
}

export function hoursUntil(start: string, now: Date = new Date()): number {
  const target = new Date(start).getTime()
  const diffMs = target - now.getTime()
  return diffMs / (1000 * 60 * 60)
}

export type CancellationWindow
  = | 'pre_acceptance'
    | 'before_24h_post_acceptance'
    | 'within_24h_post_acceptance'
    | 'after_start'
    | 'unknown'

export function classifyCancellationWindow(input: {
  acceptedAt: string | null
  startsAt: string
  now?: Date
}): CancellationWindow {
  const now = input.now ?? new Date()
  if (!input.acceptedAt) {
    return 'pre_acceptance'
  }
  const hoursToStart = hoursUntil(input.startsAt, now)
  if (hoursToStart >= 24) {
    return 'before_24h_post_acceptance'
  }
  if (hoursToStart >= 0) {
    return 'within_24h_post_acceptance'
  }
  return 'after_start'
}