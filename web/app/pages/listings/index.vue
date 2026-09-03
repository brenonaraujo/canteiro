<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ListingSearchFilters } from '~/composables/listing/types'
import { useListingList } from '~/composables/listing/useListingList'

defineOptions({ name: 'ListingsIndexPage' })

const { t } = useI18n()
const { page, pending, errorKey, search, loadCategories } = useListingList()

const filters = reactive<ListingSearchFilters>({ page: 1 })

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.listings') }
])

useHead({
  title: () => t('listing.search.title')
})

onMounted(async () => {
  await loadCategories()
  await search(filters)
})

async function onApply(next: ListingSearchFilters) {
  filters.page = 1
  await search({ ...next, page: 1 })
}

async function onPageChange(next: number) {
  filters.page = next
  await search({ ...filters, page: next })
}

const items = computed(() => page.value?.items ?? [])
const totalPages = computed(() => {
  if (!page.value) return 1
  return Math.max(1, Math.ceil(page.value.total / page.value.page_size))
})
</script>

<template>
  <UContainer class="py-12">
    <div class="flex flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />

      <header class="flex flex-col gap-2">
        <h1 class="text-3xl font-bold tracking-tight text-highlighted">
          {{ t('listing.search.title') }}
        </h1>
        <p class="text-base text-muted">
          {{ t('listing.search.subtitle') }}
        </p>
      </header>

      <ListingFilters
        v-model="filters"
        @apply="onApply"
      />

      <UAlert
        v-if="errorKey"
        color="error"
        variant="subtle"
        icon="i-lucide-alert-triangle"
        :title="t(errorKey)"
      />

      <div
        v-if="pending"
        class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
      >
        <USkeleton
          v-for="i in 6"
          :key="i"
          class="h-48 w-full"
        />
      </div>

      <div
        v-else-if="items.length > 0"
        class="flex flex-col gap-6"
      >
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          <ListingCard
            v-for="item in items"
            :key="item.id"
            :listing="item"
          />
        </div>

        <nav
          v-if="totalPages > 1"
          aria-label="pagination"
          class="flex items-center justify-center gap-2"
        >
          <UButton
            color="neutral"
            variant="outline"
            :disabled="(filters.page ?? 1) <= 1"
            icon="i-lucide-chevron-left"
            class="min-h-11"
            @click="onPageChange((filters.page ?? 1) - 1)"
          >
            {{ t('listing.search.prev') }}
          </UButton>
          <span class="text-sm text-muted">
            {{
              t('listing.search.page_x_of_y', {
                x: filters.page ?? 1,
                y: totalPages
              })
            }}
          </span>
          <UButton
            color="neutral"
            variant="outline"
            :disabled="(filters.page ?? 1) >= totalPages"
            icon="i-lucide-chevron-right"
            trailing
            class="min-h-11"
            @click="onPageChange((filters.page ?? 1) + 1)"
          >
            {{ t('listing.search.next') }}
          </UButton>
        </nav>
      </div>

      <div
        v-else
        class="flex flex-col items-center gap-4 rounded-lg border border-dashed border-default bg-elevated p-12 text-center"
      >
        <UIcon
          name="i-lucide-package-search"
          class="size-10 text-muted"
        />
        <h2 class="text-lg font-semibold text-highlighted">
          {{ t('listing.search.empty_title') }}
        </h2>
        <p class="text-sm text-muted">
          {{ t('listing.search.empty_body') }}
        </p>
      </div>
    </div>
  </UContainer>
</template>
