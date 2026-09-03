<script setup lang="ts">
import { formatCents } from '~/composables/rental/format'
import type { RentalTotals } from '~/composables/rental/types'

defineOptions({ name: 'RentalTotals' })

const props = defineProps<{
  totals: RentalTotals
  showDepositStatus?: boolean
}>()

const { t, locale } = useI18n()

const rows = computed(() => {
  const lines: Array<{ label: string, value: string, key: string }> = []
  lines.push({
    key: 'rent',
    label: t('rental.totals.rent'),
    value: formatCents(props.totals.rent_cents, locale.value)
  })
  if (props.totals.operator_cents > 0) {
    lines.push({
      key: 'operator',
      label: t('rental.totals.operator'),
      value: formatCents(props.totals.operator_cents, locale.value)
    })
  }
  lines.push({
    key: 'subtotal',
    label: t('rental.totals.subtotal'),
    value: formatCents(
      props.totals.rent_cents + props.totals.operator_cents,
      locale.value
    )
  })
  lines.push({
    key: 'deposit',
    label: t('rental.totals.deposit'),
    value: formatCents(props.totals.deposit_cents, locale.value)
  })
  lines.push({
    key: 'total',
    label: t('rental.totals.total'),
    value: formatCents(props.totals.total_cents, locale.value)
  })
  if (typeof props.totals.cancellation_fee_cents === 'number'
    && props.totals.cancellation_fee_cents > 0) {
    lines.push({
      key: 'cancellation_fee',
      label: t('rental.totals.cancellation_fee'),
      value: formatCents(props.totals.cancellation_fee_cents, locale.value)
    })
  }
  lines.push({
    key: 'commission',
    label: t('rental.totals.commission'),
    value: formatCents(props.totals.commission_cents, locale.value)
  })
  lines.push({
    key: 'owner_payout',
    label: t('rental.totals.owner_payout'),
    value: formatCents(props.totals.owner_payout_cents, locale.value)
  })
  if (props.totals.operator_payout_cents > 0) {
    lines.push({
      key: 'operator_payout',
      label: t('rental.totals.operator_payout'),
      value: formatCents(props.totals.operator_payout_cents, locale.value)
    })
  }
  return lines
})

const depositStatusLabel = computed(() => {
  if (!props.showDepositStatus || !props.totals.deposit_status) {
    return ''
  }
  return t(`rental.totals.deposit_status.${props.totals.deposit_status}`)
})
</script>

<template>
  <UCard :ui="{ body: 'flex flex-col gap-4' }">
    <template #header>
      <h3 class="text-base font-semibold text-highlighted">
        {{ t('rental.totals.title') }}
      </h3>
    </template>

    <dl class="flex flex-col gap-2">
      <div
        v-for="row in rows"
        :key="row.key"
        class="flex items-baseline justify-between gap-3"
      >
        <dt class="text-sm text-muted">
          {{ row.label }}
        </dt>
        <dd class="text-sm font-medium text-highlighted tabular-nums">
          {{ row.value }}
        </dd>
      </div>
    </dl>

    <p
      v-if="!showDepositStatus"
      class="rounded-md bg-elevated px-3 py-2 text-xs text-muted"
    >
      {{ t('rental.totals.deposit_note') }}
    </p>

    <div
      v-else-if="depositStatusLabel"
      class="flex items-center justify-between rounded-md bg-elevated px-3 py-2"
    >
      <span class="text-xs text-muted">{{ t('rental.totals.deposit') }}</span>
      <span class="text-xs font-semibold text-highlighted">
        {{ depositStatusLabel }}
      </span>
    </div>
  </UCard>
</template>
