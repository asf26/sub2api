<template>
  <div class="space-y-5" data-testid="xiao-video-config-editor">
    <section class="border-t border-gray-200 pt-4 dark:border-dark-600">
      <div class="mb-3 flex items-center justify-between gap-3">
        <label class="input-label mb-0">{{ t('admin.accounts.xiaoapi.modelMapping') }}</label>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-1.5 text-sm" data-testid="xiao-add-mapping" @click="addMapping">
          <Icon name="plus" size="xs" />
          {{ t('admin.accounts.xiaoapi.addMapping') }}
        </button>
      </div>
      <div v-if="mappings.length > 0" class="space-y-2">
        <div v-for="(mapping, index) in mappings" :key="index" class="grid grid-cols-[minmax(0,1fr)_2.5rem] items-end gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.5rem]">
          <label class="col-span-2 min-w-0 text-xs text-gray-500 dark:text-gray-400 sm:col-span-1">
            {{ t('admin.accounts.xiaoapi.publicModel') }}
            <input v-model="mapping.from" type="text" class="input mt-1" :data-testid="`xiao-mapping-public-${index}`" />
          </label>
          <label class="min-w-0 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.xiaoapi.upstreamModel') }}
            <input v-model="mapping.to" type="text" class="input mt-1" :data-testid="`xiao-mapping-upstream-${index}`" />
          </label>
          <button type="button" class="btn btn-secondary flex h-10 w-10 items-center justify-center p-0 text-red-600" :title="t('common.delete')" @click="mappings.splice(index, 1)">
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </div>
    </section>

    <section class="border-t border-gray-200 pt-4 dark:border-dark-600">
      <div class="mb-3 flex items-center justify-between gap-3">
        <label class="input-label mb-0">{{ t('admin.accounts.xiaoapi.pricing') }}</label>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-1.5 text-sm" data-testid="xiao-add-pricing" @click="addPricing">
          <Icon name="plus" size="xs" />
          {{ t('admin.accounts.xiaoapi.addPricing') }}
        </button>
      </div>
      <div class="overflow-x-auto">
        <div class="divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-600 dark:border-dark-600">
          <div v-for="(rule, index) in pricing" :key="index" class="grid grid-cols-2 items-end gap-2 py-3 sm:grid-cols-[1.25fr_0.8fr_0.85fr_0.85fr_0.7fr_0.65fr_2.5rem]">
            <label class="col-span-2 min-w-0 text-xs text-gray-500 dark:text-gray-400 sm:col-span-1">
              {{ t('admin.accounts.xiaoapi.publicModel') }}
              <input v-model="rule.model" type="text" class="input mt-1" :data-testid="`xiao-price-model-${index}`" />
            </label>
            <label class="min-w-0 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.xiaoapi.resolution') }}
              <input v-model="rule.resolution" type="text" class="input mt-1" placeholder="720p" :data-testid="`xiao-price-resolution-${index}`" />
            </label>
            <label class="min-w-0 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.xiaoapi.pricePerSecond') }}
              <input v-model.number="rule.price_per_second" type="number" min="0" step="0.00000001" class="input mt-1" :data-testid="`xiao-price-base-${index}`" />
            </label>
            <label class="min-w-0 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.xiaoapi.audioPricePerSecond') }}
              <input v-model.number="rule.audio_price_per_second" type="number" min="0" step="0.00000001" class="input mt-1" :data-testid="`xiao-price-audio-${index}`" />
            </label>
            <label class="min-w-0 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.xiaoapi.defaultDuration') }}
              <input v-model.number="rule.default_duration" type="number" min="1" max="3600" step="1" class="input mt-1" :data-testid="`xiao-price-duration-${index}`" />
            </label>
            <label class="flex h-10 items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
              <input type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" :checked="rule.default_resolution" :data-testid="`xiao-price-default-${index}`" @change="setDefault(index, $event)" />
              {{ t('admin.accounts.xiaoapi.defaultResolution') }}
            </label>
            <button type="button" class="btn btn-secondary flex h-10 w-10 items-center justify-center justify-self-end p-0 text-red-600" :title="t('common.delete')" @click="pricing.splice(index, 1)">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import {
  createXiaoVideoPricingRule,
  type XiaoVideoModelMapping,
  type XiaoVideoPricingRule
} from './xiaoVideoPricing'

const pricing = defineModel<XiaoVideoPricingRule[]>('pricing', { required: true })
const mappings = defineModel<XiaoVideoModelMapping[]>('mappings', { required: true })
const { t } = useI18n()

function addMapping() {
  mappings.value.push({ from: '', to: '' })
}

function addPricing() {
  const next = createXiaoVideoPricingRule()
  next.default_resolution = pricing.value.length === 0
  pricing.value.push(next)
}

function setDefault(index: number, event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  const selected = pricing.value[index]
  if (!selected) return
  selected.default_resolution = checked
  if (!checked) return
  const model = selected.model.trim()
  pricing.value.forEach((rule, ruleIndex) => {
    if (ruleIndex !== index && rule.model.trim() === model) rule.default_resolution = false
  })
}
</script>
