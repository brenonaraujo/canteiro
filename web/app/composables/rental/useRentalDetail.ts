import { storeToRefs } from 'pinia'
import { resolveApiBase } from '../auth/actions'
import { resolveRentalClient } from './api'
import { useRentalDetailStore } from '../../stores/rental/detail'

export function useRentalDetail() {
  const store = useRentalDetailStore()
  const refs = storeToRefs(store)
  const run = () => bindDetail(store, useRuntimeConfig())
  return {
    ...refs,
    load: (id: string) => run().load(id),
    loadReceipt: (id: string) => run().loadReceipt(id),
    cancel: (id: string) => run().cancel(id),
    clearCancelError: () => store.clearCancelError(),
    reset: () => store.reset()
  }
}

function bindDetail(
  store: ReturnType<typeof useRentalDetailStore>,
  config: { public: { apiBase: string } }
) {
  const apiBase = resolveApiBase(String(config.public.apiBase || ''), import.meta.dev)
  const { client } = resolveRentalClient({
    apiBase,
    fetch: (input, init) => globalThis.fetch(input, init)
  })
  return {
    load: (id: string) => store.load(id, client),
    loadReceipt: (id: string) => store.loadReceipt(id, client),
    cancel: (id: string) => store.cancel(id, client)
  }
}
