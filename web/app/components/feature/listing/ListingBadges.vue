<script setup lang="ts">
import type {
  ListingCategory,
  ListingSize,
  OperatorMode
} from '~/composables/listing/types'

defineOptions({ name: 'ListingBadges' })

defineProps<{
  category: ListingCategory
  size?: ListingSize
  operatorMode?: OperatorMode
}>()

const { t } = useI18n()

function categoryLabel(category: ListingCategory) {
  return t(`listing.category.${category}`)
}

function sizeLabel(size: ListingSize | undefined) {
  if (!size) return undefined
  return t(`listing.size.${size}`)
}

function operatorLabel(mode: OperatorMode | undefined) {
  if (!mode) return undefined
  return t(`listing.operator_mode.${mode}`)
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <UBadge
      color="primary"
      variant="subtle"
      :label="categoryLabel(category)"
    />
    <UBadge
      v-if="size"
      color="neutral"
      variant="outline"
      :label="sizeLabel(size)"
    />
    <UBadge
      v-if="operatorMode && operatorMode !== 'none'"
      color="info"
      variant="subtle"
      :label="operatorLabel(operatorMode)"
    />
  </div>
</template>
