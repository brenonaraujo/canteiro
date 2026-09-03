import type {
  CategoryConfig,
  Listing,
  ListingDraftErrors,
  ListingFormResult,
  ListingFormState,
  OwnerOnboarding
} from './types'
import { depositMinimumFor } from './types'

// Skill: pre-implementation-design — Pilar 2 (gate enumeration).
// Choice: ONE orchestrator (validateListingDraft, ≤ 35 lines) delegates to
// 4 helpers grouped by the source of the gate (content, category, operator,
// delivery). The orchestrator collects ALL failures into a single map per
// Decisão 4 — fail-aggregating, not fail-fast. Decomposition rejected:
// 7 single-line helpers that only delegate add glue without conceptual
// boundary (anti-pattern "split for compliance"). Splitting by group gives
// each helper a real boundary that maps to a Pilar 2 sub-clause.

export type DraftGateContext = {
  owner: OwnerOnboarding | null
  categories: CategoryConfig[]
}

export function validateListingDraft(
  form: ListingFormState,
  context: DraftGateContext
): ListingFormResult {
  const errors: ListingDraftErrors = {}
  collectContentGates(form, context, errors)
  collectDeliveryGates(form, errors)
  collectCategoryGates(form, errors)
  collectOperatorGates(form, errors)
  if (Object.keys(errors).length > 0) {
    return { ok: false, errors }
  }
  return { ok: true, value: toCreateInput(form) }
}

function collectContentGates(
  form: ListingFormState,
  context: DraftGateContext,
  errors: ListingDraftErrors
): void {
  if (form.title.trim().length < 4 || form.title.length > 120) {
    errors.title = 'draft.error.title_length'
  }
  if (form.description.trim().length < 12 || form.description.length > 4000) {
    errors.description = 'draft.error.description_length'
  }
  const city = form.pickup_city.trim()
  if (city.length === 0 || city.length > 80) {
    errors.pickup_city = 'draft.error.pickup_city_required'
  }
  if (form.pickup_neighborhood.length > 80) {
    errors.pickup_neighborhood = 'draft.error.pickup_neighborhood_length'
  }
  if (!Number.isFinite(form.price_amount_cents)
    || form.price_amount_cents <= 0) {
    errors.price_amount_cents = 'draft.error.price_amount_required'
  }
  const depositMin = depositMinimumFor(context.categories, form.category)
  if (!Number.isFinite(form.deposit_cents) || form.deposit_cents < 0) {
    errors.deposit_cents = 'draft.error.deposit_required'
  } else if (form.deposit_cents < depositMin) {
    errors.deposit_cents = 'draft.error.deposit_below_minimum'
  }
  if (!Number.isFinite(form.min_lead_time_hours)
    || form.min_lead_time_hours < 0) {
    errors.min_lead_time_hours = 'draft.error.lead_time_invalid'
  }
  if (form.photos.length === 0) {
    errors.photos = 'draft.error.photo_required'
  }
}

function collectDeliveryGates(
  form: ListingFormState,
  errors: ListingDraftErrors
): void {
  if (form.delivery_enabled && form.delivery_coverage.trim().length === 0) {
    errors.delivery_coverage = 'draft.error.delivery_coverage_required'
  }
}

function collectCategoryGates(
  form: ListingFormState,
  errors: ListingDraftErrors
): void {
  if (form.category === 'heavy' && !form.heavy_legal_cession) {
    errors.heavy_legal_cession = 'draft.error.heavy_cession_required'
  }
}

function collectOperatorGates(
  form: ListingFormState,
  errors: ListingDraftErrors
): void {
  if (form.operator_mode === 'required'
    && form.operator_name.trim().length === 0) {
    errors.operator_identity = 'draft.error.operator_identity_required'
  }
  const needsRate = form.operator_mode === 'required'
    || (form.operator_mode === 'optional'
      && form.operator_name.trim().length > 0)
  if (needsRate
    && (!Number.isFinite(form.operator_hourly_rate_cents)
      || form.operator_hourly_rate_cents <= 0)) {
    errors.operator_hourly_rate_cents = 'draft.error.operator_rate_required'
  }
}

