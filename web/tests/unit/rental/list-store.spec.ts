import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { useRentalsListStore } from '../../../app/stores/rental/list'
import {
  RentalRequestError,
  type RentalClient
} from '../../../app/composables/rental/client'
import type { Rental } from '../../../app/composables/rental/types'

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

function makeClient(overrides: Partial<RentalClient> = {}): RentalClient {
  return {
    listMine: async () => [makeRental()],
    getMine: async () => makeRental(),
    cancel: async () => makeRental({ state: 'cancellation_in_progress' }),
    getReceipt: async () => ({
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
      issued_at: '2026-09-02T12:00:00Z'
    }),
    ...overrides
  }
}

describe('rentals list store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('load fills items and clears errorKey on success', async () => {
    const store = useRentalsListStore()
    await store.load(makeClient())
    expect(store.items).toHaveLength(1)
    expect(store.errorKey).toBeNull()
    expect(store.pending).toBe(false)
  })

  it('load sets errorKey on failure', async () => {
    const store = useRentalsListStore()
    const client = makeClient({
      listMine: async () => {
        throw new RentalRequestError(401)
      }
    })
    await store.load(client)
    expect(store.items).toHaveLength(0)
    expect(store.errorKey).toBe('rental.error.unauthorized')
  })

  it('upsert replaces an existing item and prepends a new one', () => {
    const store = useRentalsListStore()
    store.items = [makeRental({ id: 'r1' })]
    store.upsert(makeRental({ id: 'r1', state: 'cancelled' }))
    expect(store.items[0]?.state).toBe('cancelled')
    expect(store.items).toHaveLength(1)
    store.upsert(makeRental({ id: 'r2' }))
    expect(store.items).toHaveLength(2)
    expect(store.items[0]?.id).toBe('r2')
  })

  it('clearError clears the errorKey', async () => {
    const store = useRentalsListStore()
    const client = makeClient({
      listMine: async () => {
        throw new RentalRequestError(500)
      }
    })
    await store.load(client)
    expect(store.errorKey).not.toBeNull()
    store.clearError()
    expect(store.errorKey).toBeNull()
  })
})
