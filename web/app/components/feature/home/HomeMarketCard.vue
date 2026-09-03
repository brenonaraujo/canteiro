<script setup lang="ts">
import type { PublicListing } from '~/composables/listing/types'

defineOptions({ name: 'HomeMarketCard' })

const props = defineProps<{
  listing: PublicListing
}>()

const photo = computed(() => props.listing.photos?.[0] ?? '')
</script>

<template>
  <UCard
    class="flex h-full flex-col"
    :ui="{ body: 'flex flex-1 flex-col gap-4' }"
  >
    <div class="overflow-hidden rounded-md bg-muted">
      <img
        v-if="photo"
        :src="photo"
        :alt="listing.title"
        class="h-40 w-full object-cover"
      >
      <div
        v-else
        class="flex h-40 items-center justify-center text-muted"
      >
        <UIcon
          name="i-lucide-wrench"
          class="size-8"
          aria-hidden="true"
        />
      </div>
    </div>
    <h3 class="text-base font-semibold text-highlighted">
      {{ listing.title }}
    </h3>
    <p class="text-sm text-muted">
      {{ listing.pickup_city }}
    </p>
    <ListingPriceLabel
      :price-amount-cents="listing.price_amount_cents"
      :price-unit="listing.price_unit"
    />
    <UButton
      :to="`/listings/${listing.id}`"
      color="primary"
      variant="soft"
      class="min-h-11"
    >
      {{ $t('listing.card.view') }}
    </UButton>
  </UCard>
</template>
