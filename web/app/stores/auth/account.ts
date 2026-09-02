import { defineStore } from 'pinia'
import type { Account } from '../../composables/auth/account'
import { canStartRental, deriveStatus } from '../../composables/auth/account'

export const useAccountStore = defineStore('account', () => {
  const account = ref<Account | null>(null)
  const pending = ref(false)
  const errorKey = ref<string | null>(null)

  const status = computed(() => {
    return account.value ? deriveStatus(account.value) : null
  })

  const isAuthenticated = computed(() => {
    return status.value !== null && status.value !== 'deactivated'
  })

  const isProfileComplete = computed(() => status.value === 'active')
  const canRent = computed(() => {
    return status.value !== null && canStartRental(status.value)
  })
  const visibleName = computed(() => account.value?.displayName.trim() ?? '')

  function setAccount(next: Account | null) {
    account.value = next
  }

  function setPending(next: boolean) {
    pending.value = next
  }

  function setErrorKey(next: string | null) {
    errorKey.value = next
  }

  function clear() {
    account.value = null
    errorKey.value = null
  }

  return {
    account,
    pending,
    errorKey,
    status,
    isAuthenticated,
    isProfileComplete,
    canRent,
    visibleName,
    setAccount,
    setPending,
    setErrorKey,
    clear
  }
})
