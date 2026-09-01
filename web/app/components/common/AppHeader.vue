<script setup lang="ts">
defineOptions({ name: 'AppHeader' })

const { t } = useI18n()

const navItems = computed(() => [
  { label: t('nav.home'), to: '/' },
  { label: t('nav.highlights'), to: '#highlights' }
])
</script>

<template>
  <UHeader>
    <template #left>
      <NuxtLink
        to="/"
        class="rounded-md p-1 -ms-1 focus-visible:outline-3 outline-primary/25"
      >
        <AppLogo />
      </NuxtLink>
    </template>

    <UNavigationMenu :items="navItems" />

    <template #right>
      <LocaleSwitcher />
      <UColorModeButton :aria-label="t('a11y.color_mode')" />
    </template>

    <template #body>
      <UNavigationMenu
        :items="navItems"
        orientation="vertical"
        class="-mx-2"
      />
      <div class="flex items-center gap-2 mt-4">
        <LocaleSwitcher />
        <UColorModeButton :aria-label="t('a11y.color_mode')" />
      </div>
    </template>
  </UHeader>
</template>
