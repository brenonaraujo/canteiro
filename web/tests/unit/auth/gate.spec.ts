import { describe, expect, it } from 'vitest'
import {
  googleAccessFailed,
  guestRedirectTarget,
  sessionView
} from '../../../app/composables/auth/gate'

describe('googleAccessFailed', () => {
  it('treats denied and error callbacks as a failed access', () => {
    expect(googleAccessFailed({ auth: 'denied' })).toBe(true)
    expect(googleAccessFailed({ auth: 'error' })).toBe(true)
  })

  it('does not treat a completed Google access as a failure', () => {
    expect(googleAccessFailed({ auth: 'ok' })).toBe(false)
    expect(googleAccessFailed({})).toBe(false)
    expect(googleAccessFailed({ error: 'not_configured' })).toBe(false)
  })
})

describe('sessionView', () => {
  it('shows the form when the person is authenticated', () => {
    expect(sessionView({ authenticated: true, pending: false })).toBe('form')
    expect(sessionView({ authenticated: true, pending: true })).toBe('form')
  })

  it('shows loading while a session is resolving', () => {
    expect(sessionView({ authenticated: false, pending: true })).toBe('loading')
  })

  it('keeps a guest on the public surface', () => {
    expect(sessionView({ authenticated: false, pending: false })).toBe('guest')
  })
})

describe('guestRedirectTarget', () => {
  it('sends a client-side guest to sign-in', () => {
    expect(guestRedirectTarget('guest', true)).toBe('/auth/login')
  })

  it('does not redirect while loading or authenticated', () => {
    expect(guestRedirectTarget('loading', true)).toBeNull()
    expect(guestRedirectTarget('form', true)).toBeNull()
    expect(guestRedirectTarget('guest', false)).toBeNull()
  })
})
