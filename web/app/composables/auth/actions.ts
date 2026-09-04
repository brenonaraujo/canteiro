import type { Account, ProfileInput } from './account'
import { deriveStatus, validateProfile } from './account'
import { AuthRequestError } from './client'
import {
  writeAccessToken,
  type AuthCallback,
  type TokenStorage
} from './session'

export type AccountMutator = {
  setAccount: (account: Account | null) => void
  setPending: (pending: boolean) => void
  setErrorKey: (key: string | null) => void
  clear: () => void
}

export type AccountClient = {
  fetchMe: () => Promise<Account>
  saveProfile: (input: ProfileInput) => Promise<Account>
  logout: () => Promise<void>
  deactivate: () => Promise<void>
}

export function resolveApiBase(configured: string, isDev: boolean): string {
  if (configured) {
    return configured
  }
  if (isDev) {
    return 'http://localhost:8080'
  }
  return ''
}

export async function hydrateAccount(
  store: AccountMutator,
  client: AccountClient,
  storage: TokenStorage
): Promise<'ready' | 'missing' | 'deactivated'> {
  store.setPending(true)
  try {
    const me = await client.fetchMe()
    if (deriveStatus(me) === 'deactivated') {
      writeAccessToken(storage, null)
      store.clear()
      store.setErrorKey('auth.error.deactivated')
      return 'deactivated'
    }
    store.setAccount(me)
    store.setErrorKey(null)
    return 'ready'
  } catch {
    store.clear()
    return 'missing'
  } finally {
    store.setPending(false)
  }
}

export async function completeAuthCallback(
  store: AccountMutator,
  client: AccountClient,
  storage: TokenStorage,
  parsed: AuthCallback
): Promise<AuthCallback> {
  if (parsed.error) {
    store.setErrorKey('auth.callback.error')
    return parsed
  }
  if (parsed.accessToken) {
    writeAccessToken(storage, parsed.accessToken)
  }
  const status = await hydrateAccount(store, client, storage)
  if (status === 'missing' && (parsed.sessionReady || parsed.accessToken)) {
    store.setErrorKey('auth.callback.error')
  }
  return parsed
}

export async function saveAccountProfile(
  store: AccountMutator,
  client: AccountClient,
  input: ProfileInput
) {
  const checked = validateProfile(input)
  if (!checked.ok) {
    store.setErrorKey('auth.profile.required')
    return checked
  }
  store.setPending(true)
  try {
    store.setAccount(await client.saveProfile(checked.value))
    store.setErrorKey(null)
    return checked
  } catch (error) {
    const deactivated = error instanceof AuthRequestError && error.status === 403
    store.setErrorKey(deactivated ? 'auth.error.deactivated' : 'auth.error.generic')
    return { ok: false as const, errors: ['displayName' as const] }
  } finally {
    store.setPending(false)
  }
}

export async function logoutAccount(
  store: AccountMutator,
  client: AccountClient,
  storage: TokenStorage
): Promise<void> {
  try {
    await client.logout()
  } catch {
    writeAccessToken(storage, null)
  }
  store.clear()
}

export async function deactivateAccount(
  store: AccountMutator,
  client: AccountClient
): Promise<void> {
  await client.deactivate()
  store.clear()
}
