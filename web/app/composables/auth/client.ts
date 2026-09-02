import type { Account, ProfileInput } from './account'
import { accountFromApi, authPath, deactivatePayload, profilePayload } from './api'
import {
  readAccessToken,
  type TokenStorage,
  writeAccessToken
} from './session'

export type FetchLike = (
  input: string,
  init?: RequestInit
) => Promise<Response>

export class AuthRequestError extends Error {
  status: number

  constructor(status: number, message = 'auth_request_failed') {
    super(message)
    this.name = 'AuthRequestError'
    this.status = status
  }
}

export function createAccountClient(opts: {
  apiBase: string
  storage: TokenStorage
  fetch: FetchLike
}) {
  async function request(
    path: string,
    init: RequestInit = {}
  ): Promise<Response> {
    const headers = new Headers(init.headers)
    const token = readAccessToken(opts.storage)
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

    if (response.status === 401) {
      writeAccessToken(opts.storage, null)
      throw new AuthRequestError(401)
    }
    if (!response.ok) {
      throw new AuthRequestError(response.status)
    }
    return response
  }

  async function fetchMe(): Promise<Account> {
    const response = await request(authPath('me'))
    return accountFromApi(await response.json())
  }

  async function saveProfile(input: ProfileInput): Promise<Account> {
    const response = await request(authPath('me'), {
      method: 'PATCH',
      body: JSON.stringify(profilePayload(input))
    })
    return accountFromApi(await response.json())
  }

  async function logout(): Promise<void> {
    await request(authPath('logout'), { method: 'POST' })
    writeAccessToken(opts.storage, null)
  }

  async function deactivate(): Promise<void> {
    await request(authPath('deactivate'), {
      method: 'POST',
      body: JSON.stringify(deactivatePayload())
    })
    writeAccessToken(opts.storage, null)
  }

  return { fetchMe, saveProfile, logout, deactivate }
}

function resolveUrl(apiBase: string, path: string): string {
  const host = apiBase.replace(/\/$/, '')
  return host ? `${host}${path}` : path
}
