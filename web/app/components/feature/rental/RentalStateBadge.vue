<script setup lang="ts">
import type { RentalState } from '~/composables/rental/types'

defineOptions({ name: 'RentalStateBadge' })

const props = defineProps<{
  state: RentalState
}>()

const { t } = useI18n()

const meta = computed(() => {
  switch (props.state) {
    case 'pending':
    case 'authorized':
      return { color: 'warning' as const, icon: 'i-lucide-clock' }
    case 'confirmed':
      return { color: 'success' as const, icon: 'i-lucide-check-circle-2' }
    case 'cancelled':
    case 'cancellation_in_progress':
      return { color: 'error' as const, icon: 'i-lucide-ban' }
    case 'declined':
      return { color: 'neutral' as const, icon: 'i-lucide-x-circle' }
    case 'expired':
      return { color: 'neutral' as const, icon: 'i-lucide-timer-off' }
    case 'refunded':
      return { color: 'info' as const, icon: 'i-lucide-receipt-refund' }
    default:
      return { color: 'neutral' as const, icon: 'i-lucide-circle' }
  }
})
</script>

<template>
  <UBadge
    :color="meta.color"
    :icon="meta.icon"
    :label="t(`rental.state.${state}`)"
  />
</template>
