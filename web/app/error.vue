<script setup lang="ts">
defineOptions({ name: 'AppError' })

const props = defineProps<{
  error: { statusCode?: number }
}>()

const { t } = useI18n()

const title = computed(() => {
  if (props.error?.statusCode === 404) {
    return t('error.not_found')
  }
  return t('error.generic')
})
</script>

<template>
  <UApp>
    <UMain>
      <div class="flex flex-col items-center text-center gap-6 py-24 px-4">
        <h1 class="text-4xl font-bold tracking-tight text-highlighted m-0">
          {{ title }}
        </h1>
        <UButton
          to="/"
          color="primary"
          size="lg"
        >
          {{ t('error.home') }}
        </UButton>
      </div>
    </UMain>
  </UApp>
</template>
