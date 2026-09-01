<script setup lang="ts">
defineOptions({ name: 'HomeSoon' })

const { t } = useI18n()
</script>

<template>
  <UPageSection id="soon">
    <UPageCTA
      :title="t('landing.soon.title')"
      :description="t('landing.soon.body')"
      variant="subtle"
      :links="[]"
    />
  </UPageSection>
</template>
