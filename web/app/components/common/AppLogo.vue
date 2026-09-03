<script setup lang="ts">
defineOptions({ name: 'AppLogo' })

const { t } = useI18n()
</script>

<template>
  <span
    class="inline-flex items-center gap-2 text-highlighted"
    :aria-label="t('a11y.logo')"
  >
    <svg
      class="size-8 text-primary"
      viewBox="0 0 32 32"
      aria-hidden="true"
    >
      <rect
        x="5"
        y="6"
        width="22"
        height="4"
        rx="1"
        fill="currentColor"
      />
      <rect
        x="5"
        y="14"
        width="22"
        height="4"
        rx="1"
        fill="currentColor"
      />
      <rect
        x="5"
        y="22"
        width="22"
        height="4"
        rx="1"
        fill="currentColor"
      />
      <rect
        x="9"
        y="6"
        width="3"
        height="20"
        fill="currentColor"
        opacity="0.65"
      />
      <rect
        x="20"
        y="6"
        width="3"
        height="20"
        fill="currentColor"
        opacity="0.65"
      />
    </svg>
    <span
      class="text-base font-semibold tracking-tight"
      aria-hidden="true"
    >
      {{ t('common.app_name') }}
    </span>
  </span>
</template>
