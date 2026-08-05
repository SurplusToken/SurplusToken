<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <div class="flex h-full flex-col rounded-lg border border-border bg-card p-4">
      <div class="flex w-full items-center justify-between">
        <p class="truncate text-[13px] text-muted-foreground">{{ t('usage.totalRequests') }}</p>
        <Icon name="document" class="h-6 w-6 shrink-0 text-muted-foreground" />
      </div>
      <p class="mt-2 text-xl font-semibold text-foreground">{{ stats?.total_requests?.toLocaleString() || '0' }}</p>
      <p class="mt-1 text-xs text-muted-foreground">{{ t('usage.inSelectedRange') }}</p>
    </div>
    <div class="flex h-full flex-col rounded-lg border border-border bg-card p-4">
      <div class="flex w-full items-center justify-between">
        <p class="truncate text-[13px] text-muted-foreground">{{ t('usage.totalTokens') }}</p>
        <Icon name="cube" class="h-6 w-6 shrink-0 text-muted-foreground" />
      </div>
      <p class="mt-2 text-xl font-semibold text-foreground">{{ formatTokens(stats?.total_tokens || 0) }}</p>
      <p class="mt-1 flex flex-wrap items-center gap-x-1 text-xs text-muted-foreground">
        <span>{{ t('usage.in') }}: {{ formatTokens(stats?.total_input_tokens || 0) }}</span>
        <span>/</span>
        <span>{{ t('usage.out') }}: {{ formatTokens(stats?.total_output_tokens || 0) }}</span>
        <span>/</span>
        <span class="group relative inline-flex cursor-help items-center gap-0.5" tabindex="0">
          <span>{{ cacheLabel() }}: {{ formatTokens(stats?.total_cache_tokens || 0) }}</span>
          <Icon name="infoCircle" size="sm" class="h-3.5 w-3.5 text-muted-foreground" />
          <span
            class="pointer-events-none absolute left-1/2 top-full z-30 mt-2 w-56 -translate-x-1/2 rounded-lg border border-border bg-card p-3 text-left text-xs text-muted-foreground opacity-0 shadow-lg transition-opacity duration-150 group-hover:opacity-100 group-focus:opacity-100"
          >
            <span class="mb-2 block font-medium text-foreground">
              {{ cacheDetailLabel() }}
            </span>
            <span class="flex items-center justify-between gap-3">
              <span>{{ t('usage.cacheCreationTokensLabel') }}</span>
              <span class="tabular-nums text-foreground">
                {{ formatTokens(stats?.total_cache_creation_tokens || 0) }}
              </span>
            </span>
            <span class="mt-1 flex items-center justify-between gap-3">
              <span>{{ t('usage.cacheReadTokensLabel') }}</span>
              <span class="tabular-nums text-foreground">
                {{ formatTokens(stats?.total_cache_read_tokens || 0) }}
              </span>
            </span>
          </span>
        </span>
      </p>
    </div>
    <div class="flex h-full flex-col rounded-lg border border-border bg-card p-4">
      <div class="flex w-full items-center justify-between">
        <p class="truncate text-[13px] text-muted-foreground">{{ t('usage.totalCost') }}</p>
        <Icon name="dollar" class="h-6 w-6 shrink-0 text-muted-foreground" />
      </div>
      <p class="mt-2 text-xl font-semibold text-foreground">
        ${{ (stats?.total_actual_cost || 0).toFixed(4) }}
      </p>
      <p class="mt-1 text-xs text-muted-foreground">
        <template v-if="showAccountCost && totalAccountCost != null">
          <span>{{ t('usage.accountCost') }} ${{ totalAccountCost.toFixed(4) }}</span>
          <span> · </span>
        </template>
        <span>
          {{ t('usage.standardCost') }}
          <span :class="{ 'line-through': strikeStandardCost }">${{ (stats?.total_cost || 0).toFixed(4) }}</span>
        </span>
      </p>
    </div>
    <div class="flex h-full flex-col rounded-lg border border-border bg-card p-4">
      <div class="flex w-full items-center justify-between">
        <p class="truncate text-[13px] text-muted-foreground">{{ t('usage.avgDuration') }}</p>
        <Icon name="clock" class="h-6 w-6 shrink-0 text-muted-foreground" />
      </div>
      <p class="mt-2 text-xl font-semibold text-foreground">{{ formatDuration(stats?.average_duration_ms || 0) }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import type { UsageStatsResponse } from '@/types'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  stats: (AdminUsageStatsResponse | UsageStatsResponse) | null
  showAccountCost?: boolean
  strikeStandardCost?: boolean
}>(), {
  showAccountCost: true,
  strikeStandardCost: false,
})

const { t } = useI18n()

const totalAccountCost = computed(() => {
  const stats = props.stats as (AdminUsageStatsResponse & { total_account_cost?: number }) | null
  return stats?.total_account_cost ?? null
})
const showAccountCost = computed(() => props.showAccountCost)
const strikeStandardCost = computed(() => props.strikeStandardCost)

const formatDuration = (ms: number) =>
  ms < 1000 ? `${ms.toFixed(0)}ms` : `${(ms / 1000).toFixed(2)}s`

const formatTokens = (value: number) => {
  if (value >= 1e9) return (value / 1e9).toFixed(2) + 'B'
  if (value >= 1e6) return (value / 1e6).toFixed(2) + 'M'
  if (value >= 1e3) return (value / 1e3).toFixed(2) + 'K'
  return value.toLocaleString()
}

const cacheLabel = () => t('usage.cacheTotal')
const cacheDetailLabel = () => t('usage.cacheBreakdown')
</script>
