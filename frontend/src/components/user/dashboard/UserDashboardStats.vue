<template>
  <!-- Row 1: Core Stats -->
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <!-- Balance -->
    <UserDashboardStatCard
      v-if="!isSimple"
      icon="creditCard"
      :title="t('dashboard.balance')"
      :value="`$${formatBalance(balance)}`"
    >
      {{ t('common.available') }}
    </UserDashboardStatCard>

    <!-- API Keys -->
    <UserDashboardStatCard
      icon="key"
      :title="t('dashboard.apiKeys')"
      :value="stats?.total_api_keys || 0"
    >
      {{ stats?.active_api_keys || 0 }} {{ t('common.active') }}
    </UserDashboardStatCard>

    <!-- Today Requests -->
    <UserDashboardStatCard
      icon="chart"
      :title="t('dashboard.todayRequests')"
      :value="stats?.today_requests || 0"
    >
      {{ t('common.total') }}: {{ formatNumber(stats?.total_requests || 0) }}
    </UserDashboardStatCard>

    <!-- Today Cost -->
    <UserDashboardStatCard
      icon="dollar"
      :title="t('dashboard.todayCost')"
      :value="`$${formatCost(stats?.today_actual_cost || 0)}`"
    >
      <template #value-extra>
        <span class="text-sm font-normal text-muted-foreground" :title="t('dashboard.standard')">
          / ${{ formatCost(stats?.today_cost || 0) }}
        </span>
      </template>
      <span :title="t('dashboard.actual')">{{ t('common.total') }}: ${{ formatCost(stats?.total_actual_cost || 0) }}</span>
      <span :title="t('dashboard.standard')"> / ${{ formatCost(stats?.total_cost || 0) }}</span>
    </UserDashboardStatCard>
  </div>

  <!-- Row 2: Token Stats -->
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <!-- Today Tokens -->
    <UserDashboardStatCard
      icon="cube"
      :title="t('dashboard.todayTokens')"
      :value="formatTokens(stats?.today_tokens || 0)"
    >
      {{ t('dashboard.input') }}: {{ formatTokens(stats?.today_input_tokens || 0) }} / {{ t('dashboard.output') }}: {{ formatTokens(stats?.today_output_tokens || 0) }}
    </UserDashboardStatCard>

    <!-- Total Tokens -->
    <UserDashboardStatCard
      icon="database"
      :title="t('dashboard.totalTokens')"
      :value="formatTokens(stats?.total_tokens || 0)"
    >
      {{ t('dashboard.input') }}: {{ formatTokens(stats?.total_input_tokens || 0) }} / {{ t('dashboard.output') }}: {{ formatTokens(stats?.total_output_tokens || 0) }}
    </UserDashboardStatCard>

    <!-- Performance (RPM/TPM) -->
    <UserDashboardStatCard
      icon="bolt"
      :title="t('dashboard.performance')"
      :value="formatTokens(stats?.rpm || 0)"
    >
      <template #value-extra>
        <span class="text-xs text-muted-foreground">RPM</span>
      </template>
      TPM: {{ formatTokens(stats?.tpm || 0) }}
    </UserDashboardStatCard>

    <!-- Avg Response Time -->
    <UserDashboardStatCard
      icon="clock"
      :title="t('dashboard.avgResponse')"
      :value="formatDuration(stats?.average_duration_ms || 0)"
    >
      {{ t('dashboard.averageTime') }}
    </UserDashboardStatCard>
  </div>

  <!-- Row 3: Contribution Stats -->
  <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
    <!-- Today Contribution Tokens -->
    <UserDashboardStatCard
      icon="upload"
      :title="t('dashboard.todayContributionTokens')"
      :value="formatTokens(stats?.today_contribution_tokens || 0)"
    >
      {{ t('dashboard.input') }}: {{ formatTokens(stats?.today_contribution_input_tokens || 0) }} /
      {{ t('dashboard.output') }}: {{ formatTokens(stats?.today_contribution_output_tokens || 0) }}
    </UserDashboardStatCard>

    <!-- Total Contribution Tokens -->
    <UserDashboardStatCard
      icon="database"
      :title="t('dashboard.totalContributionTokens')"
      :value="formatTokens(stats?.total_contribution_tokens || 0)"
    >
      {{ t('dashboard.input') }}: {{ formatTokens(stats?.total_contribution_input_tokens || 0) }} /
      {{ t('dashboard.output') }}: {{ formatTokens(stats?.total_contribution_output_tokens || 0) }}
    </UserDashboardStatCard>

    <!-- Today Contribution Balance -->
    <UserDashboardStatCard
      icon="gift"
      :title="t('dashboard.todayContributionEarned')"
      :value="formatMoney(stats?.today_contribution_earned_quota || 0)"
    >
      {{ t('common.total') }}: {{ formatMoney(stats?.total_contribution_earned_quota || 0) }}
    </UserDashboardStatCard>

    <!-- Current Contribution Balance -->
    <UserDashboardStatCard
      icon="dollar"
      :title="t('dashboard.currentContributionBalance')"
      :value="formatMoney(stats?.current_contribution_quota || 0)"
    >
      {{ t('common.available') }}
    </UserDashboardStatCard>
  </div>

  <!-- Row 4: Per-platform breakdown -->
  <div v-if="!isSimple && platformCards.length > 0" class="rounded-lg border border-border bg-card p-4">
    <div class="mb-3 flex items-center justify-between">
      <h3 class="text-sm text-muted-foreground">{{ t('dashboard.platformBreakdown') }}</h3>
      <span class="text-xs text-muted-foreground">
        {{ t('dashboard.platformCount', { count: sortedPlatforms.length }) }}
      </span>
    </div>
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div
        v-for="item in platformCards"
        :key="item.platform"
        :class="[
          'rounded-lg border border-border p-3',
          item.isOther && 'border-dashed bg-muted/50'
        ]"
      >
        <div class="flex items-center justify-between">
          <span class="text-sm font-medium text-foreground">
            {{ item.isOther ? t('dashboard.platformOther') : platformLabel(item.platform) }}
          </span>
          <span class="text-sm font-semibold tabular-nums text-foreground" :title="t('dashboard.actual')">
            ${{ formatCost(item.total_actual_cost) }}
          </span>
        </div>
        <div class="mt-2 space-y-1 text-xs">
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground">{{ t('dashboard.todayCost') }}</span>
            <span class="tabular-nums text-foreground">${{ formatCost(item.today_actual_cost) }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground">{{ t('dashboard.requests') }}</span>
            <span class="tabular-nums text-muted-foreground">
              {{ item.total_requests > 0 ? formatNumber(item.total_requests) : '-' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground">{{ t('dashboard.tokens') }}</span>
            <span class="tabular-nums text-muted-foreground">
              {{ item.total_tokens > 0 ? formatTokens(item.total_tokens) : '-' }}
            </span>
          </div>
        </div>

        <!-- Quota 区：仅当 quota 配置存在、非 __other__ 且至少有一个窗口配了 limit 时显示 -->
        <div v-if="hasAnyLimit(item.quota) && !item.isOther" class="mt-3 space-y-1.5 border-t border-border pt-2">
          <p class="text-[10px] uppercase tracking-wide text-muted-foreground">
            {{ t('dashboard.platformQuota.title') }}
          </p>
          <template v-for="w in (['daily', 'weekly', 'monthly'] as const)" :key="w">
            <div v-if="quotaVal(item.quota, `${w}_limit_usd`) != null" class="space-y-0.5">
              <!-- limit=0：完全禁用 -->
              <template v-if="(quotaVal(item.quota, `${w}_limit_usd`) as number) === 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-muted-foreground">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                  <span class="tabular-nums text-red-500">{{ t('dashboard.platformQuota.disabled') }}</span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                  <div class="h-full w-full rounded-full bg-red-500" />
                </div>
              </template>
              <!-- limit>0：正常用量进度条 -->
              <template v-else>
                <div class="flex items-center justify-between text-xs">
                  <span class="text-muted-foreground">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                  <span class="tabular-nums text-foreground">
                    ${{ formatUsd((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0) }} / ${{ formatUsd(quotaVal(item.quota, `${w}_limit_usd`) as number) }}
                  </span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="quotaBarClass(calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number))"
                    :style="{ width: calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number) + '%' }"
                  />
                </div>
                <p v-if="quotaVal(item.quota, `${w}_window_resets_at`)" class="text-[10px] text-muted-foreground">
                  {{ t('dashboard.platformQuota.resetsAt', { time: formatResetTime(quotaVal(item.quota, `${w}_window_resets_at`) as string) }) }}
                </p>
              </template>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import UserDashboardStatCard from '@/components/user/dashboard/UserDashboardStatCard.vue'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'

interface FusedPlatformCard {
  platform: string
  total_actual_cost: number
  today_actual_cost: number
  total_requests: number
  total_tokens: number
  isOther?: boolean
  quota?: PlatformQuotaItem
}

const props = defineProps<{
  stats: UserStatsType
  balance: number
  isSimple: boolean
  platformQuotas?: PlatformQuotaItem[] | null
}>()
const { t } = useI18n()

const PLATFORM_LABELS: Record<string, string> = {
  anthropic: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini',
  antigravity: 'Antigravity'
}

const platformLabel = (p: string) => PLATFORM_LABELS[p] ?? p

const sortedPlatforms = computed(() => {
  const list = props.stats?.by_platform ?? []
  return [...list].sort((a, b) => b.total_actual_cost - a.total_actual_cost)
})

// 处理"各平台之和 < 总值"的差值：后端按平台聚合时过滤了无法归属平台的行
// （group 与 account 都缺 platform）。这里把差值作为"其他"卡片显式展示，
// 避免 Row 1 总值与 Row 3 平台拆分加总对不上、用户困惑。
const OTHER_THRESHOLD = 0.0001
const platformCards = computed<FusedPlatformCard[]>(() => {
  // 建立 by_platform Map
  const byPlat = new Map<string, (typeof sortedPlatforms.value)[number]>()
  for (const item of props.stats?.by_platform ?? []) byPlat.set(item.platform, item)

  // 建立 quota Map
  const byQuota = new Map<string, PlatformQuotaItem>()
  for (const q of props.platformQuotas ?? []) byQuota.set(q.platform, q)

  // union 平台集合。后端 by_platform / quota 接口均不会返回 platform='__other__'，
  // 无需显式排除；__other__ 由下方差值补差逻辑单独追加。
  const platforms = new Set<string>([...byPlat.keys(), ...byQuota.keys()])

  const PLATFORM_ORDER = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu']
  const cards: FusedPlatformCard[] = []

  for (const p of platforms) {
    const stat = byPlat.get(p)
    cards.push({
      platform: p,
      total_actual_cost: stat?.total_actual_cost ?? 0,
      today_actual_cost: stat?.today_actual_cost ?? 0,
      total_requests: stat?.total_requests ?? 0,
      total_tokens: stat?.total_tokens ?? 0,
      quota: byQuota.get(p),
    })
  }

  // 排序：按 PLATFORM_ORDER，未知平台按名称排序
  cards.sort((a, b) => {
    const ai = PLATFORM_ORDER.indexOf(a.platform)
    const bi = PLATFORM_ORDER.indexOf(b.platform)
    if (ai === -1 && bi === -1) return a.platform.localeCompare(b.platform)
    if (ai === -1) return 1
    if (bi === -1) return -1
    return ai - bi
  })

  // __other__ 补差逻辑：只对 by_platform 有 usage 数据的总和计算
  const total = props.stats?.total_actual_cost ?? 0
  const today = props.stats?.today_actual_cost ?? 0
  const sumTotal = cards.reduce((s, c) => s + c.total_actual_cost, 0)
  const sumToday = cards.reduce((s, c) => s + c.today_actual_cost, 0)
  const diffTotal = Math.max(0, total - sumTotal)
  const diffToday = Math.max(0, today - sumToday)

  if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
    cards.push({
      platform: '__other__',
      total_actual_cost: diffTotal,
      today_actual_cost: diffToday,
      total_requests: 0,
      total_tokens: 0,
      isOther: true,
    })
  }

  return cards
})

// Quota helpers

type QuotaWindow = 'daily' | 'weekly' | 'monthly'
type QuotaField = `${QuotaWindow}_limit_usd` | `${QuotaWindow}_usage_usd` | `${QuotaWindow}_window_resets_at`

function quotaVal(q: PlatformQuotaItem | undefined, key: QuotaField): PlatformQuotaItem[QuotaField] {
  return q?.[key]
}

function hasAnyLimit(q: PlatformQuotaItem | undefined): boolean {
  if (!q) return false
  return q.daily_limit_usd != null || q.weekly_limit_usd != null || q.monthly_limit_usd != null
}

function calcPercent(usage: number, limit: number): number {
  if (!limit || limit <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((usage / limit) * 100)))
}

function quotaBarClass(p: number): string {
  if (p >= 95) return 'bg-red-500'
  if (p >= 75) return 'bg-amber-500'
  return 'bg-green-500'
}

// 与 formatBalance 一致使用 Intl.NumberFormat 做半偶舍入，避免 toFixed 在不同 JS 引擎
// 下偶发截断而非四舍五入（与后端展示精度不一致）。
const usdFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})
function formatUsd(n: number): string {
  if (!Number.isFinite(n)) return '0.00'
  return usdFormatter.format(n)
}

function formatResetTime(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

const formatBalance = (b: number) =>
  new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(b)

const formatNumber = (n: number) => n.toLocaleString()
const formatCost = (c: number) => c.toFixed(4)
const formatMoney = (value: number) => {
  if (!Number.isFinite(value)) return '$0'
  return `$${value.toFixed(value >= 10 ? 2 : 4).replace(/\.?0+$/, '')}`
}
const formatTokens = (t: number) => {
  if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`
  if (t >= 1000) return `${(t / 1000).toFixed(1)}K`
  return t.toString()
}
const formatDuration = (ms: number) => ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(0)}ms`
</script>
