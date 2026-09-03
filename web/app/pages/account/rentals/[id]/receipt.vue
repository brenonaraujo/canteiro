<script setup lang="ts">
import { useRentalDetail } from '~/composables/rental/useRentalDetail'
import {
  formatCents,
  formatRange,
  formatUtcDateTime
} from '~/composables/rental/format'
import { mapDepositStatus } from '~/composables/rental/state'

defineOptions({ name: 'AccountRentalReceiptPage' })

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()

const rentalId = computed(() => String(route.params.id))

const {
  rental,
  receipt,
  pending,
  receiptPending,
  errorKey,
  receiptErrorKey,
  load,
  loadReceipt
} = useRentalDetail()

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.rentals'), to: '/account/rentals' },
  { label: t('breadcrumb.rental'), to: `/account/rentals/${rentalId.value}` },
  { label: t('breadcrumb.rental_receipt') }
])

useHead({
  title: () => t('rental.receipt.title')
})

onMounted(async () => {
  await load(rentalId.value)
  await loadReceipt(rentalId.value)
})

const receiptTotals = computed(() => {
  if (!receipt.value) return null
  return {
    rent_cents: receipt.value.rent_cents,
    operator_cents: receipt.value.operator_cents,
    deposit_cents: receipt.value.deposit_cents,
    total_cents: receipt.value.total_cents,
    commission_cents: receipt.value.commission_cents,
    owner_payout_cents: receipt.value.owner_payout_cents,
    operator_payout_cents: receipt.value.operator_payout_cents,
    deposit_status: mapDepositStatus(receipt.value.deposit_state)
  }
})

const cancelledByLabel = computed(() => {
  const actor = receipt.value?.actor_kind ?? 'system'
  return t(`rental.receipt.cancelled_by_${actor}`)
})

function back() {
  router.push(`/account/rentals/${rentalId.value}`)
}

const range = computed(() => {
  if (!receipt.value) return ''
  return formatRange(
    receipt.value.window_starts_at,
    receipt.value.window_ends_at,
    locale.value
  )
})

const issuedAtLabel = computed(() => {
  if (!receipt.value) return ''
  return formatUtcDateTime(receipt.value.issued_at, locale.value)
})

const cancelledAtLabel = computed(() => {
  if (!receipt.value?.cancellation_issued_at) return ''
  return formatUtcDateTime(receipt.value.cancellation_issued_at, locale.value)
})

const feeLabel = computed(() => {
  if (!receipt.value) return ''
  const fee = receipt.value.cancellation_fee_cents ?? 0
  if (fee <= 0) {
    return t('common.cancel')
  }
  return formatCents(fee, locale.value)
})
</script>

<template>
  <UContainer class="py-12">
    <div class="flex flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />

      <div
        v-if="pending || receiptPending"
        class="flex flex-col gap-4"
      >
        <USkeleton class="h-12 w-1/2" />
        <USkeleton class="h-48 w-full" />
      </div>

      <template v-else-if="receipt">
        <header class="flex flex-col gap-2">
          <h1 class="text-3xl font-bold tracking-tight text-highlighted">
            {{ t('rental.receipt.title') }}
          </h1>
          <p class="text-base text-muted">
            {{ t('rental.receipt.subtitle') }}
          </p>
        </header>

        <UCard>
          <template #header>
            <div class="flex flex-wrap items-center justify-between gap-4">
              <h2 class="text-base font-semibold text-highlighted">
                {{ rental?.listing_snapshot.title ?? receipt.rental_id }}
              </h2>
              <RentalStateBadge
                v-if="rental"
                :state="rental.state"
              />
            </div>
          </template>

          <dl class="flex flex-col gap-4">
            <div class="flex items-baseline justify-between gap-4">
              <dt class="text-sm text-muted">
                {{ cancelledByLabel }}
              </dt>
              <dd class="text-sm font-medium text-highlighted">
                {{ cancelledAtLabel }}
              </dd>
            </div>
            <div class="flex items-baseline justify-between gap-4">
              <dt class="text-sm text-muted">
                {{ t('rental.receipt.window_label') }}
              </dt>
              <dd class="text-sm font-medium text-highlighted">
                {{ receipt.window_applied }}
              </dd>
            </div>
            <div class="flex items-baseline justify-between gap-4">
              <dt class="text-sm text-muted">
                {{ t('rental.detail.summary.rental_window') }}
              </dt>
              <dd class="text-sm font-medium text-highlighted">
                {{ range }}
              </dd>
            </div>
            <div class="flex items-baseline justify-between gap-4">
              <dt class="text-sm text-muted">
                {{ t('rental.receipt.issued_at') }}
              </dt>
              <dd class="text-sm font-medium text-highlighted">
                {{ issuedAtLabel }}
              </dd>
            </div>
            <div class="flex items-baseline justify-between gap-4">
              <dt class="text-sm text-muted">
                {{ t('rental.receipt.processor_ref') }}
              </dt>
              <dd class="font-mono text-sm text-highlighted break-all">
                {{ receipt.processor_operation_id }}
              </dd>
            </div>
          </dl>
        </UCard>

        <RentalTotals
          v-if="receiptTotals"
          :totals="receiptTotals"
          show-deposit-status
        />

        <div class="flex justify-start">
          <UButton
            color="neutral"
            variant="ghost"
            class="min-h-11"
            @click="back"
          >
            {{ t('rental.receipt.back_to_rental') }}
          </UButton>
        </div>

        <div class="flex items-center gap-2 text-xs text-muted">
          <UIcon
            name="i-lucide-info"
            class="size-4"
          />
          <span>{{ feeLabel }}</span>
        </div>
      </template>

      <UAlert
        v-else-if="receiptErrorKey"
        color="neutral"
        variant="subtle"
        icon="i-lucide-info"
        :title="t(receiptErrorKey)"
      />

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
