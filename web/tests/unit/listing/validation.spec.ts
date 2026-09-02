import { describe, expect, it } from 'vitest'
import {
  canPublishDraft,
  emptyListingForm,
  listingToForm,
  toCreateInput,
  validateListingDraft
} from '../../../app/composables/listing/validation'
import type { CategoryConfig, OwnerOnboarding } from '../../../app/composables/listing/types'

const readyOwner: OwnerOnboarding = {
  payout_set: true,
  terms_accepted: true,
  terms_version: 'v1',
  payout_kind: 'pix',
  payout_last4: '1234'
}

const categories: CategoryConfig[] = [
  { category: 'manual', size: 'light', deposit_min_cents: 5000 },
  { category: 'heavy', size: 'heavy', deposit_min_cents: 100000 }
]

function formWith(overrides: Partial<ReturnType<typeof emptyListingForm>> = {}) {
  return { ...emptyListingForm(), ...overrides }
}

describe('listing draft validation', () => {
  it('passes a complete manual light listing', () => {
    const form = formWith({
      title: 'Furadeira Bosch',
      description: 'Furadeira de bancada 750W com maleta e brocas.',
      pickup_city: 'São Paulo',
      photos: ['https://cdn.example.com/photo-1.jpg'],
      price_amount_cents: 15000,
      deposit_cents: 5000,
      min_lead_time_hours: 24
    })
    const result = validateListingDraft(form, {
      owner: readyOwner,
      categories
    })
    expect(result.ok).toBe(true)
    expect(result.value?.category).toBe('manual')
  })

  it('refuses a title shorter than 4 chars (EC-1-ish)', () => {
    const form = formWith({
      title: 'ab',
      photos: ['p.jpg'],
      price_amount_cents: 1000,
      deposit_cents: 5000
    })
    const result = validateListingDraft(form, {
      owner: readyOwner,
      categories
    })
    expect(result.ok).toBe(false)
    expect(result.errors?.title).toBeDefined()
  })

  it('refuses when no photo is provided (EC-1)', () => {
    const form = formWith({
      title: 'Furadeira Bosch',
      photos: [],
      price_amount_cents: 1000,
      deposit_cents: 5000
    })
    const result = validateListingDraft(form, {
      owner: readyOwner,
      categories
    })
    expect(result.ok).toBe(false)
    expect(result.errors?.photos).toBeDefined()
  })

  it('refuses deposit below category minimum (EC-2)', () => {
    const form = formWith({
      title: 'Furadeira Bosch',
      photos: ['p.jpg'],
      price_amount_cents: 1000,
      deposit_cents: 1000,
      category: 'manual'
    })
    const result = validateListingDraft(form, {
      owner: readyOwner,
      categories
    })
    expect(result.ok).toBe(false)
    expect(result.errors?.deposit_cents).toBe('draft.error.deposit_below_minimum')
  })

  it('refuses heavy without legal cession (EC-3)', () => {
    const form = formWith({
      title: 'Trator John Deere 6110J',
      description: 'Trator de esteira para obra pesada com cabine.',
      photos: ['p.jpg'],
      price_amount_cents: 250000,
      deposit_cents: 100000,
      category: 'heavy',
      heavy_legal_cession: false
    })
    const result = validateListingDraft(form, {
      owner: readyOwner,
      categories
    })
    expect(result.ok).toBe(false)
    expect(result.errors?.heavy_legal_cession).toBeDefined()
  })

  it('refuses required operator without identity (EC-4)', () => {
    const form = formWith({
      title: 'Escavadeira Komatsu',
      description: 'Escavadeira hidráulica para terraplanagem.',
      photos: ['p.jpg'],
      price_amount_cents: 200000,
      deposit_cents: 100000,
      category: 'heavy',
      heavy_legal_cession: true,
      operator_mode: 'required',
      operator_hourly_rate_cents: 8000,
      operator_name: ''
    })
    const result = validateListingDraft(form, {
      owner: readyOwner,
      categories
    })
    expect(result.ok).toBe(false)
    expect(result.errors?.operator_identity).toBeDefined()
  })

  it('refuses delivery enabled without coverage (EC-8)', () => {
    const form = formWith({
      title: 'Furadeira',
      photos: ['p.jpg'],
      price_amount_cents: 1000,
      deposit_cents: 5000,
      delivery_enabled: true,
      delivery_coverage: ''
    })
    const result = validateListingDraft(form, {
      owner: readyOwner,
      categories
    })
    expect(result.ok).toBe(false)
    expect(result.errors?.delivery_coverage).toBeDefined()
  })

  it('collects multiple gate failures in a single pass (Decisão 4)', () => {
    const form = emptyListingForm()
    const result = validateListingDraft(form, {
      owner: readyOwner,
      categories
    })
    expect(result.ok).toBe(false)
    const errorKeys = Object.keys(result.errors ?? {})
    expect(errorKeys).toContain('title')
    expect(errorKeys).toContain('description')
    expect(errorKeys).toContain('pickup_city')
    expect(errorKeys).toContain('price_amount_cents')
    expect(errorKeys).toContain('photos')
  })

  it('blocks publish when terms were not accepted', () => {
    const form = formWith({
      title: 'Furadeira Bosch',
      photos: ['p.jpg'],
      price_amount_cents: 15000,
      deposit_cents: 5000
    })
    const gate = canPublishDraft(form, {
      owner: { ...readyOwner, terms_accepted: false },
      categories
    })
    expect(gate.ok).toBe(false)
    expect(gate.reason).toBe('publish.error.terms_not_accepted')
  })

  it('blocks publish when payout is not set', () => {
    const form = formWith({
      title: 'Furadeira Bosch',
      photos: ['p.jpg'],
      price_amount_cents: 15000,
      deposit_cents: 5000
    })
    const gate = canPublishDraft(form, {
      owner: { ...readyOwner, payout_set: false },
      categories
    })
    expect(gate.ok).toBe(false)
    expect(gate.reason).toBe('publish.error.payout_not_set')
  })
})

