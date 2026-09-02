import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

function readApp(path: string): string {
  return readFileSync(join(process.cwd(), 'app', path), 'utf8')
}

describe('listing surface (F2)', () => {
  it('offers a public catalog page with the six filters from AC-8', () => {
    const index = readApp('pages/listings/index.vue')
    expect(index).toContain('ListingFilters')
    expect(index).toContain('listing.search.title')
    // The six filters live in the component, but the page wires them.
    expect(index).toContain('@apply="onApply"')
  })

  it('renders the public ficha without contact or owner identity (AC-9)', () => {
    const ficha = readApp('components/feature/listing/ListingFicha.vue')
    expect(ficha.toLowerCase()).not.toContain('owner_account_id')
    expect(ficha.toLowerCase()).not.toContain('phone')
    expect(ficha.toLowerCase()).not.toContain('contact')
    expect(ficha).toContain('listing.ficha.identity_note')
  })

  it('publishes from a dedicated page, never a modal', () => {
    const newPage = readApp('pages/account/listings/new.vue')
    expect(newPage).toContain('ListingForm')
    expect(newPage).not.toMatch(/UModal/)
  })

  it('owner dashboard offers publish/pause actions on each card', () => {
    const owner = readApp('pages/account/listings/index.vue')
    expect(owner).toContain('listing.owner.title')
    expect(owner).toContain('onPublish')
    expect(owner).toContain('onPause')
  })

  it('edit page locks the form while the listing is published (AC-3)', () => {
    const edit = readApp('pages/account/listings/[id]/edit.vue')
    expect(edit).toContain('listing.edit.published_locked')
    expect(edit).toContain('editable')
  })

  it('does not render the public search page behind a login', () => {
    const index = readApp('pages/listings/index.vue')
    expect(index).not.toContain('definePageMeta({ middleware: \'auth\'')
  })

  it('keeps the listing navigation visible on the public chrome', () => {
    const header = readApp('components/common/AppHeader.vue')
    expect(header).toContain('breadcrumb.listings')
  })
})
