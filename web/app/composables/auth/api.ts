import type { Account, AccountStatus, ProfileInput } from './account'
import { normalizeStatus } from './account'

export type AuthResource = 'me' | 'logout' | 'deactivate'

export type AccountApi = {
  id: string
  status: AccountStatus | 'incomplete_profile'
  display_name?: string | null
  phone?: string | null
  displayName?: string | null
}

export function authPath(resource: AuthResource): string {
  if (resource === 'logout') {
    return '/auth/logout'
  }
  if (resource === 'deactivate') {
    return '/account/deactivate'
  }
  return '/account'
}

export function googleStartUrl(apiBase: string): string {
  const path = '/auth/google'
  const host = apiBase.replace(/\/$/, '')
  return host ? `${host}${path}` : path
}

export function accountFromApi(payload: AccountApi): Account {
  return {
    id: payload.id,
    status: normalizeStatus(payload.status),
    displayName: payload.display_name ?? payload.displayName ?? '',
    phone: payload.phone ?? ''
  }
}

export function profilePayload(input: ProfileInput): {
  display_name: string
  phone: string
} {
  return {
    display_name: input.displayName.trim(),
    phone: input.phone.trim()
  }
}

export function deactivatePayload(): { confirm: true } {
  return { confirm: true }
}
