import { describe, expect, it } from 'vitest'
import {
  accountFromApi,
  authPath,
  deactivatePayload,
  googleStartUrl,
  profilePayload
} from '../../../app/composables/auth/api'

describe('auth api helpers', () => {
  it('builds a Google start URL against the API host', () => {
    expect(googleStartUrl('http://localhost:8080')).toBe(
      'http://localhost:8080/auth/google'
    )
  })

  it('uses a same-origin path when apiBase is empty', () => {
    expect(googleStartUrl('')).toBe('/auth/google')
  })

  it('exposes the F1 auth resource paths from OpenAPI', () => {
    expect(authPath('me')).toBe('/account')
    expect(authPath('logout')).toBe('/auth/logout')
    expect(authPath('deactivate')).toBe('/account/deactivate')
  })

  it('maps API snake_case into the account model', () => {
    expect(accountFromApi({
      id: 'acc-1',
      status: 'incomplete',
      display_name: 'Ana',
      phone: '11999999999'
    })).toEqual({
      id: 'acc-1',
      status: 'incomplete',
      displayName: 'Ana',
      phone: '11999999999'
    })
  })

  it('accepts camelCase displayName from the payload', () => {
    expect(accountFromApi({
      id: 'acc-2',
      status: 'active',
      displayName: 'Bia'
    }).displayName).toBe('Bia')
  })

  it('sends trimmed profile fields as snake_case', () => {
    expect(profilePayload({ displayName: ' Ana ', phone: ' 1199 ' })).toEqual({
      display_name: 'Ana',
      phone: '1199'
    })
  })

  it('deactivates only with an explicit confirm flag', () => {
    expect(deactivatePayload()).toEqual({ confirm: true })
  })
})
