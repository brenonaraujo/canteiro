import { describe, expect, it } from 'vitest'
import { flattenKeys } from '../../../app/utils/flattenKeys'
import en from '../../../i18n/locales/en.json'
import es from '../../../i18n/locales/es.json'
import ptBR from '../../../i18n/locales/pt-BR.json'

const required = [
  'nav.sign_in',
  'nav.sign_out',
  'nav.account',
  'auth.login.title',
  'auth.login.description',
  'auth.login.google',
  'auth.login.google_only',
  'auth.login.catalog_hint',
  'auth.callback.working',
  'auth.callback.error',
  'auth.profile.title',
  'auth.profile.description',
  'auth.profile.display_name',
  'auth.profile.phone',
  'auth.profile.save',
  'auth.profile.saved',
  'auth.profile.incomplete',
  'auth.profile.required',
  'auth.profile.cancel',
  'auth.deactivate.title',
  'auth.deactivate.body',
  'auth.deactivate.listings',
  'auth.deactivate.rentals',
  'auth.deactivate.irreversible',
  'auth.deactivate.confirm',
  'auth.deactivate.submit',
  'auth.deactivate.cancel',
  'auth.status.active',
  'auth.status.incomplete',
  'auth.status.deactivated',
  'auth.error.generic',
  'auth.error.deactivated',
  'auth.not_configured',
  'breadcrumb.home',
  'breadcrumb.sign_in',
  'breadcrumb.account',
  'breadcrumb.deactivate'
]

describe('auth i18n', () => {
  it('exposes account copy in en, pt-BR and es', () => {
    const keys = flattenKeys(en)
    for (const key of required) {
      expect(keys, key).toContain(key)
    }
    expect(flattenKeys(ptBR)).toEqual(keys)
    expect(flattenKeys(es)).toEqual(keys)
  })

  it('defaults product copy to Portuguese in pt-BR', () => {
    expect(ptBR.auth.login.google).toMatch(/Google/)
    expect(ptBR.auth.deactivate.irreversible).toBeTruthy()
  })
})
