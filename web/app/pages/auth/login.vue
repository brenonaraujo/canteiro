<script setup lang="ts">
import { useAuth } from '~/composables/auth/useAuth'

defineOptions({ name: 'AuthLoginPage' })

const { t } = useI18n()
const { startGoogle, errorKey, pending } = useAuth()

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.sign_in') }
])
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
            v-if="errorKey"
            color="warning"
            variant="subtle"
            icon="i-lucide-alert-triangle"
            :title="t(errorKey)"
          />
          <UAlert
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
            to="/"
            color="neutral"
            variant="ghost"
            class="min-h-11"
          >
            {{ t('nav.home') }}
          </UButton>
        </div>
      </UPageCard>
    </div>
  </UContainer>
</template>
