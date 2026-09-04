import { describe, expect, it } from 'vitest'
import type { Account } from '../../../app/composables/auth/account'
import {
  completeAuthCallback,
  deactivateAccount,
  hydrateAccount,
  logoutAccount,
  resolveApiBase,
  saveAccountProfile,
  type AccountMutator
} from '../../../app/composables/auth/actions'
import { createAccountClient } from '../../../app/composables/auth/client'
import { ACCESS_TOKEN_KEY, type TokenStorage } from '../../../app/composables/auth/session'

function memoryStorage(initial?: Record<string, string>): TokenStorage {
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

function fakeStore(): AccountMutator & { snapshot: () => {
  account: Account | null
  pending: boolean
  errorKey: string | null
} } {
  let account: Account | null = null
  let pending = false
  let errorKey: string | null = null
  return {
    setAccount: (next) => {
      account = next
    },
    setPending: (next) => {
      pending = next
    },
    setErrorKey: (next) => {
      errorKey = next
    },
    clear: () => {
      account = null
      errorKey = null
    },
    snapshot: () => ({ account, pending, errorKey })
  }
}

function jsonClient(
  storage: TokenStorage,
  handler: (input: string, init?: RequestInit) => Response | Promise<Response>
) {
  return createAccountClient({
    apiBase: 'http://localhost:8080',
    storage,
    fetch: async (input, init) => handler(input, init)
  })
}

describe('auth actions', () => {
  it('resolves a local API base only in development when unset', () => {
    expect(resolveApiBase('https://api.example', false)).toBe('https://api.example')
    expect(resolveApiBase('', true)).toBe('http://localhost:8080')
    expect(resolveApiBase('', false)).toBe('')
  })

  it('hydrates an active account and clears a missing session', async () => {
    const store = fakeStore()
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' })
    await hydrateAccount(store, jsonClient(storage, async () => Response.json({
      id: 'acc-1',
      status: 'incomplete',
      display_name: 'Ana',
      phone: '1199'
    })), storage)
    expect(store.snapshot().account?.displayName).toBe('Ana')

    await hydrateAccount(store, jsonClient(storage, async () => new Response(null, {
      status: 401
    })), storage)
    expect(store.snapshot().account).toBeNull()
  })

  it('drops a deactivated account during hydrate', async () => {
    const store = fakeStore()
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' })
    await hydrateAccount(store, jsonClient(storage, async () => Response.json({
      id: 'acc-1',
      status: 'deactivated',
      display_name: 'Ana',
      phone: '1199'
    })), storage)
    expect(store.snapshot().errorKey).toBe('auth.error.deactivated')
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })

  it('records callback errors without hydrating', async () => {
    const store = fakeStore()
    const storage = memoryStorage()
    const parsed = await completeAuthCallback(
      store,
      jsonClient(storage, async () => new Response(null, { status: 500 })),
      storage,
      { error: 'denied' }
    )
    expect(parsed.error).toBe('denied')
    expect(store.snapshot().errorKey).toBe('auth.callback.error')
  })

  it('stores an access token from the callback then hydrates', async () => {
    const store = fakeStore()
    const storage = memoryStorage()
    await completeAuthCallback(
      store,
      jsonClient(storage, async () => Response.json({
        id: 'acc-1',
        status: 'incomplete',
        display_name: '',
        phone: ''
      })),
      storage,
      { accessToken: 'tok-9' }
    )
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBe('tok-9')
    expect(store.snapshot().account?.id).toBe('acc-1')
  })

  it('hydrates a cookie session when the callback has no access token', async () => {
    const store = fakeStore()
    const storage = memoryStorage()
    await completeAuthCallback(
      store,
      jsonClient(storage, async () => Response.json({
        id: 'acc-2',
        status: 'incomplete',
        display_name: '',
        phone: ''
      })),
      storage,
      { sessionReady: true }
    )
    expect(store.snapshot().account?.id).toBe('acc-2')
  })

  it('keeps the visitor and explains the miss when auth=ok does not establish a session', async () => {
    const store = fakeStore()
    const storage = memoryStorage()
    const parsed = await completeAuthCallback(
      store,
      jsonClient(storage, async () => new Response(null, { status: 401 })),
      storage,
      { sessionReady: true }
    )
    expect(parsed.sessionReady).toBe(true)
    expect(store.snapshot().account).toBeNull()
    expect(store.snapshot().errorKey).toBe('auth.callback.error')
  })

  it('keeps the deactivated message when Google reopens a closed account', async () => {
    const store = fakeStore()
    const storage = memoryStorage()
    await completeAuthCallback(
      store,
      jsonClient(storage, async () => Response.json({
        id: 'acc-9',
        status: 'deactivated',
        display_name: 'Ana',
        phone: '1199'
      })),
      storage,
      { sessionReady: true }
    )
    expect(store.snapshot().account).toBeNull()
    expect(store.snapshot().errorKey).toBe('auth.error.deactivated')
  })

  it('does not call the account API when Google returns denied', async () => {
    const store = fakeStore()
    const storage = memoryStorage()
    let called = false
    await completeAuthCallback(
      store,
      jsonClient(storage, async () => {
        called = true
        return new Response(null, { status: 200 })
      }),
      storage,
      { error: 'denied' }
    )
    expect(called).toBe(false)
    expect(store.snapshot().account).toBeNull()
    expect(store.snapshot().errorKey).toBe('auth.callback.error')
  })

  it('rejects a blank profile before calling the API', async () => {
    const store = fakeStore()
    const storage = memoryStorage()
    const result = await saveAccountProfile(
      store,
      jsonClient(storage, async () => {
        throw new Error('should not call')
      }),
      { displayName: ' ', phone: '' }
    )
    expect(result.ok).toBe(false)
    expect(store.snapshot().errorKey).toBe('auth.profile.required')
  })

  it('saves a valid profile and maps 403 to deactivated', async () => {
    const store = fakeStore()
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' })
    const ok = await saveAccountProfile(
      store,
      jsonClient(storage, async () => Response.json({
        id: 'acc-1',
        status: 'active',
        display_name: 'Ana',
        phone: '1199'
      })),
      { displayName: 'Ana', phone: '1199' }
    )
    expect(ok.ok).toBe(true)
    expect(store.snapshot().account?.status).toBe('active')

    const denied = await saveAccountProfile(
      store,
      jsonClient(storage, async () => new Response(null, { status: 403 })),
      { displayName: 'Ana', phone: '1199' }
    )
    expect(denied.ok).toBe(false)
    expect(store.snapshot().errorKey).toBe('auth.error.deactivated')

    const failed = await saveAccountProfile(
      store,
      jsonClient(storage, async () => new Response(null, { status: 500 })),
      { displayName: 'Ana', phone: '1199' }
    )
    expect(failed.ok).toBe(false)
    expect(store.snapshot().errorKey).toBe('auth.error.generic')
  })

  it('clears local state after logout even if the API fails', async () => {
    const store = fakeStore()
    store.setAccount({
      id: 'acc-1',
      status: 'active',
      displayName: 'Ana',
      phone: '1199'
    })
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' })
    await logoutAccount(
      store,
      jsonClient(storage, async () => new Response(null, { status: 204 })),
      storage
    )
    expect(store.snapshot().account).toBeNull()

    store.setAccount({
      id: 'acc-1',
      status: 'active',
      displayName: 'Ana',
      phone: '1199'
    })
    storage.setItem(ACCESS_TOKEN_KEY, 'tok-1')
    await logoutAccount(
      store,
      jsonClient(storage, async () => new Response(null, { status: 500 })),
      storage
    )
    expect(store.snapshot().account).toBeNull()
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
  })

  it('clears local state after deactivate', async () => {
    const store = fakeStore()
    const storage = memoryStorage({ [ACCESS_TOKEN_KEY]: 'tok-1' })
    await deactivateAccount(
      store,
      jsonClient(storage, async () => new Response(null, { status: 204 }))
    )
    expect(store.snapshot().account).toBeNull()
  })
})
