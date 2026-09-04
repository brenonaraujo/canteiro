import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function readApp(path: string): string {
  return readFileSync(join(process.cwd(), 'app', path), 'utf8')
}

describe('auth surface', () => {
  it('offers Google sign-in without a password field', () => {
    const login = readApp('pages/auth/login.vue')
    expect(login).toContain('t(\'auth.login.google\')')
    expect(login.toLowerCase()).not.toContain('password')
    expect(login.toLowerCase()).not.toContain('type="password"')
  })

  it('shows a human alert on login when Google is not configured', () => {
    const login = readApp('pages/auth/login.vue')
    expect(login).toContain('v-if="errorKey"')
    expect(login).toContain(':title="t(errorKey)"')
    expect(login).toContain('UAlert')
    expect(login.toLowerCase()).not.toContain('application/json')
  })

  it('completes profile on a dedicated page with name and phone', () => {
    const profile = readApp('pages/auth/profile.vue')
    expect(profile).toContain('t(\'auth.profile.display_name\')')
    expect(profile).toContain('t(\'auth.profile.phone\')')
    expect(profile).not.toMatch(/UModal/)
  })

  it('does not render profile or deactivate forms without a session view gate', () => {
    const profile = readApp('pages/auth/profile.vue')
    const deactivate = readApp('pages/auth/deactivate.vue')
    expect(profile).toContain('v-if="view === \'form\'"')
    expect(profile).toContain('guestRedirectTarget')
    expect(deactivate).toContain('v-if="view === \'form\'"')
    expect(deactivate).toContain('guestRedirectTarget')
  })

  it('deactivates from a dedicated confirmation page, not a long modal', () => {
    const page = readApp('pages/auth/deactivate.vue')
    expect(page).toContain('t(\'auth.deactivate.submit\')')
    expect(page).toContain('t(\'auth.deactivate.irreversible\')')
    expect(page).not.toMatch(/UModal/)
  })

  it('shows visible name, sign-in and sign-out in the public chrome', () => {
    const menu = readApp('components/common/AccountMenu.vue')
    expect(menu).toContain('t(\'nav.sign_in\')')
    expect(menu).toContain('t(\'nav.sign_out\')')
    expect(menu).toContain('to="/auth/login"')
  })
})
