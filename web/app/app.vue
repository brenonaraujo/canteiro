<script setup lang="ts">
const { t, locale } = useI18n()
const site = useSite()

useHead({
  htmlAttrs: {
    lang: locale
  },
  link: [
    { rel: 'icon', href: '/favicon.ico' }
  ]
})

useSeoMeta({
  title: () => t('common.app_name'),
  description: () => t('landing.hero.tagline'),
  ogTitle: () => t('common.app_name'),
  ogDescription: () => t('landing.hero.tagline'),
  ogUrl: site.url
})
</script>

<template>
  <UApp>
    <AppHeader />
    <UMain>
      <NuxtPage />
    </UMain>
    <AppFooter />
  </UApp>
</template>
