import { storeToRefs } from 'pinia'
import { resolveApiBase } from '../auth/actions'
import { resolveListingClient } from './api'
import { useOwnerListingsStore } from '../../stores/listing/owner'
import type { CreateListingInput, UpdateListingInput } from './types'

export function useListingOwner() {
  const store = useOwnerListingsStore()
  const refs = storeToRefs(store)
  const run = () => bindOwner(store, useRuntimeConfig())
  return {
    ...refs,
    loadMine: () => run().loadMine(),
    loadOne: (id: string) => run().loadOne(id),
    loadOnboarding: () => run().loadOnboarding(),
    saveOnboarding: (body: Parameters<typeof store.saveOnboarding>[1]) =>
      run().saveOnboarding(body),
    create: (input: CreateListingInput) => run().create(input),
    update: (id: string, input: UpdateListingInput) => run().update(id, input),
    publish: (id: string) => run().publish(id),
    pause: (id: string) => run().pause(id)
  }
}

function bindOwner(
  store: ReturnType<typeof useOwnerListingsStore>,
  config: { public: { apiBase: string } }
) {
  const apiBase = resolveApiBase(String(config.public.apiBase || ''), import.meta.dev)
  const { client } = resolveListingClient({
    apiBase,
    fetch: (input, init) => globalThis.fetch(input, init)
  })
  return {
    loadMine: () => store.loadMine(client),
    loadOne: (id: string) => store.loadOne(client, id),
    loadOnboarding: () => store.loadOnboarding(client),
    saveOnboarding: (body: Parameters<typeof store.saveOnboarding>[1]) =>
      store.saveOnboarding(client, body),
    create: (input: CreateListingInput) => store.create(client, input),
    update: (id: string, input: UpdateListingInput) =>
      store.update(client, id, input),
    publish: (id: string) => store.publish(client, id),
    pause: (id: string) => store.pause(client, id)
  }
}
