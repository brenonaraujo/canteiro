import { describe, expect, it } from 'vitest'
import {
  guestRedirectTarget,
  sessionView
} from '../../../app/composables/auth/gate'

describe('account session view', () => {
  it('only shows the account form when a session exists', () => {
    expect(sessionView({ authenticated: false, pending: false })).toBe('guest')
    expect(sessionView({ authenticated: false, pending: true })).toBe('loading')
    expect(sessionView({ authenticated: true, pending: false })).toBe('form')
    expect(sessionView({ authenticated: true, pending: true })).toBe('form')
  })

  it('redirects guests on the client after hydrate, never on the server', () => {
    expect(guestRedirectTarget('guest', false)).toBeNull()
    expect(guestRedirectTarget('loading', true)).toBeNull()
    expect(guestRedirectTarget('form', true)).toBeNull()
    expect(guestRedirectTarget('guest', true)).toBe('/auth/login')
  })
})
