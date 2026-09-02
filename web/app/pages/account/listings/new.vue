<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useListingForm } from '~/composables/listing/useListingForm'
import { useListingList } from '~/composables/listing/useListingList'
import { useListingOwner } from '~/composables/listing/useListingOwner'
import { toCreateInput } from '~/composables/listing/validation'

defineOptions({ name: 'AccountListingNewPage' })

const { t } = useI18n()
const toast = useToast()

const {
  state, errors, isValid, isHeavy, needsOperatorIdentity, publishGate,
  hydrate, setOwner, setCategories
} = useListingForm()

const {
  saveOnboarding, loadOnboarding, create, publish, errorKey, saving,
  publishGates, onboarding
} = useListingOwner()

const { categories, loadCategories } = useListingList()

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.account_listings'), to: '/account/listings' },
  { label: t('breadcrumb.listing_new') }
])

useHead({ title: () => t('listing.new.title') })

onMounted(async () => {
  await loadCategories()
  setCategories(categories.value)
  await loadOnboarding()
  setOwner(onboarding.value)
})

async function onSaveDraft() {
  const result = toCreateInput(state)
  const created = await create(result)
  if (created) {
    toast.add({
      title: t('listing.new.draft_saved'),
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    await navigateTo(`/account/listings/${created.id}/edit`)
  }
}

async function onSaveOnboarding() {
  await saveOnboarding({
    payout_kind: 'pix',
    payout_last4: '0000',
    accept_terms: true,
    terms_version: 'v1'
  })
  setOwner(onboarding.value)
}

async function onPublish() {
  if (!isValid.value) {
    toast.add({
      title: t('listing.new.publish_invalid'),
      color: 'error',
      icon: 'i-lucide-alert-triangle'
    })
    return
  }
  const result = toCreateInput(state)
  const created = await create(result)
  if (!created) {
    return
  }
  const published = await publish(created.id)
  if (published) {
    toast.add({
      title: t('listing.new.published'),
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    await navigateTo('/account/listings')
  }
}

definePageMeta({ middleware: 'auth' })
</script>

<template>
  <UContainer class="py-12">
    <div class="flex flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />

      <header class="flex flex-col gap-2">
        <h1 class="text-3xl font-bold tracking-tight text-highlighted">
          {{ t('listing.new.title') }}
        </h1>
        <p class="text-base text-muted">
          {{ t('listing.new.subtitle') }}
        </p>
      </header>

      <UAlert
        v-if="publishGate.reason === 'publish.error.terms_not_accepted'
          || publishGate.reason === 'publish.error.payout_not_set'"
        color="warning"
        variant="subtle"
        icon="i-lucide-shield-alert"
        :title="t(publishGate.reason ?? '')"
      >
        <template #actions>
          <UButton
            color="primary"
            size="sm"
            class="min-h-11"
            @click="onSaveOnboarding"
          >
            {{ t('listing.new.accept_terms_button') }}
          </UButton>
        </template>
      </UAlert>

      <UAlert
        v-if="errorKey && publishGate.reason !== 'publish.error.terms_not_accepted'
          && publishGate.reason !== 'publish.error.payout_not_set'"
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
        :editable="true"
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
          color="neutral"
          variant="outline"
          class="min-h-11"
          :disabled="saving"
          @click="onSaveDraft"
        >
          {{ t('listing.new.save_draft') }}
        </UButton>
        <UButton
          color="primary"
          class="min-h-11"
          :disabled="!isValid || !publishGate.ok || saving"
          @click="onPublish"
        >
          {{ t('listing.new.publish') }}
        </UButton>
      </footer>
    </div>
  </UContainer>
</template>
