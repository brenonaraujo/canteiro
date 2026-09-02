<script setup lang="ts">
import { postAuthPath } from '~/composables/auth/account'
import { useAuth } from '~/composables/auth/useAuth'

defineOptions({ name: 'AccountMenu' })

const { t } = useI18n()
const route = useRoute()
const {
  isAuthenticated,
  isProfileComplete,
  visibleName,
  logout,
  completeCallback
} = useAuth()

onMounted(async () => {
  const parsed = await completeCallback(route.query as Record<string, unknown>)
  if (parsed.sessionReady || parsed.accessToken) {
    await navigateTo(postAuthPath({
      hasError: false,
      authenticated: isAuthenticated.value,
      profileComplete: isProfileComplete.value
    }))
  }
})

const label = computed(() => {
  if (!isAuthenticated.value) {
    return t('nav.sign_in')
  }
  return visibleName.value || t('auth.profile.incomplete')
})
</script>

<template>
  <div class="flex items-center gap-2">
    <UButton
      v-if="!isAuthenticated"
      to="/auth/login"
      color="primary"
      size="md"
      class="min-h-11"
    >
      {{ t('nav.sign_in') }}
    </UButton>
    <template v-else>
      <UButton
        to="/auth/profile"
        color="neutral"
        variant="ghost"
        size="md"
        class="min-h-11"
      >
        {{ label }}
      </UButton>
      <UButton
        color="neutral"
        variant="ghost"
        size="md"
        class="min-h-11"
        @click="logout"
      >
        {{ t('nav.sign_out') }}
      </UButton>
    </template>
  </div>
</template>
