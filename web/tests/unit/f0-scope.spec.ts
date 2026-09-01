import { describe, expect, it } from 'vitest'
import ptBR from '../../i18n/locales/pt-BR.json'

describe('F0 copy scope', () => {
  it('does not advertise Google sign-in or Stripe checkout', () => {
    const blob = JSON.stringify(ptBR).toLowerCase()
    expect(blob.includes('google')).toBe(false)
    expect(blob.includes('stripe')).toBe(false)
  })
})
