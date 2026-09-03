import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ListingRequestError, type ListingClient } from '../../composables/listing/client'
import type {
  CreateListingInput,
  Listing,
  OwnerOnboarding,
  UpdateListingInput
} from '../../composables/listing/types'

type OwnerOnboardingPayload = {
  payout_kind?: string
  payout_last4?: string
  accept_terms?: boolean
  terms_version?: string
}

export const useOwnerListingsStore = defineStore('listing-owner', () => {
  const items = ref<Listing[]>([])
  const current = ref<Listing | null>(null)
  const onboarding = ref<OwnerOnboarding | null>(null)
  const pending = ref(false)
  const saving = ref(false)
  const errorKey = ref<string | null>(null)
  const publishGates = ref<string[]>([])

  async function loadMine(client: ListingClient): Promise<void> {
    pending.value = true
    errorKey.value = null
    try {
      items.value = await client.listMine()
    } catch (err) {
      errorKey.value = translateError(err)
    } finally {
      pending.value = false
    }
  }

  async function loadOne(client: ListingClient, id: string): Promise<void> {
    pending.value = true
    errorKey.value = null
    current.value = null
    try {
      current.value = await client.getMine(id)
    } catch (err) {
      errorKey.value = translateError(err)
    } finally {
      pending.value = false
    }
  }

  async function loadOnboarding(client: ListingClient): Promise<void> {
    try {
      onboarding.value = await client.getOwnerOnboarding()
    } catch (err) {
      onboarding.value = null
      errorKey.value = translateError(err)
    }
  }

  async function saveOnboarding(
    client: ListingClient,
    body: OwnerOnboardingPayload
  ): Promise<void> {
    saving.value = true
    errorKey.value = null
    try {
      onboarding.value = await client.updateOwnerOnboarding(body)
    } catch (err) {
      errorKey.value = translateError(err)
    } finally {
      saving.value = false
    }
  }

  async function create(
    client: ListingClient,
    input: CreateListingInput
  ): Promise<Listing | null> {
    saving.value = true
    errorKey.value = null
    try {
      const created = await client.createDraft(input)
      items.value = [created, ...items.value]
      current.value = created
      return created
    } catch (err) {
      errorKey.value = translateError(err)
      publishGates.value = extractGateKeys(err)
      return null
    } finally {
      saving.value = false
    }
  }

  async function update(
    client: ListingClient,
    id: string,
    input: UpdateListingInput
  ): Promise<Listing | null> {
    saving.value = true
    errorKey.value = null
    try {
      const updated = await client.updateListing(id, input)
      current.value = updated
      items.value = items.value.map(item => item.id === id ? updated : item)
      return updated
    } catch (err) {
      errorKey.value = translateError(err)
      publishGates.value = extractGateKeys(err)
      return null
    } finally {
      saving.value = false
    }
  }

  async function publish(
    client: ListingClient,
    id: string
  ): Promise<Listing | null> {
    saving.value = true
    errorKey.value = null
    publishGates.value = []
    try {
      const updated = await client.publish(id)
      current.value = updated
      items.value = items.value.map(item => item.id === id ? updated : item)
      return updated
    } catch (err) {
      errorKey.value = translateError(err)
      publishGates.value = extractGateKeys(err)
      return null
    } finally {
      saving.value = false
    }
  }

  async function pause(
    client: ListingClient,
    id: string
  ): Promise<Listing | null> {
    saving.value = true
    errorKey.value = null
    try {
      const updated = await client.pause(id)
      current.value = updated
      items.value = items.value.map(item => item.id === id ? updated : item)
      return updated
    } catch (err) {
      errorKey.value = translateError(err)
      return null
    } finally {
      saving.value = false
    }
  }

  function clearError() {
    errorKey.value = null
    publishGates.value = []
  }

  function resetCurrent() {
    current.value = null
    errorKey.value = null
    publishGates.value = []
  }

  return {
    items,
    current,
    onboarding,
    pending,
    saving,
    errorKey,
    publishGates,
    loadMine,
    loadOne,
    loadOnboarding,
    saveOnboarding,
    create,
    update,
    publish,
    pause,
    clearError,
    resetCurrent
  }
})

function translateError(err: unknown): string {
  if (err instanceof ListingRequestError) {
    if (err.status === 401) {
      return 'listing.error.unauthorized'
    }
    if (err.status === 403) {
      return 'listing.error.forbidden'
    }
    if (err.status === 404) {
      return 'listing.error.not_found'
    }
    if (err.status === 409) {
      return 'listing.error.conflict'
    }
    if (err.status === 422) {
      return 'listing.error.gates_failed'
    }
  }
  return 'listing.error.generic'
}

function extractGateKeys(err: unknown): string[] {
  if (err instanceof ListingRequestError && err.fields) {
    return err.fields
  }
  return []
}
