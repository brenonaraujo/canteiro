import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { useCatalogStore } from '../../../app/stores/listing/catalog'
import { ListingRequestError, type ListingClient } from '../../../app/composables/listing/client'
import type { PublicListing } from '../../../app/composables/listing/types'

const sampleItem: PublicListing = {
  id: 'l1', title: 'Furadeira', description: 'desc',
  category: 'manual', pickup_city: 'SP', pickup_neighborhood: 'VM',
  price_unit: 'day',
  price_amount_cents: 1000, deposit_cents: 5000, min_lead_time_hours: 24,
  photos: ['p.jpg'], rules: {}, operator_mode: 'none',
  created_at: '2026-01-01T00:00:00Z'
}

function makeClient(overrides: Partial<ListingClient> = {}): ListingClient {
  return {
    searchCatalog: async () => ({
      page: 1, page_size: 20, total: 1, items: [sampleItem]
    }),
    getPublicListing: async () => sampleItem,
    getPublicCalendar: async () => ({ listing_id: 'l1', min_lead_time_hours: 0, blocks: [] }),
    listCategories: async () => [
      { category: 'manual', size: 'light', deposit_min_cents: 5000 }
    ],
    listMine: async () => [],
    getMine: async () => ({} as never),
    createDraft: async () => ({} as never),
    updateListing: async () => ({} as never),
    publish: async () => ({} as never),
    pause: async () => ({} as never),
    listBlocks: async () => [],
    addBlock: async () => ({} as never),
    removeBlock: async () => {},
    getOwnerOnboarding: async () => ({
      payout_set: true, terms_accepted: true, terms_version: 'v1'
    }),
    updateOwnerOnboarding: async () => ({
      payout_set: true, terms_accepted: true, terms_version: 'v1'
    }),
    ...overrides
  }
}

describe('catalog store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('search sets the page and clears error on success', async () => {
    const store = useCatalogStore()
    await store.search({ city: 'SP' }, makeClient())
    expect(store.page?.total).toBe(1)
    expect(store.errorKey).toBeNull()
    expect(store.lastFilters).toEqual({ city: 'SP' })
  })

  it('search sets errorKey on failure', async () => {
    const store = useCatalogStore()
    const client = makeClient({
      searchCatalog: async () => {
        throw new ListingRequestError(500)
      }
    })
    await store.search({}, client)
    expect(store.errorKey).toBe('listing.error.search_failed')
  })

  it('loadCategories fills the categories list', async () => {
    const store = useCatalogStore()
    await store.loadCategories(makeClient())
    expect(store.categories[0]?.deposit_min_cents).toBe(5000)
  })
})
