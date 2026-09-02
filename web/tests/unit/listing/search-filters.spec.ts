import { describe, expect, it } from 'vitest'
import { createListingClient } from '../../../app/composables/listing/client'
import type { TokenStorage } from '../../../app/composables/auth/session'

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

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' }
  })
}

function fakeClient(handler: (url: string) => Promise<Response>) {
  const calls: string[] = []
  const fetch: typeof globalThis.fetch = async (input) => {
    const url = String(input)
    calls.push(url)
    return handler(url)
  }
  return { fetch, calls }
}

describe('listing client — searchCatalog query encoding (regression for #4 /listings hydration bug)', () => {
  it('drops null filters (the "Any" option in the UI must not be sent to the backend)', async () => {
    const { fetch, calls } = fakeClient(async () => jsonResponse({
      page: 1, page_size: 20, total: 0, items: []
    }))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage(),
      fetch
    })
    await client.searchCatalog({
      category: null,
      size: null,
      operator_mode: null,
      page: 1
    })
    const url = new URL(calls[0]!)
    expect(url.pathname).toBe('/catalog/listings')
    expect(url.searchParams.has('category')).toBe(false)
    expect(url.searchParams.has('size')).toBe(false)
    expect(url.searchParams.has('operator_mode')).toBe(false)
    expect(url.searchParams.get('page')).toBe('1')
  })

  it('treats null and undefined equivalently for filter params', async () => {
    const { fetch, calls } = fakeClient(async () => jsonResponse({
      page: 1, page_size: 20, total: 0, items: []
    }))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage(),
      fetch
    })
    await client.searchCatalog({
      category: null,
      city: undefined,
      operator_mode: null
    })
    const url = new URL(calls[0]!)
    expect(url.searchParams.has('category')).toBe(false)
    expect(url.searchParams.has('city')).toBe(false)
    expect(url.searchParams.has('operator_mode')).toBe(false)
  })

  it('still sends truthy filters when null is mixed in', async () => {
    const { fetch, calls } = fakeClient(async () => jsonResponse({
      page: 1, page_size: 20, total: 0, items: []
    }))
    const client = createListingClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage(),
      fetch
    })
    await client.searchCatalog({
      category: 'electric',
      size: null,
      operator_mode: 'optional',
      city: 'Curitiba'
    })
    const url = new URL(calls[0]!)
    expect(url.searchParams.get('category')).toBe('electric')
    expect(url.searchParams.has('size')).toBe(false)
    expect(url.searchParams.get('operator_mode')).toBe('optional')
    expect(url.searchParams.get('city')).toBe('Curitiba')
  })
})
