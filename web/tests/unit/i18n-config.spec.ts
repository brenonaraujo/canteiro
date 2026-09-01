import { describe, expect, it } from 'vitest'
import { isAppLocale } from '../../app/utils/locale'
import { i18nLocales, I18N_DEFAULT_LOCALE } from '../../i18n/options'

describe('i18n options', () => {
  it('defaults to pt-BR', () => {
    expect(I18N_DEFAULT_LOCALE).toBe('pt-BR')
  })

  it('registers en, pt-BR and es', () => {
    expect(i18nLocales.map(locale => locale.code)).toEqual(['en', 'pt-BR', 'es'])
  })

  it('accepts only registered locale codes', () => {
    expect(isAppLocale('pt-BR')).toBe(true)
    expect(isAppLocale('fr')).toBe(false)
  })
})
