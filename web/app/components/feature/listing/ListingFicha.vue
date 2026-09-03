<script setup lang="ts">
import type { PublicCalendar, PublicListing } from '~/composables/listing/types'

defineOptions({ name: 'ListingFicha' })

const props = defineProps<{
  listing: PublicListing
  calendar: PublicCalendar | null
}>()

const { t, locale } = useI18n()

function toLocaleTag(loc: string): string {
  if (loc === 'pt-BR') return 'pt-BR'
  if (loc === 'es') return 'es-ES'
  return 'en-US'
}

const depositFormatted = computed(() => {
  return new Intl.NumberFormat(toLocaleTag(locale.value), {
    style: 'currency', currency: 'BRL'
  }).format(props.listing.deposit_cents / 100)
})

const blocks = computed(() => props.calendar?.blocks ?? [])

const sortedBlocks = computed(() => {
  return [...blocks.value].sort((a, b) => a.starts_at.localeCompare(b.starts_at))
})

function formatRange(start: string, end: string) {
  const fmt = new Intl.DateTimeFormat(toLocaleTag(locale.value), {
    dateStyle: 'medium'
  })
  return `${fmt.format(new Date(start))} – ${fmt.format(new Date(end))}`
}
</script>

<template>
  <article class="flex flex-col gap-8">
    <header class="flex flex-col gap-4">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <h1 class="text-3xl font-bold tracking-tight text-highlighted">
          {{ listing.title }}
        </h1>
        <ListingBadges
          :category="listing.category"
          :size="listing.size"
          :operator-mode="listing.operator_mode"
        />
      </div>
      <ListingPriceLabel
        :price-amount-cents="listing.price_amount_cents"
        :price-unit="listing.price_unit"
      />
    </header>

    <section class="grid grid-cols-1 gap-6 md:grid-cols-3">
      <div class="flex flex-col gap-2 rounded-lg border border-default bg-elevated p-4">
        <span class="text-sm text-muted">{{ t('listing.ficha.city') }}</span>
        <span class="text-base text-fg">{{ listing.pickup_city }}</span>
        <span class="text-sm text-muted">{{ listing.pickup_neighborhood }}</span>
      </div>

      <div class="flex flex-col gap-2 rounded-lg border border-default bg-elevated p-4">
        <span class="text-sm text-muted">{{ t('listing.ficha.deposit') }}</span>
        <span class="text-base text-fg">{{ depositFormatted }}</span>
      </div>

      <div class="flex flex-col gap-2 rounded-lg border border-default bg-elevated p-4">
        <span class="text-sm text-muted">{{ t('listing.ficha.lead_time') }}</span>
        <span class="text-base text-fg">
          {{ t('listing.ficha.lead_time_value', { hours: listing.min_lead_time_hours }) }}
        </span>
      </div>
    </section>

    <section class="flex flex-col gap-2">
      <h2 class="text-lg font-semibold text-highlighted">
        {{ t('listing.ficha.description') }}
      </h2>
      <p class="whitespace-pre-line text-base text-fg">
        {{ listing.description }}
      </p>
    </section>

    <section
      v-if="sortedBlocks.length > 0"
      class="flex flex-col gap-2"
    >
      <h2 class="text-lg font-semibold text-highlighted">
        {{ t('listing.ficha.unavailable') }}
      </h2>
      <ul class="flex flex-col gap-1 text-sm text-fg">
        <li
          v-for="block in sortedBlocks"
          :key="block.starts_at"
          class="inline-flex items-center gap-2"
        >
          <UIcon
            name="i-lucide-calendar-x"
            class="size-4 text-muted"
          />
          <span>{{ formatRange(block.starts_at, block.ends_at) }}</span>
        </li>
      </ul>
    </section>

    <section
      v-else
      class="rounded-lg border border-dashed border-default bg-elevated p-4 text-sm text-muted"
    >
      {{ t('listing.ficha.unavailable_empty') }}
    </section>

    <section class="flex flex-col gap-2 rounded-lg border border-default bg-elevated p-4">
      <div class="flex items-center gap-2 text-sm text-muted">
        <UIcon
          name="i-lucide-info"
          class="size-4"
        />
        <span>{{ t('listing.ficha.identity_note') }}</span>
      </div>
    </section>
  </article>
</template>
