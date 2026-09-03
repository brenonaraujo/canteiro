<script setup lang="ts">
defineOptions({ name: 'AppHeader' })

const { t } = useI18n()

const navItems = computed(() => [
  { label: t('nav.home'), to: '/' },
  { label: t('breadcrumb.listings'), to: '/listings' },
  { label: t('nav.highlights'), to: '/#highlights' }
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
      <AccountMenu />
      <LocaleSwitcher />
      <UColorModeButton :aria-label="t('a11y.color_mode')" />
    </template>

    <template #body>
      <UNavigationMenu
        :items="navItems"
        orientation="vertical"
        class="-mx-2"
      />
      <div class="mt-4 flex flex-col gap-4">
        <UButton
          to="/auth/login"
          color="neutral"
          variant="ghost"
          class="min-h-11 justify-start"
        >
          {{ t('nav.sign_in') }}
        </UButton>
        <div class="flex items-center gap-2">
          <LocaleSwitcher />
          <UColorModeButton :aria-label="t('a11y.color_mode')" />
        </div>
      </div>
    </template>
  </UHeader>
</template>
