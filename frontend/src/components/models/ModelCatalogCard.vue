<template>
  <article
    class="flex min-h-[300px] flex-col rounded-lg border bg-white transition-shadow hover:shadow-md dark:bg-dark-800"
    :class="platformBorderClass(model.platform)"
  >
    <div class="flex items-start gap-3 border-b border-gray-100 p-4 dark:border-dark-700">
      <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-gray-50 dark:bg-dark-700">
        <ModelIcon :model="model.name" size="24px" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-start gap-2">
          <h2 class="min-w-0 flex-1 break-words text-sm font-semibold text-gray-900 dark:text-white">
            {{ model.name }}
          </h2>
          <button
            type="button"
            class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:hover:bg-dark-600 dark:hover:text-gray-200"
            :title="t('modelSquare.copyModelId')"
            @click="emit('copy', model.name)"
          >
            <Icon name="copy" size="sm" />
          </button>
        </div>
        <span
          class="mt-1 inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-medium"
          :class="platformBadgeClass(model.platform)"
        >
          {{ platformLabel(model.platform) }}
        </span>
      </div>
    </div>

    <div class="flex flex-1 flex-col p-4">
      <div v-if="priceItems.length > 0" class="grid grid-cols-2 gap-x-4 gap-y-3">
        <div v-for="item in priceItems" :key="item.label" class="min-w-0">
          <div class="truncate text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
          <div class="mt-1 flex min-w-0 items-baseline gap-1">
            <span class="truncate font-mono text-base font-semibold text-gray-900 dark:text-white">
              {{ item.value }}
            </span>
            <span class="flex-shrink-0 text-[10px] text-gray-400 dark:text-gray-500">
              {{ item.unit }}
            </span>
          </div>
        </div>
      </div>
      <div v-else class="flex min-h-[58px] items-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('modelSquare.pricingUnavailable') }}
      </div>

      <div
        v-if="accountOverrideCount > 0"
        class="mt-3 inline-flex w-fit items-center gap-1 rounded-md bg-amber-50 px-2 py-1 text-[11px] font-medium text-amber-700 dark:bg-amber-900/25 dark:text-amber-300"
      >
        <Icon name="server" size="xs" />
        {{ t('modelSquare.accountRoutePricingCount', { count: accountOverrideCount }) }}
      </div>

      <div class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700">
        <div v-if="model.performance" class="grid grid-cols-3 gap-2 text-center">
          <div class="min-w-0">
            <div class="text-[10px] text-gray-400 dark:text-gray-500">
              {{ t('modelSquare.successRate24h') }}
            </div>
            <div
              class="mt-0.5 truncate font-mono text-sm font-semibold"
              :class="successRateClass(model.performance.success_rate)"
            >
              {{ formatSuccessRate(model.performance.success_rate) }}
            </div>
          </div>
          <div class="min-w-0 border-x border-gray-100 px-1 dark:border-dark-700">
            <div class="text-[10px] text-gray-400 dark:text-gray-500">
              {{ t('modelSquare.averageLatency') }}
            </div>
            <div class="mt-0.5 truncate font-mono text-sm font-semibold text-gray-800 dark:text-gray-200">
              {{ formatLatency(model.performance.avg_latency_ms) }}
            </div>
          </div>
          <div class="min-w-0">
            <div class="text-[10px] text-gray-400 dark:text-gray-500">
              {{ t('modelSquare.samples') }}
            </div>
            <div class="mt-0.5 truncate font-mono text-sm font-semibold text-gray-800 dark:text-gray-200">
              {{ model.performance.sample_count.toLocaleString() }}
            </div>
          </div>
        </div>
        <div v-else class="text-center text-xs text-gray-400 dark:text-gray-500">
          {{ t('modelSquare.noPerformanceData') }}
        </div>
        <div
          v-if="model.performance && model.performance.sample_count < 5"
          class="mt-1 text-center text-[10px] text-amber-600 dark:text-amber-400"
        >
          {{ t('modelSquare.insufficientSamples') }}
        </div>
      </div>

      <div class="mt-auto pt-5">
        <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span class="inline-flex items-center gap-1">
            <Icon name="server" size="xs" />
            {{ t('modelSquare.routeCount', { count: model.routes.length }) }}
          </span>
          <span class="text-gray-300 dark:text-dark-500">·</span>
          <span class="inline-flex items-center gap-1">
            <Icon name="users" size="xs" />
            {{ t('modelSquare.groupCount', { count: model.groups.length }) }}
          </span>
        </div>
        <div class="mt-3 flex items-center justify-between gap-3 border-t border-gray-100 pt-3 dark:border-dark-700">
          <span class="truncate text-xs text-gray-500 dark:text-gray-400">
            {{ model.bestRoute?.channelName || t('modelSquare.pricingUnavailable') }}
          </span>
          <button
            type="button"
            class="inline-flex flex-shrink-0 items-center gap-1 text-xs font-medium text-primary-600 hover:text-primary-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:text-primary-400 dark:hover:text-primary-300"
            @click="emit('details', model)"
          >
            {{ t('modelSquare.details') }}
            <Icon name="chevronRight" size="xs" />
          </button>
        </div>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import type { ModelCatalogItem } from '@/features/model-square/modelCatalog'
