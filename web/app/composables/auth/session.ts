export const ACCESS_TOKEN_KEY = 'canteiro.access_token'

export type TokenStorage = {
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => void
  removeItem: (key: string) => void
}

export type AuthCallback = {
  accessToken?: string
  error?: string
  sessionReady?: boolean
}

export function parseAuthCallback(search: string): AuthCallback {
  const trimmed = search.startsWith('?') ? search.slice(1) : search
  if (!trimmed) {
    return {}
  }

  const params = new URLSearchParams(trimmed)
  const error = params.get('error')
  if (error) {
    return { error }
  }

  const accessToken = params.get('access_token') || params.get('token')
  const authResult = params.get('auth')
  if (authResult === 'denied' || authResult === 'error') {
    return { error: authResult }
  }
  if (authResult === 'ok') {
    return { sessionReady: true }
  }
  if (!accessToken) {
    return {}
  }

  return { accessToken }
}

export function parseAuthQuery(query: Record<string, unknown>): AuthCallback {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (typeof value === 'string') {
      params.set(key, value)
    }
  }
  return parseAuthCallback(`?${params.toString()}`)
}

export function readAccessToken(storage: TokenStorage): string | null {
  return storage.getItem(ACCESS_TOKEN_KEY)
}

export function writeAccessToken(
  storage: TokenStorage,
  token: string | null
): void {
  if (!token) {
    storage.removeItem(ACCESS_TOKEN_KEY)
    return
  }
  storage.setItem(ACCESS_TOKEN_KEY, token)
}

export function browserStorage(): TokenStorage | null {
  if (typeof localStorage === 'undefined') {
    return null
  }
  return localStorage
}
