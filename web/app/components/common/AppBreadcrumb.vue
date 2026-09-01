<script setup lang="ts">
defineOptions({ name: 'AppBreadcrumb' })

defineProps<{
  items: Array<{ label: string, to?: string }>
}>()
</script>

<template>
  <nav
    aria-label="breadcrumb"
    class="mb-4"
  >
    <UBreadcrumb :items="items" />
  </nav>
</template>
