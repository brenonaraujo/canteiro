import { ACCESS_TOKEN_KEY, type TokenStorage } from '../auth/session'
import type {
  Rental,
  RentalReceipt
} from './types'

export type RentalApiError = {
  status: number
  messageKey?: string
  fields?: string[]
}

export class RentalRequestError extends Error {
  status: number
  messageKey?: string
  fields?: string[]

  constructor(
    status: number,
    messageKey?: string,
    fields?: string[]
  ) {
    super(`rental_request_failed_${status}`)
    this.name = 'RentalRequestError'
    this.status = status
    this.messageKey = messageKey
    this.fields = fields
  }
}

export type FetchLike = (
  input: string,
  init?: RequestInit
) => Promise<Response>

export type RentalClient = {
  listMine: () => Promise<Rental[]>
  getMine: (id: string) => Promise<Rental>
  cancel: (id: string) => Promise<Rental>
  getReceipt: (id: string) => Promise<RentalReceipt>
}

export type RentalClientOptions = {
  apiBase: string
  storage: TokenStorage
  fetch: FetchLike
}

export function createRentalClient(opts: RentalClientOptions): RentalClient {
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
      throw new RentalRequestError(401)
    }
    if (!response.ok) {
      const body = await safeJson(response)
      throw new RentalRequestError(
        response.status,
        body?.message_key,
        body?.fields
      )
    }
    return response
  }

  return {
    listMine: async () => {
      const response = await request('/rentals')
      return await response.json() as Rental[]
    },
    getMine: async (id) => {
      const response = await request(`/rentals/${id}`)
      return await response.json() as Rental
    },
    cancel: async (id) => {
      const response = await request(`/rentals/${id}/cancel`, {
        method: 'POST'
      })
      return await response.json() as Rental
    },
    getReceipt: async (id) => {
      const response = await request(`/rentals/${id}/receipt`)
      return await response.json() as RentalReceipt
    }
  }
}

function resolveUrl(apiBase: string, path: string): string {
  const host = apiBase.replace(/\/$/, '')
  return host ? `${host}${path}` : path
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

export function translateRentalError(err: unknown): string {
  if (!(err instanceof RentalRequestError)) {
    return 'rental.error.generic'
  }
  if (err.status === 401) {
    return 'rental.error.unauthorized'
  }
  if (err.status === 403) {
    return 'rental.error.forbidden'
  }
  if (err.status === 404) {
    return 'rental.error.not_found'
  }
  if (err.status === 409) {
    return 'rental.error.conflict'
  }
  return 'rental.error.generic'
}
