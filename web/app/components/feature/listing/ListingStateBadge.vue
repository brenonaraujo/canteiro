<script setup lang="ts">
import type { ListingState } from '~/composables/listing/types'

defineOptions({ name: 'ListingStateBadge' })

const props = defineProps<{
  state: ListingState
}>()

const { t } = useI18n()

const meta = computed(() => {
  if (props.state === 'published') {
    return { color: 'success' as const, icon: 'i-lucide-check-circle-2' }
  }
  if (props.state === 'paused') {
    return { color: 'warning' as const, icon: 'i-lucide-pause-circle' }
  }
  return { color: 'neutral' as const, icon: 'i-lucide-file-edit' }
})
</script>

<template>
  <UBadge
    :color="meta.color"
    variant="subtle"
    :icon="meta.icon"
    :label="t(`listing.state.${state}`)"
  />
</template>
