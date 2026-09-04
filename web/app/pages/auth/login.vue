<script setup lang="ts">
import { googleAccessFailed } from '~/composables/auth/gate'
import { useAuth } from '~/composables/auth/useAuth'

defineOptions({ name: 'AuthLoginPage' })

const { t } = useI18n()
const route = useRoute()
const { startGoogle, errorKey, pending } = useAuth()

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.sign_in') }
])

const googleUnavailable = computed(() => {
  return route.query.error === 'not_configured'
})

const accessFailed = computed(() => {
  return googleAccessFailed(route.query as Record<string, unknown>)
})

const failureKey = computed(() => {
  return errorKey.value || (accessFailed.value ? 'auth.callback.error' : null)
})
</script>

<template>
  <UContainer class="py-12">
    <div class="mx-auto flex max-w-md flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />
      <UPageCard
        :title="t('auth.login.title')"
        :description="t('auth.login.description')"
        variant="subtle"
      >
        <div class="flex flex-col gap-6">
          <UAlert
            v-if="failureKey"
            color="warning"
            variant="subtle"
            icon="i-lucide-alert-triangle"
            :title="t(failureKey)"
            :description="t('auth.login.still_visitor')"
          />
          <UAlert
            v-else-if="googleUnavailable"
            color="warning"
            variant="subtle"
            icon="i-lucide-circle-alert"
            :title="t('auth.login.unavailable_title')"
            :description="t('auth.login.unavailable_body')"
          />
          <UAlert
            v-else
            color="neutral"
            variant="subtle"
            icon="i-lucide-info"
            :description="t('auth.login.google_only')"
          />
          <UButton
            color="primary"
            size="lg"
            icon="i-lucide-log-in"
            class="min-h-11"
            block
            :loading="pending"
            :disabled="pending"
            @click="startGoogle"
          >
            {{ t('auth.login.google') }}
          </UButton>
          <p class="text-sm text-muted">
            {{ t('auth.login.catalog_hint') }}
          </p>
          <UButton
            to="/listings"
            color="neutral"
            variant="outline"
            class="min-h-11"
          >
            {{ t('auth.login.browse_catalog') }}
          </UButton>
        </div>
      </UPageCard>
    </div>
  </UContainer>
</template>
