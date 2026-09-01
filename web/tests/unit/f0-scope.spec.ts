import { describe, expect, it } from 'vitest'
import ptBR from '../../i18n/locales/pt-BR.json'

describe('F0 copy scope', () => {
  it('does not advertise Stripe checkout', () => {
    const blob = JSON.stringify(ptBR).toLowerCase()
    expect(blob.includes('stripe')).toBe(false)
  })
})
