<script setup lang="ts">
import { useAuth } from '~/composables/auth/useAuth'

defineOptions({ name: 'AuthDeactivatePage' })

const { t } = useI18n()
const { hydrate, deactivate, isAuthenticated, pending } = useAuth()
const confirmed = ref(false)
const failed = ref(false)

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.account'), to: '/auth/profile' },
  { label: t('breadcrumb.deactivate') }
])

onMounted(async () => {
  await hydrate()
  if (!isAuthenticated.value) {
    await navigateTo('/auth/login')
  }
})

async function onDeactivate() {
  if (!confirmed.value) {
    return
  }
  failed.value = false
  try {
    await deactivate()
  } catch {
    failed.value = true
  }
}
</script>

<template>
  <UContainer class="py-12">
    <div class="mx-auto flex max-w-md flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />
      <div class="flex flex-col gap-2">
        <h1 class="text-3xl font-bold tracking-tight text-highlighted">
          {{ t('auth.deactivate.title') }}
        </h1>
        <p class="text-base text-muted">
          {{ t('auth.deactivate.body') }}
        </p>
      </div>
      <UAlert
        color="warning"
        icon="i-lucide-alert-triangle"
        :description="t('auth.deactivate.listings')"
      />
      <UAlert
        color="info"
        icon="i-lucide-info"
        :description="t('auth.deactivate.rentals')"
      />
      <UAlert
        v-if="failed"
        color="error"
        icon="i-lucide-alert-triangle"
        :title="t('auth.error.generic')"
      />
      <UCheckbox
        v-model="confirmed"
        :label="t('auth.deactivate.irreversible')"
      />
      <div class="flex flex-col gap-4 sm:flex-row sm:justify-end">
        <UButton
          to="/auth/profile"
          color="neutral"
          variant="ghost"
          class="min-h-11"
        >
          {{ t('auth.deactivate.cancel') }}
        </UButton>
        <UButton
          color="error"
          class="min-h-11"
          :disabled="!confirmed"
          :loading="pending"
          @click="onDeactivate"
        >
          {{ t('auth.deactivate.submit') }}
        </UButton>
      </div>
    </div>
  </UContainer>
</template>
