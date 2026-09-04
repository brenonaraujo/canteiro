import { storeToRefs } from 'pinia'
import type { ProfileInput } from './account'
import {
  completeAuthCallback,
  deactivateAccount,
  hydrateAccount,
  logoutAccount,
  resolveApiBase,
  saveAccountProfile
} from './actions'
import { googleStartUrl } from './api'
import { createAccountClient } from './client'
import { probeGoogleStart, startGoogleLogin } from './google'
import {
  browserStorage,
  parseAuthQuery,
  type TokenStorage
} from './session'
import { useAccountStore } from '../../stores/auth/account'

const emptyStorage: TokenStorage = {
  getItem: () => null,
  setItem: () => {},
  removeItem: () => {}
}

export function useAuth() {
  const store = useAccountStore()
  const refs = storeToRefs(store)
  const run = () => bindAccount(store, useRuntimeConfig())
  return {
    ...refs,
    hydrate: () => run().hydrate(),
    startGoogle: () => run().startGoogle(),
    completeCallback: (query: Record<string, unknown>) => run().completeCallback(query),
    saveProfile: (input: ProfileInput) => run().saveProfile(input),
    logout: () => run().logout(),
    deactivate: () => run().deactivate()
  }
}

function bindAccount(
  store: ReturnType<typeof useAccountStore>,
  config: { public: { apiBase: string } }
) {
  const apiBase = resolveApiBase(String(config.public.apiBase || ''), import.meta.dev)
  const storage = browserStorage() ?? emptyStorage
  const client = createAccountClient({
    apiBase,
    storage,
    fetch: (input, init) => globalThis.fetch(input, init)
  })
  return {
    hydrate: () => hydrateAccount(store, client, storage),
    startGoogle: () => startGoogleBound(store, apiBase),
    completeCallback: (query: Record<string, unknown>) => {
      return completeAuthCallback(store, client, storage, parseAuthQuery(query))
    },
    saveProfile: (input: ProfileInput) => saveAccountProfile(store, client, input),
    logout: async () => {
      await logoutAccount(store, client, storage)
      await navigateTo('/')
    },
    deactivate: async () => {
      await deactivateAccount(store, client)
      await navigateTo('/')
    }
  }
}

async function startGoogleBound(
  store: ReturnType<typeof useAccountStore>,
  apiBase: string
) {
  const result = await startGoogleLogin(
    store,
    () => probeGoogleStart((input, init) => globalThis.fetch(input, init), apiBase),
    googleStartUrl(apiBase)
  )
  if (result.redirectTo) {
    return navigateTo(result.redirectTo, { external: true })
  }
}
