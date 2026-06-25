<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('leaderboard.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('leaderboard.description') }}</p>
        </div>

        <!-- Period segmented control -->
        <div class="flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
          <button
            v-for="p in periods"
            :key="p"
            type="button"
            class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors"
            :class="
              period === p
                ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-800 dark:text-white'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
            "
            @click="setPeriod(p)"
          >
            {{ t(`leaderboard.periods.${p}`) }}
          </button>
        </div>
      </div>

      <!-- Per-user token usage trend (one line per user; display_name masked by backend) -->
      <section
        class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800"
      >
        <h2 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('leaderboard.trendTitle') }}
        </h2>
        <div class="h-64">
          <div v-if="trendLoading" class="flex h-full items-center justify-center">
            <LoadingSpinner />
          </div>
          <Line v-else-if="trendChartData" :data="trendChartData" :options="lineOptions" />
          <div
            v-else
            class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
          >
            {{ t('leaderboard.empty') }}
          </div>
        </div>
      </section>

      <!-- Loading skeleton -->
      <div
        v-if="loading"
        class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="flex items-center justify-center py-8">
          <LoadingSpinner />
        </div>
        <div class="space-y-2">
          <div
            v-for="n in 6"
            :key="n"
            class="h-10 animate-pulse rounded-md bg-gray-100 dark:bg-dark-700"
          />
        </div>
      </div>

      <!-- Error state -->
      <div
        v-else-if="error"
        class="flex flex-col items-center justify-center gap-4 rounded-lg border border-gray-200 bg-white py-16 text-center shadow-sm dark:border-dark-700 dark:bg-dark-800"
      >
        <p class="text-sm text-red-600 dark:text-red-400">{{ t('leaderboard.loadError') }}</p>
        <button type="button" class="btn btn-secondary" @click="load">
          {{ t('leaderboard.retry') }}
        </button>
      </div>

      <!-- Content -->
      <template v-else>
        <!-- Empty state -->
        <div
          v-if="entries.length === 0"
          class="flex items-center justify-center rounded-lg border border-gray-200 bg-white py-16 text-center shadow-sm dark:border-dark-700 dark:bg-dark-800"
        >
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('leaderboard.empty') }}</p>
        </div>

        <template v-else>
          <!-- Ranked list (all entries, starting from #1) -->
          <section
            class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800"
          >
            <div
              class="grid grid-cols-[3rem_1fr_6rem_6rem] gap-2 border-b border-gray-200 px-4 py-2.5 text-xs font-medium text-gray-500 dark:border-dark-700 dark:text-gray-400 sm:grid-cols-[4rem_1fr_8rem_8rem]"
            >
              <span>{{ t('leaderboard.columns.rank') }}</span>
              <span>{{ t('leaderboard.columns.user') }}</span>
              <span class="text-right">{{ t('leaderboard.columns.tokens') }}</span>
              <span class="text-right">{{ t('leaderboard.columns.credits') }}</span>
            </div>
            <ul class="divide-y divide-gray-100 dark:divide-dark-700">
              <li
                v-for="entry in entries"
                :key="entry.user_id"
                class="grid grid-cols-[3rem_1fr_6rem_6rem] items-center gap-2 px-4 py-3 text-sm sm:grid-cols-[4rem_1fr_8rem_8rem]"
                :class="
                  isMe(entry)
                    ? 'border-l-2 border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                    : ''
                "
              >
                <span class="font-mono text-gray-500 dark:text-gray-400">#{{ entry.rank }}</span>
                <span class="flex min-w-0 items-center gap-2">
                  <span class="truncate text-gray-900 dark:text-white" :title="displayName(entry.display_name)">
                    {{ displayName(entry.display_name) }}
                  </span>
                  <span
                    v-if="isMe(entry)"
                    class="inline-flex shrink-0 items-center rounded bg-blue-50 px-1.5 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-900/20 dark:text-blue-300"
                  >
                    {{ t('leaderboard.yourRank') }}
                  </span>
                </span>
                <span class="text-right font-mono font-medium text-gray-900 dark:text-white">{{ formatTokens(entry.total_tokens) }}</span>
                <span class="text-right font-mono text-gray-600 dark:text-gray-300">{{ formatCredits(entry.total_cost) }}</span>
              </li>
            </ul>
          </section>

          <!-- "Your rank" card -->
          <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="flex flex-wrap items-center gap-x-6 gap-y-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('leaderboard.yourRank') }}</span>
              <template v-if="me">
                <span class="font-mono text-xl font-bold text-gray-900 dark:text-white">#{{ me.rank }}</span>
                <div class="flex flex-col">
                  <span class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ formatTokens(me.total_tokens) }}</span>
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('leaderboard.columns.tokens') }}</span>
                </div>
                <div class="flex flex-col">
                  <span class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ formatCredits(me.total_cost) }}</span>
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('leaderboard.columns.credits') }}</span>
                </div>
              </template>
              <span v-else class="text-sm text-gray-500 dark:text-gray-400">{{ t('leaderboard.noUsage') }}</span>
            </div>
          </div>
        </template>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { Line } from 'vue-chartjs'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAuthStore } from '@/stores/auth'
