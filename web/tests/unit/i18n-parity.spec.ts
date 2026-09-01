import { describe, expect, it } from 'vitest'
import { flattenKeys } from '../../app/utils/flattenKeys'
import en from '../../i18n/locales/en.json'
import es from '../../i18n/locales/es.json'
import ptBR from '../../i18n/locales/pt-BR.json'

describe('i18n parity', () => {
  it('keeps en, pt-BR and es keys in lockstep', () => {
    const enKeys = flattenKeys(en)
    const ptKeys = flattenKeys(ptBR)
    const esKeys = flattenKeys(es)

    expect(ptKeys).toEqual(enKeys)
    expect(esKeys).toEqual(enKeys)
  })

  it('exposes namespaced keys (no flat error1-style ids)', () => {
    const keys = flattenKeys(en)
    expect(keys.every(key => key.includes('.'))).toBe(true)
  })
})