export function canPublishDraft(
  form: ListingFormState,
  context: DraftGateContext
): { ok: boolean, reason?: string } {
  if (!context.owner) {
    return { ok: false, reason: 'publish.error.no_session' }
  }
  if (!context.owner.terms_accepted) {
    return { ok: false, reason: 'publish.error.terms_not_accepted' }
  }
  if (!context.owner.payout_set) {
    return { ok: false, reason: 'publish.error.payout_not_set' }
  }
  const draft = validateListingDraft(form, context)
  if (!draft.ok) {
    return { ok: false, reason: 'publish.error.gates_failed' }
  }
  return { ok: true }
}

export function toCreateInput(form: ListingFormState) {
  return {
    title: form.title.trim(),
    description: form.description.trim(),
    category: form.category,
    pickup_city: form.pickup_city.trim(),
    pickup_neighborhood: form.pickup_neighborhood.trim(),
    delivery: {
      enabled: form.delivery_enabled,
      coverage: form.delivery_enabled
        ? form.delivery_coverage.trim()
        : undefined
    },
    price_unit: form.price_unit,
    price_amount_cents: form.price_amount_cents,
    deposit_cents: form.deposit_cents,
    min_lead_time_hours: form.min_lead_time_hours,
    photos: [...form.photos],
    rules: {
      document_required: form.rules_document_required,
      min_age: form.rules_min_age,
      experience_required: form.rules_experience_required,
      travel_restricted: form.rules_travel_restricted
    },
    operator: {
      mode: form.operator_mode,
      hourly_rate_cents: form.operator_mode === 'none'
        ? undefined
        : form.operator_hourly_rate_cents,
      min_hours: form.operator_mode === 'none'
        ? undefined
        : Math.max(1, form.operator_min_hours),
      identity: form.operator_mode === 'required'
        ? {
            name: form.operator_name.trim(),
            phone: form.operator_phone.trim(),
            is_owner: form.operator_is_owner
          }
        : undefined
    },
    heavy_legal_cession: form.category === 'heavy'
      ? form.heavy_legal_cession
      : undefined
  }
}

export function emptyListingForm(): ListingFormState {
  return {
    title: '',
    description: '',
    category: 'manual',
    size: 'light',
    pickup_city: '',
    pickup_neighborhood: '',
    delivery_enabled: false,
    delivery_coverage: '',
    price_unit: 'day',
    price_amount_cents: 0,
    deposit_cents: 0,
    min_lead_time_hours: 24,
    photos: [],
    rules_document_required: false,
    rules_min_age: 18,
    rules_experience_required: false,
    rules_travel_restricted: false,
    operator_mode: 'none',
    operator_hourly_rate_cents: 0,
    operator_min_hours: 4,
    operator_name: '',
    operator_phone: '',
    operator_is_owner: false,
    heavy_legal_cession: false
  }
}

export function listingToForm(listing: Listing): ListingFormState {
  return {
    title: listing.title,
    description: listing.description,
    category: listing.category,
    size: listing.category === 'heavy' ? 'heavy' : 'light',
    pickup_city: listing.pickup_city,
    pickup_neighborhood: listing.pickup_neighborhood ?? '',
    delivery_enabled: listing.delivery.enabled,
    delivery_coverage: listing.delivery.coverage ?? '',
    price_unit: listing.price_unit,
    price_amount_cents: listing.price_amount_cents,
    deposit_cents: listing.deposit_cents,
    min_lead_time_hours: listing.min_lead_time_hours,
    photos: [...listing.photos],
    rules_document_required: listing.rules.document_required ?? false,
    rules_min_age: listing.rules.min_age ?? 18,
    rules_experience_required: listing.rules.experience_required ?? false,
    rules_travel_restricted: listing.rules.travel_restricted ?? false,
    operator_mode: listing.operator.mode,
    operator_hourly_rate_cents: listing.operator.hourly_rate_cents ?? 0,
    operator_min_hours: listing.operator.min_hours ?? 4,
    operator_name: listing.operator.identity?.name ?? '',
    operator_phone: listing.operator.identity?.phone ?? '',
    operator_is_owner: listing.operator.identity?.is_owner ?? false,
    heavy_legal_cession: listing.heavy_legal_cession ?? false
  }
}
