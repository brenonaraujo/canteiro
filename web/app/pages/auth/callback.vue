<script setup lang="ts">
import { postAuthPath } from '~/composables/auth/account'
import { useAuth } from '~/composables/auth/useAuth'

defineOptions({ name: 'AuthCallbackPage' })

const { t } = useI18n()
const route = useRoute()
const { completeCallback, errorKey, isAuthenticated, isProfileComplete } = useAuth()

onMounted(async () => {
  const result = await completeCallback(route.query as Record<string, unknown>)
  await navigateTo(postAuthPath({
    hasError: Boolean(result.error || errorKey.value),
    authenticated: isAuthenticated.value,
    profileComplete: isProfileComplete.value
  }))
})
</script>

<template>
  <UContainer class="py-24">
    <div class="mx-auto flex max-w-md flex-col items-center gap-4 text-center">
      <UIcon
        name="i-lucide-loader-circle"
        class="size-8 animate-spin text-primary"
      />
      <p class="text-base text-muted">
        {{ t('auth.callback.working') }}
      </p>
    </div>
  </UContainer>
</template>
