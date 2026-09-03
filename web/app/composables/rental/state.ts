// F4 cancellation state helpers. Kept in a separate file from the
// data types so each module stays ≤ 150 lines.

import type { RentalState } from './types'

// F4 deposit states are 'released' | 'captured' | 'partial' | 'held'.
// In the tenant-facing UI we group 'held' under 'held' (still held; F5
// will decide capture) and the other states map directly.
export type DepositStatusUi = 'held' | 'released' | 'partial' | 'forfeit'

export function mapDepositStatus(state: string | undefined): DepositStatusUi {
  if (!state) return 'held'
  if (state === 'captured') return 'forfeit'
  if (state === 'released') return 'released'
  if (state === 'partial') return 'partial'
  return 'held'
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
