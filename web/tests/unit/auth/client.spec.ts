import { describe, expect, it } from 'vitest'
import { createAccountClient } from '../../../app/composables/auth/client'
import { ACCESS_TOKEN_KEY } from '../../../app/composables/auth/session'

function memoryStorage(initial?: Record<string, string>) {
  const memory = new Map(Object.entries(initial ?? {}))
  return {
    getItem: (key: string) => memory.get(key) ?? null,
    setItem: (key: string, value: string) => {
      memory.set(key, value)
    },
    removeItem: (key: string) => {
      memory.delete(key)
    }
  }
}

describe('account client', () => {
  it('loads the current account with the stored bearer token', async () => {
    const calls: Array<{ url: string, auth: string | null }> = []
    const client = createAccountClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' }),
      fetch: async (input, init) => {
        const headers = new Headers(init?.headers)
        calls.push({
          url: String(input),
          auth: headers.get('Authorization')
        })
        return new Response(JSON.stringify({
          id: 'acc-1',
          status: 'incomplete',
          display_name: '',
          phone: ''
        }), { status: 200 })
      }
    })

    const account = await client.fetchMe()
    expect(account).toEqual({
      id: 'acc-1',
      status: 'incomplete',
      displayName: '',
      phone: ''
    })
    expect(calls[0]).toEqual({
      url: 'http://localhost:8080/account',
      auth: 'Bearer tok-1'
    })
  })

  it('updates the profile and returns the mapped account', async () => {
    const client = createAccountClient({
      apiBase: 'http://localhost:8080',
      storage: memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' }),
      fetch: async (_input, init) => {
        const body = JSON.parse(String(init?.body))
        expect(body).toEqual({ display_name: 'Ana', phone: '11999999999' })
        return new Response(JSON.stringify({
          id: 'acc-1',
          status: 'active',
          display_name: 'Ana',
          phone: '11999999999'
        }), { status: 200 })
      }
    })

    const account = await client.saveProfile({
      displayName: 'Ana',
      phone: '11999999999'
    })
    expect(account.status).toBe('active')
    expect(account.displayName).toBe('Ana')
  })

  it('clears the session after logout', async () => {
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' })
    const client = createAccountClient({
      apiBase: 'http://localhost:8080',
      storage,
      fetch: async () => new Response(null, { status: 204 })
    })

    await client.logout()
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })

  it('clears the session after deactivate', async () => {
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' })
    const client = createAccountClient({
      apiBase: 'http://localhost:8080',
      storage,
      fetch: async () => new Response(null, { status: 204 })
    })

    await client.deactivate()
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })

  it('treats 401 as no session', async () => {
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'stale' })
    const client = createAccountClient({
      apiBase: 'http://localhost:8080',
      storage,
      fetch: async () => new Response(JSON.stringify({
        code: 'unauthorized'
      }), { status: 401 })
    })

    await expect(client.fetchMe()).rejects.toMatchObject({ status: 401 })
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })

  it('surfaces non-401 API failures without dropping a valid session', async () => {
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' })
    const client = createAccountClient({
      apiBase: 'http://localhost:8080',
      storage,
      fetch: async () => new Response(JSON.stringify({ code: 'forbidden' }), {
        status: 403
      })
    })

    await expect(client.fetchMe()).rejects.toMatchObject({ status: 403 })
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBe('tok-1')
  })
})
