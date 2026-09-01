export type SessionView = 'form' | 'loading' | 'guest'

export function sessionView(input: {
  authenticated: boolean
  pending: boolean
}): SessionView {
  if (input.authenticated) {
    return 'form'
  }
  if (input.pending) {
    return 'loading'
  }
  return 'guest'
}

export function guestRedirectTarget(
  view: SessionView,
  isClient: boolean
): '/auth/login' | null {
  if (isClient && view === 'guest') {
    return '/auth/login'
  }
  return null
}
