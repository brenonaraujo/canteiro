import { storeToRefs } from 'pinia'
import {
  deriveStatus,
  validateProfile,
  type ProfileInput
} from './account'
import { googleStartUrl } from './api'
import { AuthRequestError, createAccountClient } from './client'
import {
  browserStorage,
  parseAuthQuery,
  writeAccessToken
} from './session'
import { useAccountStore } from '../../stores/auth/account'

export function useAuth() {
  const store = useAccountStore()
  const config = useRuntimeConfig()
  const refs = storeToRefs(store)

  function apiBase(): string {
    const configured = String(config.public.apiBase || '')
    if (configured) {
      return configured
    }
    if (import.meta.dev) {
      return 'http://localhost:8080'
    }
    return ''
  }

  function client() {
    const storage = browserStorage()
    if (!storage) {
      throw new Error('no_storage')
    }
    return createAccountClient({
      apiBase: apiBase(),
      storage,
      fetch: (input, init) => globalThis.fetch(input, init)
    })
  }

  async function hydrate() {
    const storage = browserStorage()
    if (!storage) {
      return
    }
    store.setPending(true)
    try {
      const me = await client().fetchMe()
      if (deriveStatus(me) === 'deactivated') {
        writeAccessToken(storage, null)
        store.clear()
        store.setErrorKey('auth.error.deactivated')
        return
      }
      store.setAccount(me)
      store.setErrorKey(null)
    } catch {
      store.clear()
    } finally {
      store.setPending(false)
    }
  }

  function startGoogle() {
    return navigateTo(googleStartUrl(apiBase()), { external: true })
  }

  async function completeCallback(query: Record<string, unknown>) {
    const parsed = parseAuthQuery(query)
    const storage = browserStorage()
    if (parsed.error) {
      store.setErrorKey('auth.callback.error')
      return parsed
    }
    if (parsed.accessToken && storage) {
      writeAccessToken(storage, parsed.accessToken)
    }
    await hydrate()
    return parsed
  }

  async function saveProfile(input: ProfileInput) {
    const checked = validateProfile(input)
    if (!checked.ok) {
      store.setErrorKey('auth.profile.required')
      return checked
    }
    store.setPending(true)
    try {
      store.setAccount(await client().saveProfile(checked.value))
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

  async function logout() {
    try {
      await client().logout()
    } catch {
      const storage = browserStorage()
      if (storage) {
        writeAccessToken(storage, null)
      }
    }
    store.clear()
    await navigateTo('/')
  }

  async function deactivate() {
    await client().deactivate()
    store.clear()
    await navigateTo('/')
  }

  return {
    ...refs,
    hydrate,
    startGoogle,
    completeCallback,
    saveProfile,
    logout,
    deactivate
  }
}
