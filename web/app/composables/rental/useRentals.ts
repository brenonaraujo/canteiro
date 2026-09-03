import { storeToRefs } from 'pinia'
import { resolveApiBase } from '../auth/actions'
import { resolveRentalClient } from './api'
import { useRentalsListStore } from '../../stores/rental/list'
import type { Rental } from './types'

export function useRentals() {
  const store = useRentalsListStore()
  const refs = storeToRefs(store)
  const run = () => bindList(store, useRuntimeConfig())
  return {
    ...refs,
    load: () => run().load(),
    upsert: (rental: Rental) => store.upsert(rental),
    clearError: () => store.clearError(),
    reset: () => store.reset()
  }
}

function bindList(
  store: ReturnType<typeof useRentalsListStore>,
  config: { public: { apiBase: string } }
) {
  const apiBase = resolveApiBase(String(config.public.apiBase || ''), import.meta.dev)
  const { client } = resolveRentalClient({
    apiBase,
    fetch: (input, init) => globalThis.fetch(input, init)
  })
  return {
    load: () => store.load(client)
  }
}
