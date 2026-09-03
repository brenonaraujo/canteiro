<script setup lang="ts">
import type { Listing } from '~/composables/listing/types'

defineOptions({ name: 'OwnerListingCard' })

const props = defineProps<{
  listing: Listing
}>()

const { t } = useI18n()

const isPublished = computed(() => props.listing.state === 'published')
const isPaused = computed(() => props.listing.state === 'paused')
const isDraft = computed(() => props.listing.state === 'draft')

const editPath = computed(() => `/account/listings/${props.listing.id}/edit`)
const publicPath = computed(() => `/listings/${props.listing.id}`)
</script>

<template>
  <UCard class="flex h-full flex-col">
    <template #header>
      <div class="flex items-start justify-between gap-4">
        <h3 class="text-base font-semibold text-highlighted">
          {{ listing.title }}
        </h3>
        <ListingStateBadge :state="listing.state" />
      </div>
    </template>

    <div class="flex flex-col gap-4">
      <ListingBadges
        :category="listing.category"
        :size="listing.size"
        :operator-mode="listing.operator.mode"
      />

      <ListingPriceLabel
        :price-amount-cents="listing.price_amount_cents"
        :price-unit="listing.price_unit"
      />

      <div class="flex items-center gap-2 text-sm text-muted">
        <UIcon
          name="i-lucide-map-pin"
          class="size-4 shrink-0"
        />
        <span>
          {{ listing.pickup_city }} · {{ listing.pickup_neighborhood }}
        </span>
      </div>
    </div>

    <template #footer>
      <div class="flex flex-wrap items-center gap-2">
        <UButton
          v-if="isDraft || isPaused"
          :to="editPath"
          color="neutral"
          variant="outline"
          size="sm"
          class="min-h-11"
        >
          {{ t('listing.owner.edit') }}
        </UButton>

        <UButton
          v-if="isPublished"
          :to="publicPath"
          color="primary"
          variant="soft"
          size="sm"
          class="min-h-11"
          target="_blank"
        >
          {{ t('listing.owner.view_public') }}
        </UButton>

        <slot
          name="actions"
          :listing="listing"
        />
      </div>
    </template>
  </UCard>
</template>
