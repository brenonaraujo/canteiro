<script setup lang="ts">
import { useRentalDetail } from '~/composables/rental/useRentalDetail'
import { formatCents, formatRange } from '~/composables/rental/format'
import { isCancellableByTenant } from '~/composables/rental/state'
import type { Rental } from '~/composables/rental/types'

defineOptions({ name: 'AccountRentalDetailPage' })

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const toast = useToast()

const rentalId = computed(() => String(route.params.id))

const {
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
  clearCancelError
} = useRentalDetail()

const slideoverOpen = ref(false)

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.rentals'), to: '/account/rentals' },
  { label: t('breadcrumb.rental') }
])

useHead({
  title: () => t('rental.detail.title')
})

onMounted(async () => {
  const id = rentalId.value
  await load(id)
  if (rental.value) {
    await loadReceipt(id)
  }
})

watch(rentalId, async (id) => {
  if (id) {
    await load(id)
    if (rental.value) {
      await loadReceipt(id)
    }
  }
})

const canCancelAsTenant = computed(() => {
  if (!rental.value) return false
  return isCancellableByTenant(rental.value.state)
})

const canCancelAsOwner = computed(() => {
  if (!rental.value) return false
  return ['pending', 'authorized', 'confirmed'].includes(rental.value.state)
})

const showCancelButton = computed(() => {
  return canCancelAsTenant.value || canCancelAsOwner.value
})

const totalsForDisplay = computed(() => {
  if (!rental.value) return null
  const r: Rental = rental.value
  return {
    rent_cents: r.rent_cents,
    operator_cents: r.operator_cents,
    deposit_cents: r.deposit_cents,
    total_cents: r.rent_cents + r.operator_cents + r.deposit_cents,
    commission_cents: r.commission_cents,
    owner_payout_cents: r.owner_payout_cents,
    operator_payout_cents: r.operator_payout_cents
  }
})

const windowLabel = computed(() => {
  if (!rental.value?.confirmed_at) {
    return t('rental.window.pre_acceptance')
  }
  return t('rental.window.unknown')
})

const rentalWindow = computed(() => {
  if (!rental.value) return ''
  return formatRange(rental.value.starts_at, rental.value.ends_at, locale.value)
})

async function onCancelConfirmed() {
  const id = rentalId.value
  const ok = await cancel(id)
  if (ok) {
    toast.add({
      title: t('rental.confirm_cancel.submit_tenant'),
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    slideoverOpen.value = false
    await loadReceipt(id)
    return
  }
}

function openReceipt() {
  router.push(`/account/rentals/${rentalId.value}/receipt`)
}
</script>

<template>
  <UContainer class="py-12">
    <div class="flex flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />

      <div
        v-if="pending"
        class="flex flex-col gap-4"
      >
        <USkeleton class="h-12 w-1/2" />
        <USkeleton class="h-32 w-full" />
      </div>

      <template v-else-if="rental">
        <header class="flex flex-wrap items-end justify-between gap-4">
          <div class="flex flex-col gap-2">
            <h1 class="text-3xl font-bold tracking-tight text-highlighted">
              {{ rental.listing_snapshot.title }}
            </h1>
            <p class="text-base text-muted">
              {{ t('rental.detail.subtitle') }}
            </p>
            <p class="text-sm text-muted">
              {{ rentalWindow }}
            </p>
          </div>
          <RentalStateBadge :state="rental.state" />
        </header>

        <RentalBanner
          :state="rental.state"
          :has-receipt="!!receipt"
        />

        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="flex flex-col gap-4 lg:col-span-2">
            <UCard>
              <template #header>
                <h2 class="text-base font-semibold text-highlighted">
                  {{ t('rental.detail.summary.window') }}
                </h2>
              </template>
              <p class="text-sm text-muted">
                {{ windowLabel }}
              </p>
              <p class="mt-2 text-sm text-highlighted">
                {{ rental.with_operator
                  ? t('rental.detail.summary.with_operator')
                  : t('rental.detail.summary.without_operator') }}
              </p>
            </UCard>

            <RentalTotals
              v-if="totalsForDisplay"
              :totals="totalsForDisplay"
            />

            <UAlert
              v-if="cancelErrorKey"
              color="error"
              variant="subtle"
              icon="i-lucide-alert-triangle"
              :title="t(cancelErrorKey)"
              :close-button="{
                icon: 'i-lucide-x',
                color: 'neutral',
                variant: 'ghost',
                onClick: () => clearCancelError()
              }"
            />

            <div class="flex flex-wrap items-center gap-4">
              <UButton
                v-if="showCancelButton"
                color="error"
                variant="soft"
                class="min-h-11"
                @click="slideoverOpen = true"
              >
                {{ t('rental.actions.cancel_tenant') }}
              </UButton>
              <UButton
                v-if="receipt"
                color="primary"
                variant="soft"
                class="min-h-11"
                @click="openReceipt"
              >
                {{ t('rental.actions.view_receipt') }}
              </UButton>
              <UButton
                v-if="receiptPending"
                color="neutral"
                variant="ghost"
                class="min-h-11"
                :loading="receiptPending"
              >
                {{ t('rental.actions.opening_receipt') }}
              </UButton>
            </div>
          </div>

          <div class="flex flex-col gap-4">
            <UCard>
              <template #header>
                <h2 class="text-base font-semibold text-highlighted">
                  {{ t('rental.detail.summary.rental_window') }}
                </h2>
              </template>
              <p class="text-sm text-muted">
                {{ rentalWindow }}
              </p>
            </UCard>

            <UCard v-if="receipt">
              <template #header>
                <h2 class="text-base font-semibold text-highlighted">
                  {{ t('rental.receipt.title') }}
                </h2>
              </template>
              <p class="text-sm text-muted">
                {{ t('rental.receipt.subtitle') }}
              </p>
              <p class="mt-2 text-sm font-medium text-highlighted tabular-nums">
                {{ formatCents(receipt.total_cents, locale) }}
              </p>
              <UButton
                variant="ghost"
                color="primary"
                class="mt-4 min-h-11"
                @click="openReceipt"
              >
                {{ t('rental.actions.view_receipt') }}
              </UButton>
            </UCard>

            <UAlert
              v-else-if="receiptErrorKey"
              color="neutral"
              variant="subtle"
              icon="i-lucide-info"
              :title="t(receiptErrorKey)"
            />
          </div>
        </div>

        <UAlert
          v-if="errorKey"
          color="error"
          variant="subtle"
          icon="i-lucide-alert-triangle"
          :title="t(errorKey)"
        />

        <CancelRentalSlideover
          v-if="canCancelAsTenant || canCancelAsOwner"
          v-model="slideoverOpen"
          actor="tenant"
          :busy="cancelling"
          :error-key="cancelErrorKey"
          @submit="onCancelConfirmed"
        />
      </template>

      <UAlert
        v-else-if="errorKey"
        color="error"
        variant="subtle"
        icon="i-lucide-alert-triangle"
        :title="t(errorKey)"
      />
    </div>
  </UContainer>
</template>
