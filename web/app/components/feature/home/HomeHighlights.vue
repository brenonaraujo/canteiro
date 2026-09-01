<script setup lang="ts">
defineOptions({ name: 'HomeHighlights' })

const { t } = useI18n()

const features = computed(() => [
  {
    icon: 'i-lucide-hammer',
    title: t('landing.highlights.owner.title'),
    description: t('landing.highlights.owner.desc')
  },
  {
    icon: 'i-lucide-search',
    title: t('landing.highlights.renter.title'),
    description: t('landing.highlights.renter.desc')
  },
  {
    icon: 'i-lucide-shield-check',
    title: t('landing.highlights.platform.title'),
    description: t('landing.highlights.platform.desc')
  }
])
</script>

<template>
  <UPageSection
    id="highlights"
    :title="t('landing.highlights.title')"
    :features="features"
  />
</template>
