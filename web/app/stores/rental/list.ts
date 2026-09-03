import { defineStore } from 'pinia'
import { ref } from 'vue'
import { RentalRequestError, type RentalClient } from '../../composables/rental/client'
import type { Rental } from '../../composables/rental/types'

export const useRentalsListStore = defineStore('rental-list', () => {
  const items = ref<Rental[]>([])
  const pending = ref(false)
  const errorKey = ref<string | null>(null)

  async function load(client: RentalClient): Promise<void> {
    pending.value = true
    errorKey.value = null
    try {
      items.value = await client.listMine()
    } catch (err) {
      errorKey.value = translateError(err)
    } finally {
      pending.value = false
    }
  }

  function upsert(rental: Rental): void {
    const index = items.value.findIndex(r => r.id === rental.id)
    if (index >= 0) {
      items.value.splice(index, 1, rental)
    } else {
      items.value.unshift(rental)
    }
  }

  function clearError(): void {
    errorKey.value = null
  }

  function reset(): void {
    items.value = []
    errorKey.value = null
    pending.value = false
  }

  return {
    items,
    pending,
    errorKey,
    load,
    upsert,
    clearError,
    reset
  }
})

function translateError(err: unknown): string {
  if (err instanceof RentalRequestError) {
    if (err.status === 401) {
      return 'rental.error.unauthorized'
    }
    if (err.status === 404) {
      return 'rental.error.not_found'
    }
  }
  return 'rental.error.generic'
}
