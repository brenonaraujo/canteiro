import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ListingRequestError, type ListingClient } from '../../composables/listing/client'
import type {
  CategoryConfig,
  ListingPage,
  ListingSearchFilters
} from '../../composables/listing/types'

export const useCatalogStore = defineStore('listing-catalog', () => {
  const page = ref<ListingPage | null>(null)
  const categories = ref<CategoryConfig[]>([])
  const pending = ref(false)
  const errorKey = ref<string | null>(null)
  const lastFilters = ref<ListingSearchFilters>({})

  async function search(
    filters: ListingSearchFilters,
    client: ListingClient
  ): Promise<void> {
    pending.value = true
    errorKey.value = null
    lastFilters.value = filters
    try {
      page.value = await client.searchCatalog(filters)
    } catch (err) {
      errorKey.value = translateError(err)
    } finally {
      pending.value = false
    }
  }

  async function loadCategories(client: ListingClient): Promise<void> {
    try {
      categories.value = await client.listCategories()
    } catch {
      categories.value = []
    }
  }

  function clearError() {
    errorKey.value = null
  }

  function reset() {
    page.value = null
    errorKey.value = null
    pending.value = false
    lastFilters.value = {}
  }

  return {
    page,
    categories,
    pending,
    errorKey,
    lastFilters,
    search,
    loadCategories,
    clearError,
    reset
  }
})

function translateError(err: unknown): string {
  if (err instanceof ListingRequestError && err.status === 404) {
    return 'listing.error.not_found'
  }
  return 'listing.error.search_failed'
}
