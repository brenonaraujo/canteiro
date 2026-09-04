import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function readApp(path: string): string {
  return readFileSync(join(process.cwd(), 'app', path), 'utf8')
}

describe('home surface (#26)', () => {
  it('composes a live vitrine, not a pre-launch placeholder block', () => {
    const page = readApp('pages/index.vue')
    expect(page).toContain('HomeHero')
    expect(page).toContain('HomeHighlights')
    expect(page).toContain('HomeMarket')
    expect(page).not.toContain('HomeSoon')
    expect(page).not.toContain('definePageMeta({ middleware: \'auth\'')
  })

  it('sends the primary hero action to public catalog search', () => {
    const hero = readApp('components/feature/home/HomeHero.vue')
    expect(hero).toContain('to: \'/listings\'')
    expect(hero).not.toContain('#soon')
    expect(hero).toContain('landing.hero.cta')
    expect(hero).toContain('min-h-11')
  })

  it('names the three marketplace roles on the entry', () => {
    const highlights = readApp('components/feature/home/HomeHighlights.vue')
    expect(highlights).toContain('landing.highlights.owner')
    expect(highlights).toContain('landing.highlights.renter')
    expect(highlights).toContain('landing.highlights.platform')
  })

  it('evidences a live market with real listings or an empty invite, never fixtures', () => {
    const market = readApp('components/feature/home/HomeMarket.vue')
    expect(market).toContain('useListingList')
    expect(market).toContain('landing.market.empty_title')
    expect(market).toContain('to="/listings"')
    expect(market.toLowerCase()).not.toContain('depoimento')
    expect(market.toLowerCase()).not.toContain('testimonial')
    expect(market).not.toMatch(/João|Maria|John Doe/)
  })

  it('does not disguise a catalog fetch error as an empty yard', () => {
    const market = readApp('components/feature/home/HomeMarket.vue')
    expect(market).toContain('errorKey')
    expect(market).toContain('landing.market.error_title')
    expect(market).toContain('landing.market.retry')
    expect(market).toContain('t(errorKey)')
  })
})
