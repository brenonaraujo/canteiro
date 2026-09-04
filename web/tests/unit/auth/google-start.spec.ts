import { describe, expect, it } from 'vitest'
import type { Account } from '../../../app/composables/auth/account'
import type { AccountMutator } from '../../../app/composables/auth/actions'
import { interpretGoogleStart } from '../../../app/composables/auth/api'
import { probeGoogleStart, startGoogleLogin } from '../../../app/composables/auth/google'

function fakeStore(): AccountMutator & { snapshot: () => {
  pending: boolean
  errorKey: string | null
} } {
  let pending = false
  let errorKey: string | null = null
  return {
    setAccount: (_next: Account | null) => {},
    setPending: (next) => {
      pending = next
    },
    setErrorKey: (next) => {
      errorKey = next
    },
    clear: () => {
      errorKey = null
    },
    snapshot: () => ({ pending, errorKey })
  }
}

describe('interpretGoogleStart', () => {
  it('keeps the visitor on login when the provider is missing', () => {
    expect(interpretGoogleStart({
      status: 503,
      location: null,
      code: 'not_configured'
    })).toEqual({ kind: 'not_configured' })
  })

  it('treats a bare 503 on the Google start URL as not configured', () => {
    expect(interpretGoogleStart({
      status: 503,
      location: null
    })).toEqual({ kind: 'not_configured' })
  })

  it('follows the provider Location when the backing is present', () => {
    expect(interpretGoogleStart({
      status: 302,
      location: 'https://accounts.google.com/o/oauth2/v2/auth?state=abc'
    })).toEqual({
      kind: 'redirect',
      url: 'https://accounts.google.com/o/oauth2/v2/auth?state=abc'
    })
  })

  it('falls back to a full-page start when CORS hides the 302', () => {
    expect(interpretGoogleStart({
      status: 0,
      location: null,
      type: 'opaqueredirect'
    })).toEqual({ kind: 'follow_start' })
  })

  it('does not expose a raw envelope for other failures', () => {
    expect(interpretGoogleStart({
      status: 500,
      location: null
    })).toEqual({ kind: 'unavailable' })
  })
})

describe('startGoogleLogin', () => {
  const startUrl = '/auth/google'

  it('stays on login with auth.not_configured when Google is missing', async () => {
    const store = fakeStore()
    const result = await startGoogleLogin(store, async () => ({
      status: 503,
      location: null,
      code: 'not_configured'
    }), startUrl)
    expect(result.redirectTo).toBeNull()
    expect(store.snapshot().errorKey).toBe('auth.not_configured')
    expect(store.snapshot().pending).toBe(false)
  })

  it('returns the provider Location when the backing is configured', async () => {
    const store = fakeStore()
    const result = await startGoogleLogin(store, async () => ({
      status: 302,
      location: 'https://accounts.google.com/o/oauth2/v2/auth'
    }), startUrl)
    expect(result.redirectTo).toBe('https://accounts.google.com/o/oauth2/v2/auth')
    expect(store.snapshot().errorKey).toBeNull()
  })

  it('falls back to the start URL when the 302 is opaque', async () => {
    const store = fakeStore()
    const result = await startGoogleLogin(store, async () => ({
      status: 0,
      location: null,
      type: 'opaqueredirect'
    }), startUrl)
    expect(result.redirectTo).toBe(startUrl)
    expect(store.snapshot().errorKey).toBeNull()
  })

  it('keeps a human generic error when the probe throws', async () => {
    const store = fakeStore()
    const result = await startGoogleLogin(store, async () => {
      throw new Error('network')
    }, startUrl)
    expect(result.redirectTo).toBeNull()
    expect(store.snapshot().errorKey).toBe('auth.error.generic')
  })
})

describe('probeGoogleStart', () => {
  it('reads not_configured from a 503 JSON envelope without navigating', async () => {
    const calls: Array<{ url: string, init?: RequestInit }> = []
    const raw = await probeGoogleStart(async (url, init) => {
      calls.push({ url, init })
      return new Response(JSON.stringify({
        code: 'not_configured',
        message_key: 'auth.not_configured'
      }), {
        status: 503,
        headers: { 'content-type': 'application/json' }
      })
    }, '')
    expect(raw).toMatchObject({
      status: 503,
      location: null,
      code: 'not_configured'
    })
    expect(calls[0]?.url).toBe('/auth/google')
    expect(calls[0]?.init?.redirect).toBe('manual')
  })

  it('exposes the Location of a configured Google start', async () => {
    const raw = await probeGoogleStart(async () => {
      return new Response(null, {
        status: 302,
        headers: { location: 'https://accounts.google.com/o/oauth2/v2/auth' }
      })
    }, 'http://localhost:8080')
    expect(raw.status).toBe(302)
    expect(raw.location).toBe('https://accounts.google.com/o/oauth2/v2/auth')
    expect(raw.code).toBeUndefined()
  })
})
