import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function readApp(path: string): string {
  return readFileSync(join(process.cwd(), 'app', path), 'utf8')
}

describe('visitor catalog flow (#26)', () => {
  it('lets a guest reach public search from the entry without authenticating', () => {
    const home = readApp('pages/index.vue')
    const hero = readApp('components/feature/home/HomeHero.vue')
    const catalog = readApp('pages/listings/index.vue')
    expect(home).not.toContain('definePageMeta({ middleware: \'auth\'')
    expect(hero).toContain('to: \'/listings\'')
    expect(catalog).not.toContain('definePageMeta({ middleware: \'auth\'')
    expect(catalog).toContain('ListingCard')
  })

  it('opens a public ficha without contact or other-renter identity', () => {
    const detail = readApp('pages/listings/[id].vue')
    const ficha = readApp('components/feature/listing/ListingFicha.vue')
    expect(detail).not.toContain('definePageMeta({ middleware: \'auth\'')
    expect(detail).toContain('ListingFicha')
    expect(ficha.toLowerCase()).not.toContain('owner_account_id')
    expect(ficha.toLowerCase()).not.toContain('phone')
    expect(ficha.toLowerCase()).not.toContain('contact')
  })
})
