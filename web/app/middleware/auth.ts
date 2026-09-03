import { guestRedirectTarget, sessionView } from '../composables/auth/gate'
import { useAuth } from '../composables/auth/useAuth'

export default defineNuxtRouteMiddleware(async (to) => {
  if (!to.path.startsWith('/account/listings')) {
    return
  }
  const { pending, isAuthenticated, hydrate } = useAuth()
  if (!isAuthenticated.value && !pending.value) {
    await hydrate()
  }
  const view = sessionView({
    authenticated: isAuthenticated.value,
    pending: pending.value
  })
  const target = guestRedirectTarget(view, import.meta.client)
  if (target) {
    return navigateTo(target)
  }
})
