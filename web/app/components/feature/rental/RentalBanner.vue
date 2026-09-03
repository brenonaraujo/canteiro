<script setup lang="ts">
import type { RentalState } from '~/composables/rental/types'

defineOptions({ name: 'RentalBanner' })

const props = defineProps<{
  state: RentalState
  hasReceipt: boolean
}>()

const { t } = useI18n()

const banner = computed(() => {
  if (props.state === 'cancellation_in_progress') {
    return {
      color: 'warning' as const,
      icon: 'i-lucide-loader-circle',
      title: t('rental.banner.in_progress_title'),
      body: t('rental.banner.in_progress_body')
    }
  }
  if (props.hasReceipt) {
    return null
  }
  if (props.state === 'cancelled' || props.state === 'refunded') {
    return {
      color: 'neutral' as const,
      icon: 'i-lucide-receipt',
      title: t('rental.actions.in_dispute'),
      body: ''
    }
  }
  return null
})
</script>

<template>
  <UAlert
    v-if="banner"
    :color="banner.color"
    :icon="banner.icon"
    :title="banner.title"
    :description="banner.body || undefined"
  />
</template>
