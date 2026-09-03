<script setup lang="ts">
defineOptions({ name: 'HomeHero' })

const { t } = useI18n()

const catalogCta = computed(() => ({
  label: t('landing.hero.cta'),
  to: '/listings'
}))
</script>

<template>
  <section class="bg-muted">
    <UContainer class="py-16 sm:py-24">
      <div class="flex max-w-3xl flex-col gap-8">
        <UBadge
          color="primary"
          variant="subtle"
          :label="t('landing.hero.badge')"
          class="w-fit"
        />
        <h1 class="text-4xl font-bold tracking-tight text-highlighted sm:text-5xl">
          {{ t('landing.hero.title') }}
        </h1>
        <p class="text-lg text-muted">
          {{ t('landing.hero.tagline') }}
        </p>
        <div class="flex flex-col gap-4 sm:flex-row">
          <UButton
            :to="catalogCta.to"
            color="primary"
            size="xl"
            trailing-icon="i-lucide-arrow-right"
            class="min-h-11"
          >
            {{ catalogCta.label }}
          </UButton>
        </div>
      </div>
    </UContainer>
  </section>
</template>
