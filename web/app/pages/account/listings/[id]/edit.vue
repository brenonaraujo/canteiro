<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useListingForm } from '~/composables/listing/useListingForm'
import { useListingList } from '~/composables/listing/useListingList'
import { useListingOwner } from '~/composables/listing/useListingOwner'
import { toCreateInput } from '~/composables/listing/validation'

defineOptions({ name: 'AccountListingEditPage' })

const route = useRoute()
const { t } = useI18n()
const toast = useToast()

const {
  state, errors, isValid, isHeavy, needsOperatorIdentity,
  hydrate, hydrateFromListing, setOwner, setCategories, publishGate
} = useListingForm()

const {
  current, loadOne, update, publish, pause, errorKey,
  saving, publishGates, loadOnboarding, onboarding
} = useListingOwner()

const { categories, loadCategories } = useListingList()

const id = computed(() => String(route.params.id ?? ''))

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.account_listings'), to: '/account/listings' },
  { label: current.value?.title ?? t('breadcrumb.listing_edit') }
])

useHead(() => ({
  title: () => current.value?.title ?? t('listing.edit.title')
}))

const editable = computed(() => {
  if (!current.value) return true
  return current.value.state !== 'published'
})

onMounted(async () => {
  await Promise.all([loadCategories(), loadOne(id.value), loadOnboarding()])
  setCategories(categories.value)
  setOwner(onboarding.value)
  if (current.value) {
    hydrateFromListing(current.value)
  }
})

async function onSave() {
  const result = toCreateInput(state)
  const updated = await update(id.value, result)
  if (updated) {
    toast.add({
      title: t('listing.edit.saved'),
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
  }
}

async function onPublish() {
  const result = toCreateInput(state)
  const updated = await update(id.value, result)
  if (!updated) {
    return
  }
  const published = await publish(id.value)
  if (published) {
    toast.add({
      title: t('listing.edit.published'),
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    await navigateTo('/account/listings')
  }
}

async function onPause() {
  const paused = await pause(id.value)
  if (paused) {
    toast.add({
      title: t('listing.edit.paused'),
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
  }
}

definePageMeta({ middleware: 'auth' })
</script>

<template>
  <UContainer class="py-12">
    <div class="flex flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />

      <header
        v-if="current"
        class="flex flex-wrap items-end justify-between gap-4"
      >
        <div class="flex flex-col gap-2">
          <h1 class="text-3xl font-bold tracking-tight text-highlighted">
            {{ current.title }}
          </h1>
          <ListingStateBadge :state="current.state" />
        </div>
      </header>

      <UAlert
        v-if="!editable"
        color="info"
        variant="subtle"
        icon="i-lucide-info"
        :title="t('listing.edit.published_locked')"
        :description="t('listing.edit.published_locked_desc')"
      />

      <UAlert
        v-if="errorKey"
        color="error"
        variant="subtle"
        icon="i-lucide-alert-triangle"
        :title="t(errorKey)"
        :description="publishGates.length > 0
          ? publishGates.map((k: string) => t(k)).join(' · ')
          : undefined"
      />

      <ListingForm
        :form="state"
        :errors="errors"
        :is-heavy="isHeavy"
        :needs-operator-identity="needsOperatorIdentity"
        :needs-operator-rate="false"
        :editable="editable"
        :show-rules="true"
        @update:form="(next) => hydrate(next)"
      />

      <footer class="flex flex-wrap items-center justify-end gap-4">
        <UButton
          to="/account/listings"
          color="neutral"
          variant="ghost"
          class="min-h-11"
        >
          {{ t('common.cancel') }}
        </UButton>

        <UButton
          v-if="current?.state === 'published'"
          color="warning"
          variant="soft"
          class="min-h-11"
          :disabled="saving"
          @click="onPause"
        >
          {{ t('listing.edit.pause') }}
        </UButton>

        <UButton
          v-if="editable"
          color="neutral"
          variant="outline"
          class="min-h-11"
          :disabled="saving"
          @click="onSave"
        >
          {{ t('listing.edit.save') }}
        </UButton>

        <UButton
          v-if="editable && current?.state !== 'published'"
          color="primary"
          class="min-h-11"
          :disabled="!isValid || !publishGate.ok || saving"
          @click="onPublish"
        >
          {{ t('listing.edit.publish') }}
        </UButton>
      </footer>
    </div>
  </UContainer>
</template>
