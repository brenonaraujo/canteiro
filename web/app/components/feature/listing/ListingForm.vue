<script setup lang="ts">
import type {
  ListingDraftErrors,
  ListingFormState
} from '~/composables/listing/types'

defineOptions({ name: 'ListingForm' })

const props = defineProps<{
  form: ListingFormState
  errors: ListingDraftErrors
  isHeavy: boolean
  needsOperatorIdentity: boolean
  needsOperatorRate: boolean
  editable: boolean
  showRules?: boolean
}>()

const emit = defineEmits<{
  'update:form': [form: ListingFormState]
  'update:errors': [errors: ListingDraftErrors]
}>()

function patchForm<K extends keyof ListingFormState>(
  key: K,
  value: ListingFormState[K]
) {
  emit('update:form', { ...props.form, [key]: value })
}

defineExpose({ patchForm })
</script>

<template>
  <div class="flex flex-col gap-8">
    <section class="flex flex-col gap-4">
      <h2 class="text-lg font-semibold text-highlighted">
        {{ $t('listing.form.section_basic') }}
      </h2>

      <UFormField
        :label="$t('listing.form.title')"
        :error="errors.title ? $t(errors.title) : undefined"
        required
      >
        <UInput
          :model-value="form.title"
          :disabled="!editable"
          :placeholder="$t('listing.form.title_placeholder')"
          class="min-h-11"
          maxlength="120"
          @update:model-value="(v: string) => patchForm('title', v)"
        />
      </UFormField>

      <UFormField
        :label="$t('listing.form.description')"
        :error="errors.description ? $t(errors.description) : undefined"
        required
      >
        <UTextarea
          :model-value="form.description"
          :disabled="!editable"
          :placeholder="$t('listing.form.description_placeholder')"
          :rows="5"
          @update:model-value="(v: string) => patchForm('description', v)"
        />
      </UFormField>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <UFormField
          :label="$t('listing.form.category')"
          required
        >
          <USelect
            :model-value="form.category"
            :items="[
              { label: $t('listing.category.manual'), value: 'manual' },
              { label: $t('listing.category.electric'), value: 'electric' },
              { label: $t('listing.category.light_construction'), value: 'light_construction' },
              { label: $t('listing.category.agricultural'), value: 'agricultural' },
              { label: $t('listing.category.heavy'), value: 'heavy' }
            ]"
            value-key="value"
            :disabled="!editable"
            class="min-h-11"
            @update:model-value="(v: string) => patchForm('category', v as ListingFormState['category'])"
          />
        </UFormField>

        <UFormField
          :label="$t('listing.form.photos')"
          :error="errors.photos ? $t(errors.photos) : undefined"
          :hint="$t('listing.form.photos_hint')"
          required
        >
          <UInput
            :model-value="form.photos[0] ?? ''"
            :disabled="!editable"
            :placeholder="$t('listing.form.photos_placeholder')"
            class="min-h-11"
            @update:model-value="(v: string) => patchForm('photos', v ? [v] : [])"
          />
        </UFormField>
      </div>
    </section>

    <section class="flex flex-col gap-4">
      <h2 class="text-lg font-semibold text-highlighted">
        {{ $t('listing.form.section_location') }}
      </h2>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <UFormField
          :label="$t('listing.form.pickup_city')"
          :error="errors.pickup_city ? $t(errors.pickup_city) : undefined"
          required
        >
          <UInput
            :model-value="form.pickup_city"
            :disabled="!editable"
            class="min-h-11"
            @update:model-value="(v: string) => patchForm('pickup_city', v)"
          />
        </UFormField>

        <UFormField
          :label="$t('listing.form.pickup_neighborhood')"
          :error="errors.pickup_neighborhood
            ? $t(errors.pickup_neighborhood) : undefined"
        >
          <UInput
            :model-value="form.pickup_neighborhood"
            :disabled="!editable"
            class="min-h-11"
            @update:model-value="(v: string) => patchForm('pickup_neighborhood', v)"
          />
        </UFormField>
      </div>

      <UCheckbox
        :model-value="form.delivery_enabled"
        :disabled="!editable"
        :label="$t('listing.form.delivery_enabled')"
        @update:model-value="(v) => patchForm('delivery_enabled', v === true)"
      />

      <UFormField
        v-if="form.delivery_enabled"
        :label="$t('listing.form.delivery_coverage')"
        :error="errors.delivery_coverage
          ? $t(errors.delivery_coverage) : undefined"
        required
      >
        <UTextarea
          :model-value="form.delivery_coverage"
          :disabled="!editable"
          :rows="3"
          :placeholder="$t('listing.form.delivery_coverage_placeholder')"
          @update:model-value="(v: string) => patchForm('delivery_coverage', v)"
        />
      </UFormField>
    </section>

    <section class="flex flex-col gap-4">
      <h2 class="text-lg font-semibold text-highlighted">
        {{ $t('listing.form.section_pricing') }}
      </h2>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
        <UFormField
          :label="$t('listing.form.price_unit')"
          required
        >
          <USelect
            :model-value="form.price_unit"
            :items="[
              { label: $t('listing.form.per_day'), value: 'day' },
              { label: $t('listing.form.per_hour'), value: 'hour' }
            ]"
            value-key="value"
            :disabled="!editable"
            class="min-h-11"
            @update:model-value="(v: string) => patchForm('price_unit', v as ListingFormState['price_unit'])"
          />
        </UFormField>

        <UFormField
          :label="$t('listing.form.price_amount')"
          :error="errors.price_amount_cents
            ? $t(errors.price_amount_cents) : undefined"
          required
        >
          <UInput
            :model-value="form.price_amount_cents"
            type="number"
            :min="1"
            :disabled="!editable"
            class="min-h-11"
            @update:model-value="(v: number) => patchForm('price_amount_cents', Number(v) || 0)"
          />
        </UFormField>

        <UFormField
          :label="$t('listing.form.deposit')"
          :error="errors.deposit_cents ? $t(errors.deposit_cents) : undefined"
          required
        >
          <UInput
            :model-value="form.deposit_cents"
            type="number"
            :min="0"
            :disabled="!editable"
            class="min-h-11"
            @update:model-value="(v: number) => patchForm('deposit_cents', Number(v) || 0)"
          />
        </UFormField>
      </div>

      <UFormField
        :label="$t('listing.form.min_lead_time_hours')"
        :error="errors.min_lead_time_hours
          ? $t(errors.min_lead_time_hours) : undefined"
      >
        <UInput
          :model-value="form.min_lead_time_hours"
          type="number"
          :min="0"
          :disabled="!editable"
          class="min-h-11"
          @update:model-value="(v: number) => patchForm('min_lead_time_hours', Number(v) || 0)"
        />
      </UFormField>
    </section>

    <section
      v-if="showRules"
      class="flex flex-col gap-4"
    >
      <h2 class="text-lg font-semibold text-highlighted">
        {{ $t('listing.form.section_rules') }}
      </h2>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <UCheckbox
          :model-value="form.rules_document_required"
          :disabled="!editable"
          :label="$t('listing.form.rules_document_required')"
          @update:model-value="(v) => patchForm('rules_document_required', v === true)"
        />
        <UCheckbox
          :model-value="form.rules_experience_required"
          :disabled="!editable"
          :label="$t('listing.form.rules_experience_required')"
          @update:model-value="(v) => patchForm('rules_experience_required', v === true)"
        />
        <UCheckbox
          :model-value="form.rules_travel_restricted"
          :disabled="!editable"
          :label="$t('listing.form.rules_travel_restricted')"
          @update:model-value="(v) => patchForm('rules_travel_restricted', v === true)"
        />
        <UFormField :label="$t('listing.form.rules_min_age')">
          <UInput
            :model-value="form.rules_min_age"
            type="number"
            :min="0"
            :max="130"
            :disabled="!editable"
            class="min-h-11"
            @update:model-value="(v: number) => patchForm('rules_min_age', Number(v) || 0)"
          />
        </UFormField>
      </div>
    </section>

    <section class="flex flex-col gap-4">
      <h2 class="text-lg font-semibold text-highlighted">
        {{ $t('listing.form.section_operator') }}
      </h2>

      <UFormField
        :label="$t('listing.form.operator_mode')"
        required
      >
        <USelect
          :model-value="form.operator_mode"
          :items="[
            { label: $t('listing.operator_mode.none'), value: 'none' },
            { label: $t('listing.operator_mode.optional'), value: 'optional' },
            { label: $t('listing.operator_mode.required'), value: 'required' }
          ]"
          value-key="value"
          :disabled="!editable"
          class="min-h-11"
          @update:model-value="(v: string) => patchForm('operator_mode', v as ListingFormState['operator_mode'])"
        />
      </UFormField>

      <template v-if="form.operator_mode !== 'none'">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <UFormField
            :label="$t('listing.form.operator_hourly_rate')"
            :error="errors.operator_hourly_rate_cents
              ? $t(errors.operator_hourly_rate_cents) : undefined"
            required
          >
            <UInput
              :model-value="form.operator_hourly_rate_cents"
              type="number"
              :min="0"
              :disabled="!editable"
              class="min-h-11"
              @update:model-value="(v: number) => patchForm('operator_hourly_rate_cents', Number(v) || 0)"
            />
          </UFormField>

          <UFormField
            :label="$t('listing.form.operator_min_hours')"
          >
            <UInput
              :model-value="form.operator_min_hours"
              type="number"
              :min="1"
              :disabled="!editable"
              class="min-h-11"
              @update:model-value="(v: number) => patchForm('operator_min_hours', Number(v) || 0)"
            />
          </UFormField>
        </div>

        <template v-if="form.operator_mode === 'required'">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <UFormField
              :label="$t('listing.form.operator_name')"
              :error="errors.operator_identity
                ? $t(errors.operator_identity) : undefined"
              required
            >
              <UInput
                :model-value="form.operator_name"
                :disabled="!editable"
                class="min-h-11"
                @update:model-value="(v: string) => patchForm('operator_name', v)"
              />
            </UFormField>

            <UFormField
              :label="$t('listing.form.operator_phone')"
              required
            >
              <UInput
                :model-value="form.operator_phone"
                :disabled="!editable"
                class="min-h-11"
                @update:model-value="(v: string) => patchForm('operator_phone', v)"
              />
            </UFormField>
          </div>

          <UCheckbox
            :model-value="form.operator_is_owner"
            :disabled="!editable"
            :label="$t('listing.form.operator_is_owner')"
            @update:model-value="(v) => patchForm('operator_is_owner', v === true)"
          />
        </template>
      </template>
    </section>

    <section
      v-if="isHeavy"
      class="flex flex-col gap-4"
    >
      <h2 class="text-lg font-semibold text-highlighted">
        {{ $t('listing.form.section_heavy') }}
      </h2>

      <UCheckbox
        :model-value="form.heavy_legal_cession"
        :disabled="!editable"
        :label="$t('listing.form.heavy_legal_cession')"
        :description="$t('listing.form.heavy_legal_cession_desc')"
        @update:model-value="(v) => patchForm('heavy_legal_cession', v === true)"
      />
    </section>
  </div>
</template>
