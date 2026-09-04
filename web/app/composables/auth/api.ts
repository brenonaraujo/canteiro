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

export type GoogleStartRaw = {
  status: number
  location: string | null
  code?: string
  type?: string
}

export type GoogleStartDecision
  = { kind: 'not_configured' }
    | { kind: 'redirect', url: string }
    | { kind: 'follow_start' }
    | { kind: 'unavailable' }

export function interpretGoogleStart(raw: GoogleStartRaw): GoogleStartDecision {
  if (raw.status === 503 || raw.code === 'not_configured') {
    return { kind: 'not_configured' }
  }
  if (raw.location) {
    return { kind: 'redirect', url: raw.location }
  }
  if (raw.status === 0 || raw.type === 'opaqueredirect') {
    return { kind: 'follow_start' }
  }
  if (raw.status >= 300 && raw.status < 400) {
    return { kind: 'follow_start' }
  }
  return { kind: 'unavailable' }
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
