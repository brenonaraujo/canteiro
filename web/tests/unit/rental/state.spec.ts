import { describe, expect, it } from 'vitest'
import {
  classifyCancellationWindow,
  isCancellableByOwner,
  isCancellableByTenant,
  isRentalState,
  isTerminalRentalState
} from '../../../app/composables/rental/state'

describe('rental state helpers', () => {
  it('isRentalState validates the full union', () => {
    expect(isRentalState('confirmed')).toBe(true)
    expect(isRentalState('cancellation_in_progress')).toBe(true)
    expect(isRentalState('unknown')).toBe(false)
  })

  it('isTerminalRentalState covers the F4 terminal set', () => {
    expect(isTerminalRentalState('cancelled')).toBe(true)
    expect(isTerminalRentalState('refunded')).toBe(true)
    expect(isTerminalRentalState('declined')).toBe(true)
    expect(isTerminalRentalState('expired')).toBe(true)
    expect(isTerminalRentalState('confirmed')).toBe(false)
    expect(isTerminalRentalState('cancellation_in_progress')).toBe(false)
  })

  it('isCancellableByTenant and isCancellableByOwner share the same set', () => {
    const cases = [
      'pending', 'authorized', 'confirmed', 'cancelled', 'declined', 'refunded'
    ] as const
    for (const state of cases) {
      expect(isCancellableByTenant(state)).toBe(isCancellableByOwner(state))
    }
  })
})

describe('classifyCancellationWindow', () => {
  it('returns pre_acceptance when there is no acceptance timestamp', () => {
    expect(classifyCancellationWindow({
      acceptedAt: null,
      startsAt: '2030-01-01T00:00:00Z'
    })).toBe('pre_acceptance')
  })

  it('returns before_24h_post_acceptance when ≥ 24h to pickup', () => {
    const now = new Date('2026-09-02T00:00:00Z')
    expect(classifyCancellationWindow({
      acceptedAt: '2026-08-01T00:00:00Z',
      startsAt: '2026-09-03T00:00:00Z',
      now
    })).toBe('before_24h_post_acceptance')
  })

  it('returns within_24h_post_acceptance when < 24h to pickup', () => {
    const now = new Date('2026-09-02T18:00:00Z')
    expect(classifyCancellationWindow({
      acceptedAt: '2026-09-01T00:00:00Z',
      startsAt: '2026-09-03T00:00:00Z',
      now
    })).toBe('within_24h_post_acceptance')
  })

  it('returns after_start when pickup has passed', () => {
    const now = new Date('2026-09-04T00:00:00Z')
    expect(classifyCancellationWindow({
      acceptedAt: '2026-08-01T00:00:00Z',
      startsAt: '2026-09-03T00:00:00Z',
      now
    })).toBe('after_start')
  })
})
