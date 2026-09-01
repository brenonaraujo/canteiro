<script setup lang="ts">
defineOptions({ name: 'HomeHero' })

const { t } = useI18n()

const links = computed(() => [
  {
    label: t('landing.hero.cta'),
    to: '#soon',
    trailingIcon: 'i-lucide-arrow-right',
    size: 'xl' as const
  }
])
</script>

<template>
  <UPageHero
    :title="t('landing.hero.title')"
    :description="t('landing.hero.tagline')"
    :links="links"
  >
    <template #headline>
      <UBadge
        color="primary"
        variant="subtle"
        :label="t('landing.hero.badge')"
      />
    </template>
  </UPageHero>
</template>
