import { describe, expect, it } from 'vitest'
import { createListingClient } from '../../../app/composables/listing/client'
import { ACCESS_TOKEN_KEY, type TokenStorage } from '../../../app/composables/listing/../auth/session'

function memoryStorage(initial?: Record<string, string>): TokenStorage {
  const memory = new Map(Object.entries(initial ?? {}))
  return {
    getItem: key => memory.get(key) ?? null,
    setItem: (key, value) => {
      memory.set(key, value)
    },
    removeItem: (key) => {
      memory.delete(key)
    }
  }
}

type CallRecord = {
  url: string
  method: string
  body: unknown
  auth: string | null
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' }
  })
}

function fakeClient(handler: (record: CallRecord) => Promise<Response>) {
  const calls: CallRecord[] = []
  const fetch: typeof globalThis.fetch = async (input, init) => {
    const headers = new Headers(init?.headers)
    const body = typeof init?.body === 'string' ? JSON.parse(init.body) : null
    calls.push({
      url: String(input),
      method: init?.method ?? 'GET',
      body,
      auth: headers.get('Authorization')
    })
    return handler(calls[calls.length - 1]!)
  }
  return { fetch, calls }
}

describe('listing client — public catalog', () => {
  it('searches the public catalog with filters as query string', async () => {
    const { fetch, calls } = fakeClient(async () => jsonResponse({
      page: 1, page_size: 20, total: 0, items: []
    }))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage(),
      fetch
    })
    await client.searchCatalog({
      category: 'manual',
      city: 'São Paulo',
      min_price_cents: 1000,
      page: 1
    })
    const url = new URL(calls[0]!.url)
    expect(url.pathname).toBe('/catalog/listings')
    expect(url.searchParams.get('category')).toBe('manual')
    expect(url.searchParams.get('city')).toBe('São Paulo')
    expect(url.searchParams.get('min_price_cents')).toBe('1000')
    expect(url.searchParams.get('page')).toBe('1')
  })

  it('fetches the public ficha without a token', async () => {
    const { fetch, calls } = fakeClient(async () => jsonResponse({
      id: 'l1',
      title: 'Furadeira',
      description: 'desc',
      category: 'manual',
      pickup_city: 'SP',
      pickup_neighborhood: 'Vila Mariana',
      delivery: { enabled: false },
      price_unit: 'day',
      price_amount_cents: 1000,
      deposit_cents: 5000,
      min_lead_time_hours: 24,
      photos: ['p.jpg'],
      rules: {},
      operator: { mode: 'none' },
      created_at: '2026-01-01T00:00:00Z'
    }))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage(),
      fetch
    })
    const ficha = await client.getPublicListing('l1')
    expect(ficha.id).toBe('l1')
    expect(calls[0]!.auth).toBeNull()
  })
})

describe('listing client — owner surface', () => {
  it('sends the bearer token on protected endpoints', async () => {
    const { fetch, calls } = fakeClient(async () => jsonResponse([]))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok' }),
      fetch
    })
    await client.listMine()
    expect(calls[0]!.auth).toBe('Bearer tok')
  })

  it('creates a draft with the full payload', async () => {
    const { fetch, calls } = fakeClient(async () => jsonResponse({
      id: 'l1',
      state: 'draft'
    }))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok' }),
      fetch
    })
    await client.createDraft({
      title: 'Furadeira',
      description: 'Furadeira de bancada.',
      category: 'manual',
      pickup_city: 'São Paulo',
      price_unit: 'day',
      price_amount_cents: 15000,
      deposit_cents: 5000,
      min_lead_time_hours: 24,
      operator: { mode: 'none' }
    })
    expect(calls[0]!.method).toBe('POST')
    expect(calls[0]!.url).toBe('http://localhost:8080/listings')
    expect(calls[0]!.body).toMatchObject({
      title: 'Furadeira',
      category: 'manual'
    })
  })

  it('throws ListingRequestError with status 422 when publish gates fail', async () => {
    const { fetch } = fakeClient(async () => jsonResponse({
      code: 'unprocessable',
      message: 'fail',
      message_key: 'listing.publish.gates_failed',
      fields: ['photos', 'deposit_cents']
    }, 422))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok' }),
      fetch
    })
    await expect(client.publish('l1')).rejects.toMatchObject({
      status: 422,
      messageKey: 'listing.publish.gates_failed',
      fields: ['photos', 'deposit_cents']
    })
  })

  it('returns 204 as a successful no-content for block removal', async () => {
    const { fetch, calls } = fakeClient(async () => new Response(null, { status: 204 }))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok' }),
      fetch
    })
    await client.removeBlock('l1', 'b1')
    expect(calls[0]!.method).toBe('DELETE')
  })

  it('marks 401 as an error', async () => {
    const { fetch } = fakeClient(async () => jsonResponse({}, 401))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage(),
      fetch
    })
    await expect(client.listMine()).rejects.toMatchObject({ status: 401 })
  })
})
