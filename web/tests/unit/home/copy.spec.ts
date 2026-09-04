import { describe, expect, it } from 'vitest'
import { flattenKeys } from '../../../app/utils/flattenKeys'
import en from '../../../i18n/locales/en.json'
import es from '../../../i18n/locales/es.json'
import ptBR from '../../../i18n/locales/pt-BR.json'

const PLACEHOLDER = /em construção|em breve|veja o que vem|see what's coming|under construction|coming soon|en construcción|ver lo que viene|what's coming|ver o que vem/i

const required = [
  'landing.hero.title',
  'landing.hero.tagline',
  'landing.hero.cta',
  'landing.hero.badge',
  'landing.highlights.title',
  'landing.highlights.owner.title',
  'landing.highlights.owner.desc',
  'landing.highlights.renter.title',
  'landing.highlights.renter.desc',
  'landing.highlights.platform.title',
  'landing.highlights.platform.desc',
  'landing.market.title',
  'landing.market.empty_title',
  'landing.market.empty_body',
  'landing.market.view_all',
  'landing.market.publish',
  'landing.market.error_title',
  'landing.market.error_body',
  'landing.market.retry',
  'auth.login.unavailable_title',
  'auth.login.unavailable_body',
  'auth.login.browse_catalog',
  'auth.login.still_visitor',
  'a11y.logo'
]

describe('home copy (#26)', () => {
  it('exposes vitrine keys in en, pt-BR and es', () => {
    const keys = flattenKeys(en)
    for (const key of required) {
      expect(keys, key).toContain(key)
    }
    expect(flattenKeys(ptBR)).toEqual(keys)
    expect(flattenKeys(es)).toEqual(keys)
  })

  it('drops pre-launch placeholder residue in every locale', () => {
    const blobs = [
      JSON.stringify(en.landing),
      JSON.stringify(ptBR.landing),
      JSON.stringify(es.landing),
      JSON.stringify(en.auth.login),
      JSON.stringify(ptBR.auth.login),
      JSON.stringify(es.auth.login)
    ]
    for (const blob of blobs) {
      expect(blob).not.toMatch(PLACEHOLDER)
    }
    expect(en.landing).not.toHaveProperty('soon')
    expect(ptBR.landing).not.toHaveProperty('soon')
    expect(es.landing).not.toHaveProperty('soon')
  })

  it('keeps the primary CTA as a catalog action, not a teaser', () => {
    expect(en.landing.hero.cta.toLowerCase()).not.toMatch(/coming|soon|construction/)
    expect(ptBR.landing.hero.cta.toLowerCase()).not.toMatch(/construção|breve|vem aí/)
    expect(es.landing.hero.cta.toLowerCase()).not.toMatch(/construcción|breve|viene/)
  })
})