import {
  leaderboardAPI,
  type LeaderboardEntry,
  type LeaderboardMe,
  type LeaderboardPeriod,
  type LeaderboardTrendPoint,
} from '@/api/leaderboard'
import { formatCompactNumber, formatCostFixed } from '@/utils/format'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()
const authStore = useAuthStore()

const periods: LeaderboardPeriod[] = ['today', 'week', 'month']
const period = ref<LeaderboardPeriod>('today')

const loading = ref(false)
const error = ref(false)
const entries = ref<LeaderboardEntry[]>([])
const me = ref<LeaderboardMe | null>(null)

// Per-user token usage trend (one line per user)
const trendLoading = ref(false)
const userTrend = ref<LeaderboardTrendPoint[]>([])

const currentUserId = computed(() => authStore.user?.id)
function isMe(entry: LeaderboardEntry): boolean {
  return currentUserId.value != null && entry.user_id === currentUserId.value
}

// Render "—" when the backend hands back an empty / placeholder display name.
function displayName(name: string): string {
  const raw = (name ?? '').trim()
  if (!raw) return '—'
  const lower = raw.toLowerCase()
  if (lower === 'null' || lower === 'undefined') return '—'
  return raw
}

// Tokens: big-number formatting (e.g. 1.2M / 123.4K) — reuses the shared util.
function formatTokens(tokens: number): string {
  return formatCompactNumber(tokens)
}

// Credits (cost): "$" prefix, 4 decimals (e.g. $33.4738).
function formatCredits(cost: number): string {
  return `$${formatCostFixed(cost, 4)}`
}

// Compact token formatting for chart axis/tooltip (mirrors admin dashboard).
function formatChartTokens(value: number | undefined): string {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}

// ---- Multi-user trend chart (mirrors admin DashboardView) ----
const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
}))

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const,
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: { size: 11 },
      },
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => `${context.dataset.label}: ${formatChartTokens(context.raw)}`,
      },
    },
  },
  scales: {
    x: {
      grid: { color: chartColors.value.grid },
      ticks: { color: chartColors.value.text, font: { size: 10 } },
    },
    y: {
      grid: { color: chartColors.value.grid },
      ticks: {
        color: chartColors.value.text,
        font: { size: 10 },
        callback: (value: string | number) => formatChartTokens(Number(value)),
      },
    },
  },
}))

const trendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  // Group by user_id to avoid merging different users with the same display name.
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    if (!userGroups.has(point.user_id)) {
      userGroups.set(point.user_id, { name: displayName(point.display_name), data: new Map() })
    }
    userGroups.get(point.user_id)!.data.set(point.date, point.tokens)
  })

  const sortedDates = Array.from(allDates).sort()
  const colors = [
    '#3b82f6',
    '#10b981',
    '#f59e0b',
    '#ef4444',
    '#8b5cf6',
    '#ec4899',
    '#14b8a6',
    '#f97316',
    '#6366f1',
    '#84cc16',
    '#06b6d4',
    '#a855f7',
  ]

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: colors[idx % colors.length],
    backgroundColor: `${colors[idx % colors.length]}20`,
    fill: false,
    tension: 0.3,
  }))

  return { labels: sortedDates, datasets }
})

async function load() {
  loading.value = true
  error.value = false
  try {
    const data = await leaderboardAPI.getUsageLeaderboard(period.value)
    entries.value = data.entries ?? []
    me.value = data.me ?? null
  } catch (err) {
    console.error('Failed to load usage statistics:', err)
    error.value = true
  } finally {
    loading.value = false
  }
}

async function loadTrend() {
  trendLoading.value = true
  try {
    const res = await leaderboardAPI.getUsageLeaderboardTrend(period.value)
    userTrend.value = res.users_trend ?? []
  } catch (err) {
    console.error('Failed to load usage trend:', err)
    userTrend.value = []
  } finally {
    trendLoading.value = false
  }
}

function reload() {
  load()
  loadTrend()
}

function setPeriod(p: LeaderboardPeriod) {
  if (period.value === p) return
  period.value = p
  reload()
}

onMounted(reload)
</script>
