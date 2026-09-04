import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function readApp(path: string): string {
  return readFileSync(join(process.cwd(), 'app', path), 'utf8')
}

describe('access failure perception (#26)', () => {
  it('explains Google unavailability on the login journey without a dead end', () => {
    const login = readApp('pages/auth/login.vue')
    expect(login).toContain('auth.login.unavailable_title')
    expect(login).toContain('auth.login.unavailable_body')
    expect(login).toContain('not_configured')
    expect(login).toContain('to="/listings"')
    expect(login).toContain('auth.login.browse_catalog')
    expect(login).not.toMatch(/UModal/)
  })

  it('tells the visitor they did not enter when Google denies or errors', () => {
    const login = readApp('pages/auth/login.vue')
    expect(login).toContain('accessFailed')
    expect(login).toContain('auth.callback.error')
    expect(login).toContain('auth.login.still_visitor')
    expect(login).toContain('to="/listings"')
  })

  it('still offers Google as the v1 identity action', () => {
    const login = readApp('pages/auth/login.vue')
    expect(login).toContain('startGoogle')
    expect(login).toContain('auth.login.google')
    expect(login.toLowerCase()).not.toContain('type="password"')
  })
})
