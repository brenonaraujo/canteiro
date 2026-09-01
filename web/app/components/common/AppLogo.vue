<script setup lang="ts">
defineOptions({ name: 'AppLogo' })
</script>

<template>
  <span class="inline-flex items-center gap-2 text-highlighted">
    <UIcon
      name="i-lucide-hammer"
      class="size-6 text-primary"
    />
    <span class="text-base font-semibold tracking-tight">
      {{ $t('common.app_name') }}
    </span>
  </span>
</template>
