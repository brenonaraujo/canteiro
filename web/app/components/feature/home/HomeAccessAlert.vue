<script setup lang="ts">
import { googleAccessFailed } from '~/composables/auth/gate'
import { useAuth } from '~/composables/auth/useAuth'

defineOptions({ name: 'HomeAccessAlert' })

const { t } = useI18n()
const route = useRoute()
const { errorKey } = useAuth()

const failed = computed(() => {
  if (errorKey.value === 'auth.callback.error') {
    return true
  }
  return googleAccessFailed(route.query as Record<string, unknown>)
})
</script>

<template>
  <UContainer
    v-if="failed"
    class="pt-8"
  >
    <UAlert
      color="warning"
      variant="subtle"
      icon="i-lucide-alert-triangle"
      :title="t('auth.callback.error')"
      :description="t('auth.login.still_visitor')"
    >
      <template #actions>
        <UButton
          to="/listings"
          color="neutral"
          variant="outline"
          class="min-h-11"
        >
          {{ t('auth.login.browse_catalog') }}
        </UButton>
      </template>
    </UAlert>
  </UContainer>
</template>
