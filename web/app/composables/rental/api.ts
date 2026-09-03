import { browserStorage, type TokenStorage } from '../auth/session'
import {
  createRentalClient,
  type FetchLike,
  type RentalClient
} from './client'

const emptyStorage: TokenStorage = {
  getItem: () => null,
  setItem: () => {},
  removeItem: () => {}
}

export type BoundRentalClient = {
  client: RentalClient
  storage: TokenStorage
}

export function resolveRentalClient(opts: {
  apiBase: string
  fetch?: FetchLike
}): BoundRentalClient {
  const storage = browserStorage() ?? emptyStorage
  const fetchImpl: FetchLike = opts.fetch
    ?? ((input, init) => globalThis.fetch(input, init))
  const client = createRentalClient({
    apiBase: opts.apiBase,
    storage,
    fetch: fetchImpl
  })
  return { client, storage }
}
