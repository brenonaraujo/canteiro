<script setup lang="ts">
import type { PriceUnit } from '~/composables/listing/types'

defineOptions({ name: 'ListingPriceLabel' })

const props = defineProps<{
  priceAmountCents: number
  priceUnit: PriceUnit
}>()

const { t, locale } = useI18n()

const formatted = computed(() => {
  const value = props.priceAmountCents / 100
  return new Intl.NumberFormat(toLocaleTag(locale.value), {
    style: 'currency',
    currency: 'BRL'
  }).format(value)
})

const unitLabel = computed(() => {
  return props.priceUnit === 'hour'
    ? t('listing.price.per_hour')
    : t('listing.price.per_day')
})

function toLocaleTag(loc: string): string {
  if (loc === 'pt-BR') return 'pt-BR'
  if (loc === 'es') return 'es-ES'
  return 'en-US'
}

defineExpose({ formatted, unitLabel })
</script>

<template>
  <span class="inline-flex items-baseline gap-1">
    <span class="text-xl font-semibold text-highlighted">{{ formatted }}</span>
    <span class="text-sm text-muted">/ {{ unitLabel }}</span>
  </span>
</template>
