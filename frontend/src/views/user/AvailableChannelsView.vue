<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1600px] space-y-5">
      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-center">
          <div class="relative min-w-0 flex-1">
            <Icon
              name="search"
              size="md"
              class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
            />
            <input
              v-model="searchQuery"
              type="search"
              :placeholder="t('modelSquare.searchPlaceholder')"
              class="input w-full pl-10 pr-10"
            />
            <button
              v-if="searchQuery"
              type="button"
              class="absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-600 dark:hover:text-gray-200"
              :title="t('modelSquare.clearSearch')"
              @click="searchQuery = ''"
            >
              <Icon name="x" size="sm" />
            </button>
          </div>

          <div class="flex items-start gap-3 xl:flex-shrink-0">
            <div class="grid min-w-0 flex-1 grid-cols-1 gap-3 sm:grid-cols-2 xl:flex">
              <label class="relative">
                <span class="sr-only">{{ t('modelSquare.billingMode') }}</span>
                <select v-model="selectedBillingMode" class="input w-full min-w-0 pr-9 xl:min-w-44">
                  <option value="all">{{ t('modelSquare.allBillingModes') }}</option>
                  <option value="token">{{ t('modelSquare.billingToken') }}</option>
                  <option value="per_request">{{ t('modelSquare.billingPerRequest') }}</option>
                  <option value="image">{{ t('modelSquare.billingImage') }}</option>
                  <option value="unpriced">{{ t('modelSquare.billingUnpriced') }}</option>
                </select>
              </label>
              <label class="relative">
                <span class="sr-only">{{ t('modelSquare.sort') }}</span>
                <select v-model="sortBy" class="input w-full min-w-0 pr-9 xl:min-w-44">
                  <option value="name">{{ t('modelSquare.sortName') }}</option>
                  <option value="routes">{{ t('modelSquare.sortRoutes') }}</option>
                  <option value="price">{{ t('modelSquare.sortPrice') }}</option>
                  <option value="success">{{ t('modelSquare.sortSuccessRate') }}</option>
                </select>
              </label>
            </div>

            <button
              type="button"
              class="btn btn-secondary h-10 w-10 flex-shrink-0 p-0"
              :disabled="loading"
              :title="t('modelSquare.refresh')"
              @click="loadChannels"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>

        <div class="mt-4 flex gap-2 overflow-x-auto pb-1">
          <button
            type="button"
            class="flex-shrink-0 rounded-md border px-3 py-1.5 text-xs font-medium transition-colors"
            :class="selectedPlatform === 'all' ? activeFilterClass : inactiveFilterClass"
            @click="selectedPlatform = 'all'"
          >
            {{ t('modelSquare.allPlatforms') }}
            <span class="ml-1 opacity-70">{{ catalog.length }}</span>
          </button>
          <button
            v-for="platform in platformOptions"
            :key="platform.name"
            type="button"
            class="flex-shrink-0 rounded-md border px-3 py-1.5 text-xs font-medium transition-colors"
            :class="selectedPlatform === platform.name ? activeFilterClass : inactiveFilterClass"
            @click="selectedPlatform = platform.name"
          >
            {{ platformLabel(platform.name) }}
            <span class="ml-1 opacity-70">{{ platform.count }}</span>
          </button>
        </div>
      </section>

      <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
        <p class="text-gray-600 dark:text-gray-400">
          {{ t('modelSquare.resultCount', { count: filteredModels.length }) }}
          <span class="mx-1 text-gray-300 dark:text-dark-600">·</span>
          {{ t('modelSquare.totalRoutes', { count: filteredRouteCount }) }}
        </p>
        <button
          v-if="hasActiveFilters"
          type="button"
          class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
          @click="clearFilters"
        >
          {{ t('modelSquare.clearFilters') }}
        </button>
      </div>

      <div
        v-if="loading && catalog.length === 0"
        class="grid grid-cols-[repeat(auto-fill,minmax(min(100%,280px),1fr))] gap-4"
        :aria-label="t('modelSquare.loading')"
      >
        <div
          v-for="index in 8"
          :key="index"
          class="h-[300px] animate-pulse rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="flex gap-3 border-b border-gray-100 p-4 dark:border-dark-700">
            <div class="h-10 w-10 rounded-lg bg-gray-200 dark:bg-dark-600"></div>
            <div class="flex-1 space-y-2 pt-1">
              <div class="h-4 w-3/4 rounded bg-gray-200 dark:bg-dark-600"></div>
              <div class="h-3 w-20 rounded bg-gray-100 dark:bg-dark-700"></div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-else-if="pagedModels.length > 0"
        class="grid grid-cols-[repeat(auto-fill,minmax(min(100%,280px),1fr))] gap-4"
      >
        <ModelCatalogCard
          v-for="model in pagedModels"
          :key="model.key"
          :model="model"
          @copy="copyModelName"
          @details="selectedModel = $event"
        />
      </div>

      <div
        v-else
        class="flex min-h-64 flex-col items-center justify-center rounded-lg border border-dashed border-gray-300 px-6 py-12 text-center dark:border-dark-600"
      >
        <Icon name="inbox" size="xl" class="text-gray-300 dark:text-dark-500" />
        <h2 class="mt-4 text-sm font-semibold text-gray-800 dark:text-gray-200">
          {{ hasActiveFilters ? t('modelSquare.noResults') : t('modelSquare.noModels') }}
        </h2>
        <p class="mt-1 max-w-md text-xs text-gray-500 dark:text-gray-400">
          {{ hasActiveFilters ? t('modelSquare.noResultsHint') : t('modelSquare.noModelsHint') }}
        </p>
        <button
          v-if="hasActiveFilters"
          type="button"
          class="btn btn-secondary mt-4"
          @click="clearFilters"
        >
          {{ t('modelSquare.clearFilters') }}
        </button>
      </div>

      <nav
        v-if="pageCount > 1"
        class="flex items-center justify-center gap-3"
        :aria-label="t('modelSquare.pagination')"
      >
        <button
          type="button"
          class="btn btn-secondary h-9 w-9 p-0"
          :disabled="currentPage === 1"
          :title="t('modelSquare.previousPage')"
          @click="currentPage--"
        >
          <Icon name="chevronLeft" size="sm" />
        </button>
        <span class="min-w-20 text-center text-xs text-gray-600 dark:text-gray-400">
          {{ t('modelSquare.pageStatus', { current: currentPage, total: pageCount }) }}
        </span>
        <button
          type="button"
          class="btn btn-secondary h-9 w-9 p-0"
          :disabled="currentPage === pageCount"
          :title="t('modelSquare.nextPage')"
          @click="currentPage++"
        >
          <Icon name="chevronRight" size="sm" />
        </button>
      </nav>
    </div>

    <BaseDialog
      :show="selectedModel !== null"
      :title="selectedModel?.name || t('modelSquare.details')"
      width="wide"
      @close="selectedModel = null"
    >
      <div v-if="selectedModel" class="space-y-5">
        <div class="flex flex-wrap items-center gap-2">
          <span
            class="inline-flex items-center rounded-md border px-2 py-1 text-xs font-medium"
            :class="platformBadgeClass(selectedModel.platform)"
          >
            {{ platformLabel(selectedModel.platform) }}
          </span>
          <button
            type="button"
            class="inline-flex min-w-0 items-center gap-1.5 rounded-md bg-gray-100 px-2 py-1 font-mono text-xs text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
            :title="t('modelSquare.copyModelId')"
            @click="copyModelName(selectedModel.name)"
          >
            <span class="truncate">{{ selectedModel.name }}</span>
            <Icon name="copy" size="xs" class="flex-shrink-0" />
          </button>
        </div>

        <div v-if="selectedModel.performance" class="grid grid-cols-2 border-y border-gray-200 py-4 sm:grid-cols-4 dark:border-dark-700">
          <div class="px-3 first:pl-0">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('modelSquare.successRate24h') }}</div>
            <div
              class="mt-1 font-mono text-lg font-semibold"
              :class="successRateClass(selectedModel.performance.success_rate)"
            >
              {{ formatSuccessRate(selectedModel.performance.success_rate) }}
            </div>
          </div>
          <div class="border-l border-gray-100 px-3 dark:border-dark-700">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('modelSquare.averageLatency') }}</div>
            <div class="mt-1 font-mono text-lg font-semibold text-gray-900 dark:text-gray-100">
              {{ formatLatency(selectedModel.performance.avg_latency_ms) }}
            </div>
          </div>
          <div class="mt-3 border-l-0 px-3 sm:mt-0 sm:border-l dark:border-dark-700">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('modelSquare.averageTTFT') }}</div>
            <div class="mt-1 font-mono text-lg font-semibold text-gray-900 dark:text-gray-100">
              {{ formatLatency(selectedModel.performance.avg_ttft_ms) }}
            </div>
          </div>
          <div class="mt-3 border-l border-gray-100 px-3 sm:mt-0 dark:border-dark-700">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('modelSquare.realRequests') }}</div>
            <div class="mt-1 font-mono text-lg font-semibold text-gray-900 dark:text-gray-100">
              {{ selectedModel.performance.sample_count.toLocaleString() }}
            </div>
            <div v-if="selectedModel.performance.sample_count < 5" class="text-[10px] text-amber-600 dark:text-amber-400">
              {{ t('modelSquare.insufficientSamples') }}
            </div>
          </div>
        </div>
        <p v-else class="border-y border-gray-200 py-4 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
          {{ t('modelSquare.noPerformanceDataHint') }}
        </p>

        <div>
          <h3 class="text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">
            {{ t('modelSquare.availableRoutes') }}
          </h3>
          <div class="mt-2 divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-700 dark:border-dark-700">
            <section v-for="route in selectedModel.routes" :key="route.key" class="py-5 first:pt-4 last:pb-4">
              <div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ route.channelName }}</h4>
                    <span
                      v-if="route.key === selectedModel.bestRoute?.key && selectedModel.bestRoute?.pricing"
                      class="rounded-md bg-emerald-50 px-2 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
                    >
                      {{ t('modelSquare.lowestStartingPrice') }}
                    </span>
                  </div>
                  <p v-if="route.channelDescription" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ route.channelDescription }}
                  </p>
                </div>
                <span class="flex-shrink-0 text-xs text-gray-500 dark:text-gray-400">
                  {{ billingModeLabel(route.pricing) }}
                </span>
              </div>

              <div v-if="pricingRows(route.pricing).length > 0" class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div v-for="row in pricingRows(route.pricing)" :key="row.label">
                  <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ row.label }}</div>
                  <div class="mt-0.5 font-mono text-sm font-medium text-gray-900 dark:text-gray-100">
                    {{ row.value }}
                    <span class="text-[10px] font-normal text-gray-400">{{ row.unit }}</span>
                  </div>
                </div>
              </div>
              <p v-else class="mt-3 text-xs text-gray-500 dark:text-gray-400">
                {{ t('modelSquare.pricingUnavailable') }}
              </p>

              <div v-if="route.pricing?.intervals?.length" class="mt-4 overflow-x-auto">
                <table class="w-full min-w-[520px] text-left text-xs">
                  <thead class="text-gray-500 dark:text-gray-400">
                    <tr>
                      <th class="pb-2 font-medium">{{ t('modelSquare.tokenRange') }}</th>
                      <th class="pb-2 font-medium">{{ t('modelSquare.input') }}</th>
                      <th class="pb-2 font-medium">{{ t('modelSquare.output') }}</th>
                      <th class="pb-2 font-medium">{{ t('modelSquare.cacheRead') }}</th>
                      <th class="pb-2 font-medium">{{ t('modelSquare.cacheWrite') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 font-mono text-gray-700 dark:divide-dark-700 dark:text-gray-300">
                    <tr v-for="(interval, index) in route.pricing.intervals" :key="index">
                      <td class="py-2 pr-4 font-sans">{{ intervalLabel(interval) }}</td>
                      <td class="py-2 pr-4">{{ formatScaled(interval.input_price, 1_000_000) }}</td>
                      <td class="py-2 pr-4">{{ formatScaled(interval.output_price, 1_000_000) }}</td>
                      <td class="py-2 pr-4">{{ formatScaled(interval.cache_read_price, 1_000_000) }}</td>
                      <td class="py-2">{{ formatScaled(interval.cache_write_price, 1_000_000) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div
                v-for="override in route.accountPricingOverrides"
                :key="`${override.group_id}:${override.account_name}`"
                class="mt-4 border-l-2 border-amber-400 pl-3"
              >
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div class="text-xs font-semibold text-gray-800 dark:text-gray-200">
                    {{ t('modelSquare.accountRoutePricing') }} · {{ override.account_name }}
                  </div>
                  <span class="text-[11px] text-gray-500 dark:text-gray-400">
                    {{ groupName(route, override.group_id) }}
                  </span>
                </div>
                <p class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
                  {{ t('modelSquare.accountRoutePricingHint') }}
                </p>
                <div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
                  <div v-for="row in pricingRows(override.pricing)" :key="row.label">
                    <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ row.label }}</div>
                    <div class="mt-0.5 font-mono text-sm font-medium text-gray-900 dark:text-gray-100">
                      {{ row.value }} <span class="text-[10px] font-normal text-gray-400">{{ row.unit }}</span>
                    </div>
                  </div>
                </div>
                <div v-if="override.pricing.intervals?.length" class="mt-3 overflow-x-auto">
                  <table class="w-full min-w-[520px] text-left text-xs">
                    <thead class="text-gray-500 dark:text-gray-400">
                      <tr>
                        <th class="pb-2 font-medium">{{ t('modelSquare.tokenRange') }}</th>
                        <th class="pb-2 font-medium">{{ t('modelSquare.input') }}</th>
                        <th class="pb-2 font-medium">{{ t('modelSquare.output') }}</th>
                        <th class="pb-2 font-medium">{{ t('modelSquare.cacheRead') }}</th>
                        <th class="pb-2 font-medium">{{ t('modelSquare.cacheWrite') }}</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-100 font-mono text-gray-700 dark:divide-dark-700 dark:text-gray-300">
                      <tr v-for="(interval, index) in override.pricing.intervals" :key="index">
                        <td class="py-2 pr-4 font-sans">{{ intervalLabel(interval) }}</td>
                        <td class="py-2 pr-4">{{ formatScaled(interval.input_price, 1_000_000) }}</td>
                        <td class="py-2 pr-4">{{ formatScaled(interval.output_price, 1_000_000) }}</td>
                        <td class="py-2 pr-4">{{ formatScaled(interval.cache_read_price, 1_000_000) }}</td>
                        <td class="py-2">{{ formatScaled(interval.cache_write_price, 1_000_000) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <div v-if="route.performance?.groups.length" class="mt-4 overflow-x-auto">
                <table class="w-full min-w-[560px] text-left text-xs">
                  <thead class="text-gray-500 dark:text-gray-400">
                    <tr>
                      <th class="pb-2 font-medium">{{ t('modelSquare.group') }}</th>
                      <th class="pb-2 font-medium">{{ t('modelSquare.successRate') }}</th>
                      <th class="pb-2 font-medium">{{ t('modelSquare.averageLatency') }}</th>
                      <th class="pb-2 font-medium">{{ t('modelSquare.averageTTFT') }}</th>
                      <th class="pb-2 text-right font-medium">{{ t('modelSquare.samples') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 text-gray-700 dark:divide-dark-700 dark:text-gray-300">
                    <tr v-for="performance in route.performance.groups" :key="performance.group_id">
                      <td class="py-2 pr-4 font-medium">{{ groupName(route, performance.group_id) }}</td>
                      <td class="py-2 pr-4 font-mono" :class="successRateClass(performance.success_rate)">
                        {{ formatSuccessRate(performance.success_rate) }}
                      </td>
                      <td class="py-2 pr-4 font-mono">{{ formatLatency(performance.avg_latency_ms) }}</td>
                      <td class="py-2 pr-4 font-mono">{{ formatLatency(performance.avg_ttft_ms) }}</td>
                      <td class="py-2 text-right font-mono">
                        {{ performance.sample_count.toLocaleString() }}
                        <span v-if="performance.sample_count < 5" class="ml-1 font-sans text-[10px] text-amber-600 dark:text-amber-400">
                          {{ t('modelSquare.insufficientSamples') }}
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div class="mt-4">
                <div class="mb-2 text-[11px] text-gray-500 dark:text-gray-400">
                  {{ t('modelSquare.accessibleGroups') }}
                </div>
                <div class="flex flex-wrap gap-2">
                  <GroupBadge
                    v-for="group in route.groups"
                    :key="group.id"
                    :name="group.name"
                    :platform="asGroupPlatform(group.platform)"
                    :subscription-type="asSubscriptionType(group.subscription_type)"
                    :rate-multiplier="group.rate_multiplier"
                    :user-rate-multiplier="userGroupRates[group.id]"
                    :peak-rate-enabled="group.peak_rate_enabled"
                    :peak-start="group.peak_start"
                    :peak-end="group.peak_end"
                    :peak-rate-multiplier="group.peak_rate_multiplier"
                    always-show-rate
                  />
                  <span v-if="route.groups.length === 0" class="text-xs text-gray-400">-</span>
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="selectedModel = null">
          {{ t('modelSquare.close') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelCatalogCard from '@/components/models/ModelCatalogCard.vue'
import userChannelsAPI, {
  type UserAvailableChannel,
  type UserPricingInterval,
  type UserSupportedModelPricing
} from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import {
  buildModelCatalog,
  modelStartingPrice,
  type ModelCatalogItem
} from '@/features/model-square/modelCatalog'
import { formatScaled } from '@/utils/pricing'
import { platformBadgeClass, platformLabel } from '@/utils/platformColors'
import { extractApiErrorMessage } from '@/utils/apiError'

type BillingFilter = 'all' | 'token' | 'per_request' | 'image' | 'unpriced'
type SortOption = 'name' | 'routes' | 'price' | 'success'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const searchQuery = ref('')
const selectedPlatform = ref('all')
const selectedBillingMode = ref<BillingFilter>('all')
const sortBy = ref<SortOption>('name')
const selectedModel = ref<ModelCatalogItem | null>(null)
const currentPage = ref(1)
const pageSize = 24

const activeFilterClass =
  'border-gray-900 bg-gray-900 text-white dark:border-gray-100 dark:bg-gray-100 dark:text-gray-900'
const inactiveFilterClass =
  'border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'

const catalog = computed(() => buildModelCatalog(channels.value))

const platformOptions = computed(() => {
  const counts = new Map<string, number>()
  for (const model of catalog.value) counts.set(model.platform, (counts.get(model.platform) ?? 0) + 1)
  return [...counts.entries()]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => platformLabel(a.name).localeCompare(platformLabel(b.name)))
})

const filteredModels = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase()
  const result = catalog.value.filter((model) => {
    if (selectedPlatform.value !== 'all' && model.platform !== selectedPlatform.value) return false
    if (query && !model.searchableText.includes(query)) return false
    if (selectedBillingMode.value === 'unpriced') {
      return model.routes.every((route) => route.pricing === null)
    }
    if (selectedBillingMode.value !== 'all') {
      return model.routes.some((route) => route.pricing?.billing_mode === selectedBillingMode.value)
    }
    return true
  })

  return [...result].sort((a, b) => {
    if (sortBy.value === 'routes') {
      return b.routes.length - a.routes.length || a.name.localeCompare(b.name)
    }
    if (sortBy.value === 'price') {
      const aPrice = modelStartingPrice(a) ?? Number.POSITIVE_INFINITY
      const bPrice = modelStartingPrice(b) ?? Number.POSITIVE_INFINITY
      return aPrice - bPrice || a.name.localeCompare(b.name)
    }
    if (sortBy.value === 'success') {
      const aRate = a.performance?.success_rate ?? -1
      const bRate = b.performance?.success_rate ?? -1
      return bRate - aRate || a.name.localeCompare(b.name)
    }
    return a.name.localeCompare(b.name) || a.platform.localeCompare(b.platform)
  })
})

const filteredRouteCount = computed(() =>
  filteredModels.value.reduce((total, model) => total + model.routes.length, 0)
)
const pageCount = computed(() => Math.max(1, Math.ceil(filteredModels.value.length / pageSize)))
const pagedModels = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredModels.value.slice(start, start + pageSize)
})
const hasActiveFilters = computed(
  () =>
    searchQuery.value.trim() !== '' ||
    selectedPlatform.value !== 'all' ||
    selectedBillingMode.value !== 'all'
)

watch([searchQuery, selectedPlatform, selectedBillingMode, sortBy], () => {
  currentPage.value = 1
})

watch(pageCount, (count) => {
  if (currentPage.value > count) currentPage.value = count
})

function clearFilters() {
  searchQuery.value = ''
  selectedPlatform.value = 'all'
  selectedBillingMode.value = 'all'
}

function copyModelName(modelName: string) {
  void copyToClipboard(modelName, t('modelSquare.copied'))
}

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

function groupName(route: ModelCatalogItem['routes'][number], groupID: number): string {
  return route.groups.find((group) => group.id === groupID)?.name ?? `#${groupID}`
}

function billingModeLabel(pricing: UserSupportedModelPricing | null): string {
  if (!pricing) return t('modelSquare.pricingUnavailable')
  if (pricing.billing_mode === 'per_request') return t('modelSquare.billingPerRequest')
  if (pricing.billing_mode === 'image') return t('modelSquare.billingImage')
  return t('modelSquare.billingToken')
}

function pricingRows(pricing: UserSupportedModelPricing | null) {
  if (!pricing) return []
  const row = (label: string, value: number | null, scale: number, unit: string) => ({
    label,
    value: formatScaled(value, scale),
    unit
  })

  if (pricing.billing_mode === 'per_request') {
    return [row(t('modelSquare.perRequest'), pricing.per_request_price, 1, t('modelSquare.perRequestUnit'))]
      .filter((entry) => entry.value !== '-')
  }
  if (pricing.billing_mode === 'image') {
    return [
      row(t('modelSquare.imageInput'), pricing.image_input_price, 1, t('modelSquare.perImageUnit')),
      row(t('modelSquare.imageOutput'), pricing.image_output_price, 1, t('modelSquare.perImageUnit')),
      row(t('modelSquare.perRequest'), pricing.per_request_price, 1, t('modelSquare.perRequestUnit'))
    ].filter((entry) => entry.value !== '-')
  }
  return [
    row(t('modelSquare.input'), pricing.input_price, 1_000_000, t('modelSquare.perMillionUnit')),
    row(t('modelSquare.output'), pricing.output_price, 1_000_000, t('modelSquare.perMillionUnit')),
    row(t('modelSquare.cacheRead'), pricing.cache_read_price, 1_000_000, t('modelSquare.perMillionUnit')),
    row(t('modelSquare.cacheWrite'), pricing.cache_write_price, 1_000_000, t('modelSquare.perMillionUnit'))
  ].filter((entry) => entry.value !== '-')
}

function intervalLabel(interval: UserPricingInterval): string {
  if (interval.tier_label) return interval.tier_label
  return `${interval.min_tokens.toLocaleString()} - ${
    interval.max_tokens === null ? t('modelSquare.noLimit') : interval.max_tokens.toLocaleString()
  }`
}

function asGroupPlatform(platform: string): GroupPlatform {
  return platform as GroupPlatform
}

function asSubscriptionType(subscriptionType: string): SubscriptionType {
  return subscriptionType as SubscriptionType
}

async function loadChannels() {
  loading.value = true
  try {
    const [list, rates] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch((error: unknown) => {
        console.error('Failed to load user group rates:', error)
        return {} as Record<number, number>
      })
    ])
    channels.value = list
    userGroupRates.value = rates
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadChannels)
</script>
