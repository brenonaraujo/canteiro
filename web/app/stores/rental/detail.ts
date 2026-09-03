import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  RentalRequestError,
  type RentalClient
} from '../../composables/rental/client'
import type { Rental, RentalReceipt } from '../../composables/rental/types'

export const useRentalDetailStore = defineStore('rental-detail', () => {
  const rental = ref<Rental | null>(null)
  const receipt = ref<RentalReceipt | null>(null)
  const pending = ref(false)
  const receiptPending = ref(false)
  const cancelling = ref(false)
  const errorKey = ref<string | null>(null)
  const receiptErrorKey = ref<string | null>(null)
  const cancelErrorKey = ref<string | null>(null)

  async function load(id: string, client: RentalClient): Promise<void> {
    pending.value = true
    errorKey.value = null
    try {
      rental.value = await client.getMine(id)
    } catch (err) {
      errorKey.value = translateError(err)
    } finally {
      pending.value = false
    }
  }

  async function loadReceipt(id: string, client: RentalClient): Promise<void> {
    receiptPending.value = true
    receiptErrorKey.value = null
    try {
      receipt.value = await client.getReceipt(id)
    } catch (err) {
      receiptErrorKey.value = translateReceiptError(err)
    } finally {
      receiptPending.value = false
    }
  }

  async function cancel(id: string, client: RentalClient): Promise<boolean> {
    cancelling.value = true
    cancelErrorKey.value = null
    try {
      rental.value = await client.cancel(id)
      return true
    } catch (err) {
      cancelErrorKey.value = translateError(err)
      return false
    } finally {
      cancelling.value = false
    }
  }

  function clearCancelError(): void {
    cancelErrorKey.value = null
  }

  function reset(): void {
    rental.value = null
    receipt.value = null
    pending.value = false
    receiptPending.value = false
    cancelling.value = false
    errorKey.value = null
    receiptErrorKey.value = null
    cancelErrorKey.value = null
  }

  return {
    rental,
    receipt,
    pending,
    receiptPending,
    cancelling,
    errorKey,
    receiptErrorKey,
    cancelErrorKey,
    load,
    loadReceipt,
    cancel,
    clearCancelError,
    reset
  }
})

function translateError(err: unknown): string {
  if (!(err instanceof RentalRequestError)) {
    return 'rental.error.generic'
  }
  if (err.status === 401) {
    return 'rental.error.unauthorized'
  }
  if (err.status === 403) {
    return 'rental.error.forbidden'
  }
  if (err.status === 404) {
    return 'rental.error.not_found'
  }
  if (err.status === 409) {
    return 'rental.error.conflict'
  }
  return 'rental.error.generic'
}

function translateReceiptError(err: unknown): string {
  if (err instanceof RentalRequestError && err.status === 404) {
    return 'rental.error.receipt_not_ready'
  }
  return translateError(err)
}
