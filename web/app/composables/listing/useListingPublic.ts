import { ref } from 'vue'
import { resolveApiBase } from '../auth/actions'
import { resolveListingClient } from './api'
import type { ListingRequestError } from './client'
import type { PublicCalendar, PublicListing } from './types'

export function useListingPublic() {
  const listing = ref<PublicListing | null>(null)
  const calendar = ref<PublicCalendar | null>(null)
  const pending = ref(false)
  const errorKey = ref<string | null>(null)

  async function load(id: string) {
    const { client } = resolveListingClient({
      apiBase: resolveApiBase(
        String(useRuntimeConfig().public.apiBase || ''),
        import.meta.dev
      ),
      fetch: (input, init) => globalThis.fetch(input, init)
    })
    pending.value = true
    errorKey.value = null
    try {
      const [item, blocks] = await Promise.all([
        client.getPublicListing(id),
        client.getPublicCalendar(id)
      ])
      listing.value = item
      calendar.value = blocks
    } catch (err) {
      errorKey.value = translateError(err)
    } finally {
      pending.value = false
    }
  }

  function reset() {
    listing.value = null
    calendar.value = null
    errorKey.value = null
    pending.value = false
  }

  return { listing, calendar, pending, errorKey, load, reset }
}

function translateError(err: unknown): string {
  const reqErr = err as ListingRequestError | undefined
  if (!reqErr || typeof reqErr.status !== 'number') {
    return 'listing.error.generic'
  }
  if (reqErr.status === 404) {
    return 'listing.error.not_found'
  }
  return 'listing.error.generic'
}
