<script setup lang="ts">
import { guestRedirectTarget, sessionView } from '~/composables/auth/gate'
import { useAuth } from '~/composables/auth/useAuth'

defineOptions({ name: 'AuthProfilePage' })

const { t } = useI18n()
const toast = useToast()
const {
  account,
  errorKey,
  pending,
  isAuthenticated,
  hydrate,
  saveProfile
} = useAuth()

const state = reactive({
  displayName: '',
  phone: ''
})

const crumbs = computed(() => [
  { label: t('breadcrumb.home'), to: '/' },
  { label: t('breadcrumb.account') }
])

const view = computed(() => sessionView({
  authenticated: isAuthenticated.value,
  pending: pending.value
}))

onMounted(async () => {
  await hydrate()
  const target = guestRedirectTarget(view.value, true)
  if (target) {
    await navigateTo(target)
    return
  }
  state.displayName = account.value?.displayName ?? ''
  state.phone = account.value?.phone ?? ''
})

async function onSubmit() {
  const result = await saveProfile({
    displayName: state.displayName,
    phone: state.phone
  })
  if (!result.ok) {
    return
  }
  toast.add({
    title: t('auth.profile.saved'),
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}
</script>

<template>
  <UContainer
    v-if="view === 'form'"
    class="py-12"
  >
    <div class="mx-auto flex max-w-md flex-col gap-8">
      <AppBreadcrumb :items="crumbs" />
      <div class="flex flex-col gap-2">
        <h1 class="text-3xl font-bold tracking-tight text-highlighted">
          {{ t('auth.profile.title') }}
        </h1>
        <p class="text-base text-muted">
          {{ t('auth.profile.description') }}
        </p>
      </div>
      <UAlert
        v-if="errorKey"
        color="error"
        icon="i-lucide-alert-triangle"
        :title="t(errorKey)"
      />
      <UForm
        class="flex flex-col gap-6"
        @submit="onSubmit"
      >
        <UFormField
          name="displayName"
          :label="t('auth.profile.display_name')"
          required
        >
          <UInput
            v-model="state.displayName"
            class="w-full min-h-11"
            autocomplete="name"
          />
        </UFormField>
        <UFormField
          name="phone"
          :label="t('auth.profile.phone')"
          required
        >
          <UInput
            v-model="state.phone"
            type="tel"
            class="w-full min-h-11"
            autocomplete="tel"
          />
        </UFormField>
        <div class="flex flex-col gap-4 sm:flex-row sm:justify-end">
          <UButton
            to="/"
            color="neutral"
            variant="ghost"
            class="min-h-11"
          >
            {{ t('auth.profile.cancel') }}
          </UButton>
          <UButton
            type="submit"
            color="primary"
            class="min-h-11"
            :loading="pending"
          >
            {{ t('auth.profile.save') }}
          </UButton>
        </div>
      </UForm>
      <USeparator />
      <UButton
        to="/auth/deactivate"
        color="error"
        variant="ghost"
        class="min-h-11 self-start"
      >
        {{ t('auth.deactivate.title') }}
      </UButton>
    </div>
  </UContainer>
</template>
