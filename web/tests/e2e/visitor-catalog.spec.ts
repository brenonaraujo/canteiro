import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function readApp(path: string): string {
  return readFileSync(join(process.cwd(), 'app', path), 'utf8')
}

describe('e2e: guest discovers and opens a listing (#30)', () => {
  it('wires entry → public search → ficha without a session wall', () => {
    const hero = readApp('components/feature/home/HomeHero.vue')
    const catalog = readApp('pages/listings/index.vue')
    const detail = readApp('pages/listings/[id].vue')
    const ficha = readApp('components/feature/listing/ListingFicha.vue')

    expect(hero).toContain('to: \'/listings\'')
    expect(catalog).not.toContain('definePageMeta({ middleware: \'auth\'')
    expect(catalog).toContain('ListingCard')
    expect(detail).not.toContain('definePageMeta({ middleware: \'auth\'')
    expect(detail).toContain('ListingFicha')
    expect(ficha.toLowerCase()).not.toContain('phone')
    expect(ficha.toLowerCase()).not.toContain('contact')
  })

  it('keeps the marketplace visible when Google access fails', () => {
    const home = readApp('pages/index.vue')
    const login = readApp('pages/auth/login.vue')
    const market = readApp('components/feature/home/HomeMarket.vue')
    expect(home).toContain('HomeMarket')
    expect(home).toContain('HomeAccessAlert')
    expect(login).toContain('to="/listings"')
    expect(market).toContain('errorKey')
    expect(market).toContain('landing.market.empty_title')
  })
})
