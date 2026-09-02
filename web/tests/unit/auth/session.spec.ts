import { describe, expect, it } from 'vitest'
import {
  ACCESS_TOKEN_KEY,
  browserStorage,
  parseAuthCallback,
  readAccessToken,
  writeAccessToken
} from '../../../app/composables/auth/session'

describe('auth session', () => {
  it('reads access_token from the callback query', () => {
    expect(parseAuthCallback('?access_token=tok-1')).toEqual({
      accessToken: 'tok-1'
    })
  })

  it('accepts token as an alias used by some backends', () => {
    expect(parseAuthCallback('?token=tok-2')).toEqual({
      accessToken: 'tok-2'
    })
  })

  it('surfaces provider errors without creating a session', () => {
    expect(parseAuthCallback('?error=access_denied')).toEqual({
      error: 'access_denied'
    })
    expect(parseAuthCallback('?auth=denied')).toEqual({ error: 'denied' })
    expect(parseAuthCallback('?auth=error')).toEqual({ error: 'error' })
  })

  it('treats a cookie callback as session-ready', () => {
    expect(parseAuthCallback('?auth=ok')).toEqual({ sessionReady: true })
  })

  it('returns empty when Google is interrupted before callback data exists', () => {
    expect(parseAuthCallback('')).toEqual({})
    expect(parseAuthCallback('?')).toEqual({})
    expect(parseAuthCallback('?state=xyz')).toEqual({})
  })

  it('persists and clears the access token through a storage adapter', () => {
    const memory = new Map<string, string>()
    const storage = {
      getItem: (key: string) => memory.get(key) ?? null,
      setItem: (key: string, value: string) => {
        memory.set(key, value)
      },
      removeItem: (key: string) => {
        memory.delete(key)
      }
    }

    writeAccessToken(storage, 'tok-3')
    expect(memory.get(ACCESS_TOKEN_KEY)).toBe('tok-3')
    expect(readAccessToken(storage)).toBe('tok-3')

    writeAccessToken(storage, null)
    expect(readAccessToken(storage)).toBeNull()
  })

  it('does not read window storage in Node tests', () => {
    expect(browserStorage()).toBeNull()
  })
})