import { minimumPricingValue } from '@/features/model-square/modelCatalog'
import { formatScaled } from '@/utils/pricing'
import { platformBadgeClass, platformBorderClass, platformLabel } from '@/utils/platformColors'

const props = defineProps<{ model: ModelCatalogItem }>()
const emit = defineEmits<{
  (event: 'copy', modelName: string): void
  (event: 'details', model: ModelCatalogItem): void
}>()

const { t } = useI18n()

const accountOverrideCount = computed(() =>
  props.model.routes.reduce((count, route) => count + route.accountPricingOverrides.length, 0)
)

function formatSuccessRate(value: number): string {
  return `${value.toFixed(value >= 99.95 ? 0 : 1)}%`
}

function formatLatency(value: number | null): string {
  if (value === null || !Number.isFinite(value) || value < 0) return '-'
  if (value >= 1000) return `${(value / 1000).toFixed(value >= 10_000 ? 0 : 1)}s`
  return `${Math.round(value)}ms`
}

function successRateClass(value: number): string {
  if (value >= 99) return 'text-emerald-600 dark:text-emerald-400'
  if (value >= 95) return 'text-amber-600 dark:text-amber-400'
  return 'text-red-600 dark:text-red-400'
}

const priceItems = computed(() => {
  const pricing = props.model.bestRoute?.pricing
  if (!pricing) return []

  const item = (label: string, value: number | null, scale: number, unit: string) => ({
    label: pricing.intervals?.length > 0 ? t('modelSquare.startingAt', { label }) : label,
    value: formatScaled(value, scale),
    unit
  })

  if (pricing.billing_mode === 'per_request') {
    const value = minimumPricingValue(pricing, 'per_request_price')
    return value === null ? [] : [item(t('modelSquare.perRequest'), value, 1, t('modelSquare.perRequestUnit'))]
  }

  if (pricing.billing_mode === 'image') {
    const values = [
      item(t('modelSquare.imageInput'), minimumPricingValue(pricing, 'image_input_price'), 1, t('modelSquare.perImageUnit')),
      item(t('modelSquare.imageOutput'), minimumPricingValue(pricing, 'image_output_price'), 1, t('modelSquare.perImageUnit'))
    ]
    return values.filter((entry) => entry.value !== '-')
  }

  return [
    item(t('modelSquare.input'), minimumPricingValue(pricing, 'input_price'), 1_000_000, t('modelSquare.perMillionUnit')),
    item(t('modelSquare.output'), minimumPricingValue(pricing, 'output_price'), 1_000_000, t('modelSquare.perMillionUnit'))
  ].filter((entry) => entry.value !== '-')
})
</script>
