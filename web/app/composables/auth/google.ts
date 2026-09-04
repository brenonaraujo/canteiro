import type { AccountMutator } from './actions'
import { googleStartUrl, interpretGoogleStart, type GoogleStartRaw } from './api'
import type { FetchLike } from './client'

export type GoogleStartResult = {
  redirectTo: string | null
}

export async function startGoogleLogin(
  store: AccountMutator,
  probe: () => Promise<GoogleStartRaw>,
  startUrl: string
): Promise<GoogleStartResult> {
  store.setPending(true)
  store.setErrorKey(null)
  try {
    return decideGoogleStart(store, await probe(), startUrl)
  } catch {
    store.setErrorKey('auth.error.generic')
    return { redirectTo: null }
  } finally {
    store.setPending(false)
  }
}

function decideGoogleStart(
  store: AccountMutator,
  raw: GoogleStartRaw,
  startUrl: string
): GoogleStartResult {
  const decision = interpretGoogleStart(raw)
  if (decision.kind === 'redirect') {
    return { redirectTo: decision.url }
  }
  if (decision.kind === 'follow_start') {
    return { redirectTo: startUrl }
  }
  store.setErrorKey(googleStartErrorKey(decision.kind))
  return { redirectTo: null }
}

function googleStartErrorKey(kind: 'not_configured' | 'unavailable'): string {
  if (kind === 'not_configured') {
    return 'auth.not_configured'
  }
  return 'auth.error.generic'
}

export async function probeGoogleStart(
  fetch: FetchLike,
  apiBase: string
): Promise<GoogleStartRaw> {
  const response = await fetch(googleStartUrl(apiBase), {
    method: 'GET',
    redirect: 'manual',
    credentials: 'include'
  })
  return {
    status: response.status,
    location: response.headers.get('location'),
    code: await readErrorCode(response),
    type: response.type
  }
}

async function readErrorCode(response: Response): Promise<string | undefined> {
  const type = response.headers.get('content-type') ?? ''
  if (!type.includes('application/json')) {
    return undefined
  }
  try {
    const body = await response.json() as { code?: unknown }
    return typeof body.code === 'string' ? body.code : undefined
  } catch {
    return undefined
  }
}
