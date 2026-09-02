import { ACCESS_TOKEN_KEY, type TokenStorage } from '../auth/session'
import type {
  AvailabilityBlock,
  CategoryConfig,
  CreateListingInput,
  Listing,
  ListingPage,
  ListingSearchFilters,
  OwnerOnboarding,
  PublicCalendar,
  PublicListing,
  UpdateListingInput
} from './types'

export type ListingApiError = {
  status: number
  message_key?: string
  fields?: string[]
}

export class ListingRequestError extends Error {
  status: number
  messageKey?: string
  fields?: string[]

  constructor(
    status: number,
    messageKey?: string,
    fields?: string[]
  ) {
    super(`listing_request_failed_${status}`)
    this.name = 'ListingRequestError'
    this.status = status
    this.messageKey = messageKey
    this.fields = fields
  }
}

export type FetchLike = (
  input: string,
  init?: RequestInit
) => Promise<Response>

export type ListingClient = {
  searchCatalog: (filters: ListingSearchFilters) => Promise<ListingPage>
  getPublicListing: (id: string) => Promise<PublicListing>
  getPublicCalendar: (
    id: string,
    range?: { from?: string, to?: string }
  ) => Promise<PublicCalendar>
  listCategories: () => Promise<CategoryConfig[]>
  listMine: () => Promise<Listing[]>
  getMine: (id: string) => Promise<Listing>
  createDraft: (input: CreateListingInput) => Promise<Listing>
  updateListing: (id: string, input: UpdateListingInput) => Promise<Listing>
  publish: (id: string) => Promise<Listing>
  pause: (id: string) => Promise<Listing>
  listBlocks: (id: string) => Promise<AvailabilityBlock[]>
  addBlock: (
    id: string,
    body: { starts_at: string, ends_at: string, reason?: string }
  ) => Promise<AvailabilityBlock>
  removeBlock: (id: string, blockId: string) => Promise<void>
  getOwnerOnboarding: () => Promise<OwnerOnboarding>
  updateOwnerOnboarding: (
    body: {
      payout_kind?: string
      payout_last4?: string
      accept_terms?: boolean
      terms_version?: string
    }
  ) => Promise<OwnerOnboarding>
}

export type ListingClientOptions = {
  apiBase: string
  storage: TokenStorage
  fetch: FetchLike
}

export function createListingClient(opts: ListingClientOptions): ListingClient {
  async function request(
    path: string,
    init: RequestInit = {}
  ): Promise<Response> {
    const headers = new Headers(init.headers)
    const token = opts.storage.getItem(ACCESS_TOKEN_KEY)
    if (token && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${token}`)
    }
    if (init.body && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json')
    }

    const response = await opts.fetch(resolveUrl(opts.apiBase, path), {
      ...init,
      headers,
      credentials: 'include'
    })

    if (response.status === 204) {
      return response
    }
    if (response.status === 401) {
      throw new ListingRequestError(401)
    }
    if (!response.ok) {
      const body = await safeJson(response)
      throw new ListingRequestError(
        response.status,
        body?.message_key,
        body?.fields
      )
    }
    return response
  }

  return {
    searchCatalog: async (filters) => {
      const query = toSearchQuery(filters)
      const response = await request(`/catalog/listings${query}`)
      return await response.json() as ListingPage
    },
    getPublicListing: async (id) => {
      const response = await request(`/catalog/listings/${id}`)
      return await response.json() as PublicListing
    },
    getPublicCalendar: async (id, range) => {
      const query = toCalendarQuery(range)
      const response = await request(
        `/catalog/listings/${id}/calendar${query}`
      )
      return await response.json() as PublicCalendar
    },
    listCategories: async () => {
      const response = await request('/catalog/categories')
      return await response.json() as CategoryConfig[]
    },
    listMine: async () => {
      const response = await request('/listings')
      return await response.json() as Listing[]
    },
    getMine: async (id) => {
      const response = await request(`/listings/${id}`)
      return await response.json() as Listing
    },
    createDraft: async (input) => {
      const response = await request('/listings', {
        method: 'POST',
        body: JSON.stringify(input)
      })
      return await response.json() as Listing
    },
    updateListing: async (id, input) => {
      const response = await request(`/listings/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input)
      })
      return await response.json() as Listing
    },
    publish: async (id) => {
      const response = await request(`/listings/${id}/publish`, {
        method: 'POST'
      })
      return await response.json() as Listing
    },
    pause: async (id) => {
      const response = await request(`/listings/${id}/pause`, {
        method: 'POST'
      })
      return await response.json() as Listing
    },
    listBlocks: async (id) => {
      const response = await request(`/listings/${id}/blocks`)
      return await response.json() as AvailabilityBlock[]
    },
    addBlock: async (id, body) => {
      const response = await request(`/listings/${id}/blocks`, {
        method: 'POST',
        body: JSON.stringify(body)
      })
      return await response.json() as AvailabilityBlock
    },
    removeBlock: async (id, blockId) => {
      await request(`/listings/${id}/blocks/${blockId}`, {
        method: 'DELETE'
      })
    },
    getOwnerOnboarding: async () => {
      const response = await request('/owner/onboarding')
      return await response.json() as OwnerOnboarding
    },
    updateOwnerOnboarding: async (body) => {
      const response = await request('/owner/onboarding', {
        method: 'PATCH',
        body: JSON.stringify(body)
      })
      return await response.json() as OwnerOnboarding
    }
  }
}

function resolveUrl(apiBase: string, path: string): string {
  const host = apiBase.replace(/\/$/, '')
  return host ? `${host}${path}` : path
}

function toSearchQuery(filters: ListingSearchFilters): string {
  const params = new URLSearchParams()
  appendIfPresent(params, 'category', filters.category)
  appendIfPresent(params, 'city', filters.city)
  appendIfPresent(params, 'from', filters.from)
  appendIfPresent(params, 'to', filters.to)
  appendIfPresent(params, 'operator_mode', filters.operator_mode)
  appendIfPresent(params, 'size', filters.size)
  appendIfPresent(params, 'min_price_cents', filters.min_price_cents)
  appendIfPresent(params, 'max_price_cents', filters.max_price_cents)
  appendIfPresent(params, 'page', filters.page ?? 1)
  const search = params.toString()
  return search ? `?${search}` : ''
}

function toCalendarQuery(range: { from?: string, to?: string } = {}): string {
  const params = new URLSearchParams()
  appendIfPresent(params, 'from', range.from)
  appendIfPresent(params, 'to', range.to)
  const search = params.toString()
  return search ? `?${search}` : ''
}

function appendIfPresent(
  params: URLSearchParams,
  key: string,
  value: string | number | undefined
): void {
  if (value === undefined || value === null || value === '') {
    return
  }
  params.set(key, String(value))
}

async function safeJson(response: Response): Promise<{
  message_key?: string
  fields?: string[]
} | null> {
  try {
    const body = await response.json() as Record<string, unknown>
    return {
      message_key: typeof body.message_key === 'string'
        ? body.message_key
        : undefined,
      fields: Array.isArray(body.fields)
        ? body.fields.filter((f): f is string => typeof f === 'string')
        : undefined
    }
  } catch {
    return null
  }
}