describe('listing form helpers', () => {
  it('round-trips listingToForm / toCreateInput for a complete listing', () => {
    const listing = {
      title: 'Furadeira',
      description: 'Furadeira de bancada 750W com maleta.',
      category: 'manual' as const,
      pickup_city: 'São Paulo',
      pickup_neighborhood: 'Vila Mariana',
      delivery: { enabled: false },
      price_unit: 'day' as const,
      price_amount_cents: 15000,
      deposit_cents: 5000,
      min_lead_time_hours: 24,
      photos: ['p1.jpg'],
      rules: { document_required: true, min_age: 21 },
      operator: { mode: 'none' as const },
      heavy_legal_cession: false
    }
    const form = listingToForm(listing)
    expect(form.title).toBe('Furadeira')
    expect(form.rules_document_required).toBe(true)
    expect(form.rules_min_age).toBe(21)
    const input = toCreateInput(form)
    expect(input.title).toBe('Furadeira')
    expect(input.delivery.enabled).toBe(false)
  })

  it('omits operator identity when mode is none', () => {
    const form = formWith({ operator_mode: 'none' })
    const input = toCreateInput(form)
    expect(input.operator.identity).toBeUndefined()
    expect(input.operator.hourly_rate_cents).toBeUndefined()
  })

  it('keeps operator identity when mode is required', () => {
    const form = formWith({
      operator_mode: 'required',
      operator_name: 'João',
      operator_phone: '+5511...',
      operator_is_owner: true,
      operator_hourly_rate_cents: 8000,
      operator_min_hours: 4
    })
    const input = toCreateInput(form)
    expect(input.operator.identity?.name).toBe('João')
    expect(input.operator.identity?.is_owner).toBe(true)
    expect(input.operator.hourly_rate_cents).toBe(8000)
  })

  it('includes heavy legal cession only for heavy category', () => {
    const light = formWith({ category: 'manual', heavy_legal_cession: true })
    const heavy = formWith({ category: 'heavy', heavy_legal_cession: true })
    expect(toCreateInput(light).heavy_legal_cession).toBeUndefined()
    expect(toCreateInput(heavy).heavy_legal_cession).toBe(true)
  })
})
