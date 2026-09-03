<script setup lang="ts">
import { useRentals } from '~/composables/rental/useRentals'
import { formatCents, formatRange } from '~/composables/rental/format'
import type { Rental } from '~/composables/rental/types'

defineOptions({ name: 'AccountRentalsIndexPage' })

const { t, locale } = useI18n()
const { items, pending, errorKey, load, clearError } = useRentals()

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.rentals') }
])

useHead({
  title: () => t('rental.list.title')
})

onMounted(async () => {
  await load()
})

function subtitle(rental: Rental): string {
  return formatRange(rental.starts_at, rental.ends_at, locale.value)
}

function total(rental: Rental): string {
  return formatCents(rental.rent_cents + rental.operator_cents + rental.deposit_cents, locale.value)
}
</script>

<template>
  <UContainer class="py-12">
    <div class="flex flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />

      <header class="flex flex-col gap-2">
        <h1 class="text-3xl font-bold tracking-tight text-highlighted">
          {{ t('rental.list.title') }}
        </h1>
        <p class="text-base text-muted">
          {{ t('rental.list.subtitle') }}
        </p>
      </header>

      <UAlert
        v-if="errorKey"
        color="error"
        variant="subtle"
        icon="i-lucide-alert-triangle"
        :title="t(errorKey)"
        :close-button="{
          icon: 'i-lucide-x',
          color: 'neutral',
          variant: 'ghost',
          onClick: () => clearError()
        }"
      />

      <div
        v-if="pending"
        class="flex flex-col gap-4"
      >
        <USkeleton
          v-for="i in 4"
          :key="i"
          class="h-24 w-full"
        />
      </div>

      <div
        v-else-if="items.length > 0"
        class="flex flex-col gap-3"
      >
        <UCard
          v-for="rental in items"
          :key="rental.id"
          class="transition hover:ring-1 hover:ring-primary"
        >
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div class="flex min-w-0 flex-col gap-1">
              <NuxtLink
                :to="`/account/rentals/${rental.id}`"
                class="text-base font-semibold text-highlighted hover:underline"
                :aria-label="t('rental.list.view_detail_aria', { title: rental.listing_snapshot.title })"
              >
                {{ rental.listing_snapshot.title }}
              </NuxtLink>
              <span class="text-sm text-muted">{{ subtitle(rental) }}</span>
            </div>
            <div class="flex flex-col items-end gap-2">
              <RentalStateBadge :state="rental.state" />
              <span class="text-sm font-semibold tabular-nums text-highlighted">
                {{ total(rental) }}
              </span>
            </div>
            <NuxtLink
              :to="`/account/rentals/${rental.id}`"
              class="text-sm font-medium text-primary hover:underline sm:ms-auto"
            >
              {{ t('rental.list.view_detail') }}
            </NuxtLink>
          </div>
        </UCard>
      </div>

      <div
        v-else
        class="flex flex-col items-center gap-4 rounded-lg border border-dashed border-default bg-elevated p-12 text-center"
      >
        <UIcon
          name="i-lucide-package-open"
          class="size-10 text-muted"
        />
        <h2 class="text-lg font-semibold text-highlighted">
          {{ t('rental.list.empty_title') }}
        </h2>
        <p class="text-sm text-muted">
          {{ t('rental.list.empty_body') }}
        </p>
        <UButton
          to="/listings"
          color="primary"
          class="min-h-11"
        >
          {{ t('rental.list.go_catalog') }}
        </UButton>
      </div>
    </div>
  </UContainer>
</template>
