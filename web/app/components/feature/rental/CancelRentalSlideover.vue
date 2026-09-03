<script setup lang="ts">
import type { RentalActor } from '~/composables/rental/types'

defineOptions({ name: 'CancelRentalSlideover' })

const props = defineProps<{
  modelValue: boolean
  actor: RentalActor
  busy: boolean
  errorKey?: string | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'submit': []
}>()

const { t } = useI18n()

const title = computed(() => {
  return props.actor === 'tenant'
    ? t('rental.confirm_cancel.title_tenant')
    : t('rental.confirm_cancel.title_owner')
})

const body = computed(() => {
  return props.actor === 'tenant'
    ? t('rental.confirm_cancel.body_tenant')
    : t('rental.confirm_cancel.body_owner')
})

const submitLabel = computed(() => {
  if (props.busy) {
    return t('rental.confirm_cancel.submit_pending')
  }
  return props.actor === 'tenant'
    ? t('rental.confirm_cancel.submit_tenant')
    : t('rental.confirm_cancel.submit_owner')
})

function close() {
  if (props.busy) {
    return
  }
  emit('update:modelValue', false)
}

function onSubmit() {
  if (props.busy) {
    return
  }
  emit('submit')
}
</script>

<template>
  <USlideover
    :open="modelValue"
    @update:open="(value: boolean) => emit('update:modelValue', value)"
  >
    <template #content>
      <div class="flex h-full w-full max-w-md flex-col gap-6 p-6">
        <header class="flex flex-col gap-2">
          <h2 class="text-lg font-semibold text-highlighted">
            {{ title }}
          </h2>
          <p class="text-sm text-muted">
            {{ body }}
          </p>
        </header>

        <UAlert
          v-if="errorKey"
          color="error"
          variant="subtle"
          icon="i-lucide-alert-triangle"
          :title="t(errorKey)"
        />

        <footer class="mt-auto flex flex-col gap-4 sm:flex-row sm:justify-end">
          <UButton
            color="neutral"
            variant="ghost"
            class="min-h-11"
            :disabled="busy"
            @click="close"
          >
            {{ t('common.cancel') }}
          </UButton>
          <UButton
            color="error"
            class="min-h-11"
            :loading="busy"
            :disabled="busy"
            @click="onSubmit"
          >
            {{ submitLabel }}
          </UButton>
        </footer>
      </div>
    </template>
  </USlideover>
</template>
