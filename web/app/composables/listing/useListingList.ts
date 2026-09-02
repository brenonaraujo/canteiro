import { storeToRefs } from 'pinia'
import { resolveApiBase } from '../auth/actions'
import { resolveListingClient } from './api'
import { useCatalogStore } from '../../stores/listing/catalog'

export function useListingList() {
  const store = useCatalogStore()
  const refs = storeToRefs(store)
  const run = () => bindCatalog(store, useRuntimeConfig())
  return {
    ...refs,
    search: (filters: Parameters<typeof store.search>[0]) => run().search(filters),
    loadCategories: () => run().loadCategories()
  }
}

function bindCatalog(
  store: ReturnType<typeof useCatalogStore>,
  config: { public: { apiBase: string } }
) {
  const apiBase = resolveApiBase(String(config.public.apiBase || ''), import.meta.dev)
  const { client } = resolveListingClient({
    apiBase,
    fetch: (input, init) => globalThis.fetch(input, init)
  })
  return {
    search: (filters: Parameters<typeof store.search>[0]) =>
      store.search(filters, client),
    loadCategories: () => store.loadCategories(client)
  }
}
