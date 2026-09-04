<script setup lang="ts">
import { useListingList } from '~/composables/listing/useListingList'

defineOptions({ name: 'HomeMarket' })

const { t } = useI18n()
const { page, pending, errorKey, search } = useListingList()

onMounted(async () => {
  await search({ page: 1 })
})

const items = computed(() => (page.value?.items ?? []).slice(0, 3))
const hasItems = computed(() => items.value.length > 0)

async function retry() {
  await search({ page: 1 })
}
</script>

<template>
  <UPageSection
    id="market"
    :title="t('landing.market.title')"
  >
    <div
      v-if="pending"
      class="grid grid-cols-1 gap-4 md:grid-cols-3"
    >
      <USkeleton
        v-for="i in 3"
        :key="i"
        class="h-64 w-full"
      />
    </div>

    <div
      v-else-if="errorKey"
      class="flex flex-col items-center gap-6 rounded-lg border border-default bg-elevated p-8 text-center"
    >
      <UAlert
        color="error"
        variant="subtle"
        icon="i-lucide-alert-triangle"
        :title="t('landing.market.error_title')"
        :description="t(errorKey)"
        class="w-full"
      />
      <p class="max-w-lg text-base text-muted">
        {{ t('landing.market.error_body') }}
      </p>
      <div class="flex flex-col gap-4 sm:flex-row">
        <UButton
          color="primary"
          class="min-h-11"
          @click="retry"
        >
          {{ t('landing.market.retry') }}
        </UButton>
        <UButton
          to="/listings"
          color="neutral"
          variant="outline"
          class="min-h-11"
        >
          {{ t('landing.market.view_all') }}
        </UButton>
      </div>
    </div>

    <div
      v-else-if="hasItems"
      class="flex flex-col gap-8"
    >
      <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
        <HomeMarketCard
          v-for="item in items"
          :key="item.id"
          :listing="item"
        />
      </div>
      <div class="flex justify-center">
        <UButton
          to="/listings"
          color="primary"
          variant="outline"
          class="min-h-11"
        >
          {{ t('landing.market.view_all') }}
        </UButton>
      </div>
    </div>

    <div
      v-else
      class="flex flex-col items-center gap-6 rounded-lg border border-default bg-elevated p-8 text-center"
    >
      <h3 class="text-xl font-semibold text-highlighted">
        {{ t('landing.market.empty_title') }}
      </h3>
      <p class="max-w-lg text-base text-muted">
        {{ t('landing.market.empty_body') }}
      </p>
      <div class="flex flex-col gap-4 sm:flex-row">
        <UButton
          to="/listings"
          color="primary"
          class="min-h-11"
        >
          {{ t('landing.market.view_all') }}
        </UButton>
        <UButton
          to="/account/listings/new"
          color="neutral"
          variant="outline"
          class="min-h-11"
        >
          {{ t('landing.market.publish') }}
        </UButton>
      </div>
    </div>
  </UPageSection>
</template>
