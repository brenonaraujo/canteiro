import { describe, expect, it } from 'vitest'
import { parseAuthQuery } from '../../../app/composables/auth/session'

describe('auth callback query', () => {
  it('reads Nuxt route query objects', () => {
    expect(parseAuthQuery({ access_token: 'tok-9' })).toEqual({
      accessToken: 'tok-9'
    })
    expect(parseAuthQuery({ error: 'access_denied' })).toEqual({
      error: 'access_denied'
    })
  })
})
