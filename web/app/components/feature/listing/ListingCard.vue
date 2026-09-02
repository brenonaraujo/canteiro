<script setup lang="ts">
import type { PublicListing } from '~/composables/listing/types'

defineOptions({ name: 'ListingCard' })

const props = defineProps<{
  listing: PublicListing
}>()

const { t, locale } = useI18n()

function toLocaleTag(loc: string): string {
  if (loc === 'pt-BR') return 'pt-BR'
  if (loc === 'es') return 'es-ES'
  return 'en-US'
}

const depositFormatted = computed(() => {
  return new Intl.NumberFormat(toLocaleTag(locale.value), {
    style: 'currency',
    currency: 'BRL'
  }).format(props.listing.deposit_cents / 100)
})
</script>

<template>
  <UCard
    class="flex h-full flex-col"
    :ui="{ body: 'flex flex-1 flex-col gap-4' }"
  >
    <template #header>
      <div class="flex items-start justify-between gap-4">
        <h3 class="text-base font-semibold text-highlighted">
          {{ listing.title }}
        </h3>
        <ListingBadges
          :category="listing.category"
          :size="listing.size"
          :operator-mode="listing.operator_mode"
        />
      </div>
    </template>

    <ListingPriceLabel
      :price-amount-cents="listing.price_amount_cents"
      :price-unit="listing.price_unit"
    />

    <div class="flex flex-col gap-2 text-sm text-muted">
      <div class="inline-flex items-center gap-2">
        <UIcon
          name="i-lucide-map-pin"
          class="size-4 shrink-0"
        />
        <span>
          {{ listing.pickup_city }} · {{ listing.pickup_neighborhood }}
        </span>
      </div>
      <div class="inline-flex items-center gap-2">
        <UIcon
          name="i-lucide-shield"
          class="size-4 shrink-0"
        />
        <span>{{ t('listing.card.deposit_label') }}</span>
        <span class="text-fg">{{ depositFormatted }}</span>
      </div>
    </div>

    <template #footer>
      <UButton
        :to="`/listings/${listing.id}`"
        color="primary"
        variant="soft"
        size="sm"
        class="min-h-11"
        :aria-label="t('listing.card.view_listing', { title: listing.title })"
      >
        {{ t('listing.card.view') }}
      </UButton>
    </template>
  </UCard>
</template>
