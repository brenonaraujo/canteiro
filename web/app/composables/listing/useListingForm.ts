import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  canPublishDraft,
  emptyListingForm,
  listingToForm,
  validateListingDraft,
  type DraftGateContext
} from './validation'
import type {
  ListingDraftErrors,
  ListingFormState
} from './types'

export function useListingForm(initial?: ListingFormState) {
  const state = reactive<ListingFormState>(initial ?? emptyListingForm())
  const { locale } = useI18n()

  const context = reactive<DraftGateContext>({
    owner: null,
    categories: []
  })

  const validation = computed(() => validateListingDraft(state, context))
  const errors = computed<ListingDraftErrors>(() => validation.value.errors ?? {})
  const isValid = computed(() => validation.value.ok)

  const publishGate = computed(() => canPublishDraft(state, context))

  const isHeavy = computed(() => state.category === 'heavy')
  const needsOperatorIdentity = computed(() => state.operator_mode === 'required')
  const needsOperatorRate = computed(() => state.operator_mode !== 'none')

  function hydrateFromListing(listing: Parameters<typeof listingToForm>[0]) {
    Object.assign(state, listingToForm(listing))
  }

  function hydrate(next: ListingFormState) {
    Object.assign(state, next)
  }

  function reset() {
    Object.assign(state, emptyListingForm())
  }

  function setOwner(value: DraftGateContext['owner']) {
    context.owner = value
  }

  function setCategories(value: DraftGateContext['categories']) {
    context.categories = value
  }

  // Locale change should clear transient string errors so they re-resolve.
  watch(locale, () => {
    if (validation.value.ok) {
      return
    }
  })

  return {
    state,
    errors,
    isValid,
    publishGate,
    isHeavy,
    needsOperatorIdentity,
    needsOperatorRate,
    hydrate,
    hydrateFromListing,
    reset,
    setOwner,
    setCategories
  }
}
