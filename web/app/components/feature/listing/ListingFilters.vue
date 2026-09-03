<script setup lang="ts">
import type {
  ListingSearchFilters
} from '~/composables/listing/types'

defineOptions({ name: 'ListingFilters' })

const props = defineProps<{
  modelValue: ListingSearchFilters
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ListingSearchFilters]
  'apply': [value: ListingSearchFilters]
}>()

const { t } = useI18n()

const local = reactive<ListingSearchFilters>({ ...props.modelValue })

watch(() => props.modelValue, (next) => {
  Object.assign(local, next)
})

const categoryOptions = computed(() => [
  { label: t('listing.filters.any'), value: null as string | null },
  { label: t('listing.category.manual'), value: 'manual' },
  { label: t('listing.category.electric'), value: 'electric' },
  { label: t('listing.category.light_construction'), value: 'light_construction' },
  { label: t('listing.category.agricultural'), value: 'agricultural' },
  { label: t('listing.category.heavy'), value: 'heavy' }
])

const sizeOptions = computed(() => [
  { label: t('listing.filters.any'), value: null as string | null },
  { label: t('listing.size.light'), value: 'light' },
  { label: t('listing.size.heavy'), value: 'heavy' }
])

const operatorOptions = computed(() => [
  { label: t('listing.filters.any'), value: null as string | null },
  { label: t('listing.operator_mode.none'), value: 'none' },
  { label: t('listing.operator_mode.optional'), value: 'optional' },
  { label: t('listing.operator_mode.required'), value: 'required' }
])

function emitUpdate() {
  emit('update:modelValue', { ...local })
}

function onApply() {
  emitUpdate()
  emit('apply', { ...local })
}

function onClear() {
  local.category = null
  local.size = null
  local.city = undefined
  local.from = undefined
  local.to = undefined
  local.operator_mode = null
  local.min_price_cents = undefined
  local.max_price_cents = undefined
  emitUpdate()
  emit('apply', { ...local })
}
</script>

<template>
  <UForm
    class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4"
    @submit.prevent="onApply"
  >
    <UFormField
      :label="t('listing.filters.category')"
      name="category"
    >
      <USelect
        v-model="local.category"
        :items="categoryOptions"
        value-key="value"
        class="min-h-11"
        @update:model-value="emitUpdate"
      />
    </UFormField>

    <UFormField
      :label="t('listing.filters.size')"
      name="size"
    >
      <USelect
        v-model="local.size"
        :items="sizeOptions"
        value-key="value"
        class="min-h-11"
        @update:model-value="emitUpdate"
      />
    </UFormField>

    <UFormField
      :label="t('listing.filters.operator_mode')"
      name="operator_mode"
    >
      <USelect
        v-model="local.operator_mode"
        :items="operatorOptions"
        value-key="value"
        class="min-h-11"
        @update:model-value="emitUpdate"
      />
    </UFormField>

    <UFormField
      :label="t('listing.filters.city')"
      name="city"
    >
      <UInput
        v-model="local.city"
        :placeholder="t('listing.filters.city_placeholder')"
        class="min-h-11"
      />
    </UFormField>

    <UFormField
      :label="t('listing.filters.from')"
      name="from"
    >
      <UInput
        v-model="local.from"
        type="date"
        class="min-h-11"
      />
    </UFormField>

    <UFormField
      :label="t('listing.filters.to')"
      name="to"
    >
      <UInput
        v-model="local.to"
        type="date"
        class="min-h-11"
      />
    </UFormField>

    <UFormField
      :label="t('listing.filters.price_min')"
      name="min_price_cents"
    >
      <UInput
        v-model.number="local.min_price_cents"
        type="number"
        min="0"
        class="min-h-11"
      />
    </UFormField>

    <UFormField
      :label="t('listing.filters.price_max')"
      name="max_price_cents"
    >
      <UInput
        v-model.number="local.max_price_cents"
        type="number"
        min="0"
        class="min-h-11"
      />
    </UFormField>

    <div class="flex items-end gap-2 lg:col-span-4 lg:justify-end">
      <UButton
        type="button"
        color="neutral"
        variant="ghost"
        class="min-h-11"
        @click="onClear"
      >
        {{ t('listing.filters.clear') }}
      </UButton>
      <UButton
        type="submit"
        color="primary"
        class="min-h-11"
        :aria-label="t('listing.filters.apply')"
      >
        {{ t('listing.filters.apply') }}
      </UButton>
    </div>
  </UForm>
</template>
