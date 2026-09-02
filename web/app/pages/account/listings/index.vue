<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useListingOwner } from '~/composables/listing/useListingOwner'

defineOptions({ name: 'AccountListingsIndexPage' })

const { t } = useI18n()
const {
  items, pending, errorKey, loadMine, publish, pause
} = useListingOwner()

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.account_listings') }
])

useHead({
  title: () => t('listing.owner.title')
})

onMounted(async () => {
  await loadMine()
})

async function onPublish(id: string) {
  await publish(id)
}

async function onPause(id: string) {
  await pause(id)
}
</script>

<template>
  <UContainer class="py-12">
    <div class="flex flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />

      <header class="flex flex-wrap items-end justify-between gap-4">
        <div class="flex flex-col gap-2">
          <h1 class="text-3xl font-bold tracking-tight text-highlighted">
            {{ t('listing.owner.title') }}
          </h1>
          <p class="text-base text-muted">
            {{ t('listing.owner.subtitle') }}
          </p>
        </div>
        <UButton
          to="/account/listings/new"
          color="primary"
          icon="i-lucide-plus"
          class="min-h-11"
        >
          {{ t('listing.owner.new') }}
        </UButton>
      </header>

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
          v-for="i in 3"
          :key="i"
          class="h-48 w-full"
        />
      </div>

      <div
        v-else-if="items.length > 0"
        class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
      >
        <OwnerListingCard
          v-for="listing in items"
          :key="listing.id"
          :listing="listing"
        >
          <template #actions="{ listing: item }">
            <UButton
              v-if="item.state === 'draft'"
              color="primary"
              variant="soft"
              size="sm"
              class="min-h-11"
              @click="onPublish(item.id)"
            >
              {{ t('listing.owner.publish') }}
            </UButton>
            <UButton
              v-else-if="item.state === 'published'"
              color="warning"
              variant="soft"
              size="sm"
              class="min-h-11"
              @click="onPause(item.id)"
            >
              {{ t('listing.owner.pause') }}
            </UButton>
            <UButton
              v-else-if="item.state === 'paused'"
              color="primary"
              variant="soft"
              size="sm"
              class="min-h-11"
              @click="onPublish(item.id)"
            >
              {{ t('listing.owner.republish') }}
            </UButton>
          </template>
        </OwnerListingCard>
      </div>

      <div
        v-else
        class="flex flex-col items-center gap-4 rounded-lg border border-dashed border-default bg-elevated p-12 text-center"
      >
        <UIcon
          name="i-lucide-package-plus"
          class="size-10 text-muted"
        />
        <h2 class="text-lg font-semibold text-highlighted">
          {{ t('listing.owner.empty_title') }}
        </h2>
        <p class="text-sm text-muted">
          {{ t('listing.owner.empty_body') }}
        </p>
        <UButton
          to="/account/listings/new"
          color="primary"
          class="min-h-11"
        >
          {{ t('listing.owner.create_first') }}
        </UButton>
      </div>
    </div>
  </UContainer>
</template>
