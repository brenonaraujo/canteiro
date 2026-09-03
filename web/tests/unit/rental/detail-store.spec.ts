import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { useRentalDetailStore } from '../../../app/stores/rental/detail'
import {
  RentalRequestError,
  type RentalClient
} from '../../../app/composables/rental/client'
import type { Rental, RentalReceipt } from '../../../app/composables/rental/types'

function makeRental(overrides: Partial<Rental> = {}): Rental {
  return {
    id: 'r1',
    listing_id: 'l1',
    tenant_account_id: 'a1',
    state: 'confirmed',
    starts_at: '2026-09-10T00:00:00Z',
    ends_at: '2026-09-10T20:00:00Z',
    with_operator: false,
    operator_terms_accepted: false,
    intent_key: 'k1',
    rent_cents: 15000,
    operator_cents: 0,
    deposit_cents: 5000,
    commission_cents: 1800,
    owner_payout_cents: 13200,
    operator_payout_cents: 0,
    listing_snapshot: {
      owner_id: 'o1',
      title: 'Furadeira',
      category: 'manual',
      price_unit: 'day',
      price_amount_cents: 15000,
      deposit_cents: 5000,
      operator: {
        mode: 'none',
        hourly_rate_cents: 0,
        min_hours: 0,
        is_owner: false
      }
    },
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
    ...overrides
  } as Rental
}

function makeReceipt(overrides: Partial<RentalReceipt> = {}): RentalReceipt {
  return {
    rental_id: 'r1',
    tenant_account_id: 'a1',
    rent_cents: 15000,
    operator_cents: 0,
    deposit_cents: 5000,
    total_cents: 20000,
    commission_base_cents: 15000,
    commission_cents: 1800,
    owner_payout_cents: 13200,
    operator_payout_cents: 0,
    listing_snapshot: makeRental().listing_snapshot,
    window_starts_at: '2026-09-10T00:00:00Z',
    window_ends_at: '2026-09-10T20:00:00Z',
    issued_at: '2026-09-02T12:00:00Z',
    ...overrides
  }
}

function makeClient(overrides: Partial<RentalClient> = {}): RentalClient {
  return {
    listMine: async () => [makeRental()],
    getMine: async () => makeRental(),
    cancel: async () => makeRental({ state: 'cancellation_in_progress' }),
    getReceipt: async () => makeReceipt(),
    ...overrides
  }
}

describe('rental detail store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('load fills the rental and clears the error', async () => {
    const store = useRentalDetailStore()
    await store.load('r1', makeClient())
    expect(store.rental?.id).toBe('r1')
    expect(store.errorKey).toBeNull()
  })

  it('load sets a translated errorKey on failure', async () => {
    const store = useRentalDetailStore()
    const client = makeClient({
      getMine: async () => {
        throw new RentalRequestError(404)
      }
    })
    await store.load('r1', client)
    expect(store.rental).toBeNull()
    expect(store.errorKey).toBe('rental.error.not_found')
  })

  it('loadReceipt surfaces the receipt-not-ready key for 404', async () => {
    const store = useRentalDetailStore()
    const client = makeClient({
      getReceipt: async () => {
        throw new RentalRequestError(404)
      }
    })
    await store.loadReceipt('r1', client)
    expect(store.receiptErrorKey).toBe('rental.error.receipt_not_ready')
    expect(store.receipt).toBeNull()
  })

  it('cancel returns true and updates the rental when the API succeeds', async () => {
    const store = useRentalDetailStore()
    const ok = await store.cancel('r1', makeClient())
    expect(ok).toBe(true)
    expect(store.rental?.state).toBe('cancellation_in_progress')
    expect(store.cancelErrorKey).toBeNull()
  })

  it('cancel returns false and surfaces conflict errorKey on 409', async () => {
    const store = useRentalDetailStore()
    const client = makeClient({
      cancel: async () => {
        throw new RentalRequestError(409)
      }
    })
    const ok = await store.cancel('r1', client)
    expect(ok).toBe(false)
    expect(store.cancelErrorKey).toBe('rental.error.conflict')
  })

  it('reset clears every flag', async () => {
    const store = useRentalDetailStore()
    await store.load('r1', makeClient())
    await store.loadReceipt('r1', makeClient())
    store.reset()
    expect(store.rental).toBeNull()
    expect(store.receipt).toBeNull()
    expect(store.errorKey).toBeNull()
    expect(store.receiptErrorKey).toBeNull()
    expect(store.cancelErrorKey).toBeNull()
  })
})
