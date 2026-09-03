import { browserStorage, type TokenStorage } from '../auth/session'
import { createListingClient, type FetchLike, type ListingClient } from './client'

const emptyStorage: TokenStorage = {
  getItem: () => null,
  setItem: () => {},
  removeItem: () => {}
}

export type BoundListingClient = {
  client: ListingClient
  storage: TokenStorage
}

export function resolveListingClient(opts: {
  apiBase: string
  fetch?: FetchLike
}): BoundListingClient {
  const storage = browserStorage() ?? emptyStorage
  const fetchImpl: FetchLike = opts.fetch
    ?? ((input, init) => globalThis.fetch(input, init))
  const client = createListingClient({
    apiBase: opts.apiBase,
    storage,
    fetch: fetchImpl
  })
  return { client, storage }
}
