<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useListingPublic } from '~/composables/listing/useListingPublic'

defineOptions({ name: 'ListingDetailPage' })

const route = useRoute()
const { t } = useI18n()
const { listing, calendar, pending, errorKey, load } = useListingPublic()

const id = computed(() => String(route.params.id ?? ''))

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.listings'), to: '/listings' },
  { label: listing.value?.title ?? t('breadcrumb.listing') }
])

useHead(() => ({
  title: () => listing.value?.title ?? t('listing.ficha.title')
}))

onMounted(async () => {
  await load(id.value)
})
</script>

<template>
  <UContainer class="py-12">
    <div class="flex flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />

      <div
        v-if="pending"
        class="flex flex-col gap-4"
      >
        <USkeleton class="h-8 w-2/3" />
        <USkeleton class="h-6 w-1/3" />
        <USkeleton class="h-48 w-full" />
      </div>

      <UAlert
        v-else-if="errorKey"
        color="error"
        variant="subtle"
        icon="i-lucide-alert-triangle"
        :title="t(errorKey)"
      />

      <ListingFicha
        v-else-if="listing"
        :listing="listing"
        :calendar="calendar"
      />
    </div>
  </UContainer>
</template>
