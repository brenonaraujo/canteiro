<script setup lang="ts">
defineOptions({ name: 'LocaleSwitcher' })

const { locale, locales, setLocale, t } = useI18n()

const items = computed(() =>
  locales.value.map(item => ({
    label: item.name,
    value: item.code
  }))
)

async function onLocaleChange(code: unknown) {
  if (typeof code !== 'string' || !isAppLocale(code)) {
    return
  }
  await setLocale(code)
}
</script>

<template>
  <USelect
    :model-value="locale"
    :items="items"
    value-key="value"
    :aria-label="t('locale.label')"
    class="w-36"
    @update:model-value="onLocaleChange"
  />
</template>
