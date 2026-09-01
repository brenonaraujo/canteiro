<script setup lang="ts">
defineOptions({ name: 'AppFooter' })

const { t } = useI18n()
const year = new Date().getFullYear()
</script>

<template>
  <USeparator />
  <UFooter>
    <template #left>
      <p class="text-sm text-muted">
        {{ t('footer.copyright', { year }) }}
      </p>
    </template>
    <template #right>
      <p class="text-sm text-muted">
        {{ t('footer.host') }}
      </p>
    </template>
  </UFooter>
</template>
