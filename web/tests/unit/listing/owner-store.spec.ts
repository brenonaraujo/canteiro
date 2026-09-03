import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { useOwnerListingsStore } from '../../../app/stores/listing/owner'
import { ListingRequestError, type ListingClient } from '../../../app/composables/listing/client'
import type { Listing } from '../../../app/composables/listing/types'

function makeListing(overrides: Partial<Listing> = {}): Listing {
  return {
    id: 'l1',
    owner_account_id: 'acc-1',
    state: 'draft',
    title: 'Furadeira',
    description: 'Furadeira de bancada 750W.',
    category: 'manual',
    pickup_city: 'São Paulo',
    pickup_neighborhood: 'Vila Mariana',
    delivery: { enabled: false },
    price_unit: 'day',
    price_amount_cents: 15000,
    deposit_cents: 5000,
    min_lead_time_hours: 24,
    photos: ['p.jpg'],
    rules: {},
    operator: { mode: 'none' },
    ...overrides
  } as Listing
}

function makeClient(overrides: Partial<ListingClient> = {}): ListingClient {
  return {
    searchCatalog: async () => ({ page: 1, page_size: 20, total: 0, items: [] }),
    getPublicListing: async () => ({} as never),
    getPublicCalendar: async () => ({ listing_id: 'l1', min_lead_time_hours: 0, blocks: [] }),
    listCategories: async () => [],
    listMine: async () => [],
    getMine: async () => makeListing(),
    createDraft: async input => makeListing({ title: input.title }),
    updateListing: async (_id, input) => makeListing({ title: input.title ?? 'updated' }),
    publish: async () => makeListing({ state: 'published' }),
    pause: async () => makeListing({ state: 'paused' }),
    listBlocks: async () => [],
    addBlock: async () => ({} as never),
    removeBlock: async () => {},
    getOwnerOnboarding: async () => ({
      payout_set: true,
      terms_accepted: true,
      terms_version: 'v1'
    }),
    updateOwnerOnboarding: async body => ({
      payout_set: true,
      terms_accepted: true,
      terms_version: 'v1',
      ...body
    }),
    ...overrides
  }
}

describe('owner listings store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('loads the owner listings list', async () => {
    const store = useOwnerListingsStore()
    const client = makeClient({
      listMine: async () => [makeListing({ id: 'a' }), makeListing({ id: 'b' })]
    })
    await store.loadMine(client)
    expect(store.items).toHaveLength(2)
    expect(store.errorKey).toBeNull()
  })

  it('publish propagates server gate errors into errorKey and publishGates', async () => {
    const store = useOwnerListingsStore()
    const client = makeClient({
      publish: async () => {
        throw new ListingRequestError(422, 'listing.publish.gates_failed', [
          'photos',
          'deposit_cents'
        ])
      }
    })
    const result = await store.publish(client, 'l1')
    expect(result).toBeNull()
    expect(store.errorKey).toBe('listing.error.gates_failed')
    expect(store.publishGates).toEqual(['photos', 'deposit_cents'])
  })

  it('create prepends the new listing to items', async () => {
    const store = useOwnerListingsStore()
    const client = makeClient({
      listMine: async () => [makeListing({ id: 'existing' })],
      createDraft: async input => makeListing({ id: 'new', title: input.title })
    })
    await store.loadMine(client)
    await store.create(client, {
      title: 'Furadeira',
      description: 'Furadeira de bancada 750W.',
      category: 'manual',
      pickup_city: 'SP',
      price_unit: 'day',
      price_amount_cents: 1000,
      deposit_cents: 5000,
      min_lead_time_hours: 24,
      operator: { mode: 'none' }
    })
    expect(store.items[0]?.id).toBe('new')
    expect(store.items[1]?.id).toBe('existing')
  })

  it('pause updates the local item state', async () => {
    const store = useOwnerListingsStore()
    const client = makeClient({
      listMine: async () => [makeListing({ id: 'l1', state: 'published' })]
    })
    await store.loadMine(client)
    await store.pause(client, 'l1')
    expect(store.items[0]?.state).toBe('paused')
  })

  it('loadOnboarding exposes the owner state', async () => {
    const store = useOwnerListingsStore()
    await store.loadOnboarding(makeClient())
    expect(store.onboarding?.payout_set).toBe(true)
    expect(store.onboarding?.terms_accepted).toBe(true)
  })
})
