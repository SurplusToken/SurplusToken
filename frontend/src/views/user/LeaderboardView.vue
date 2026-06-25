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

      <!-- Personal token usage trend (privacy-safe: current user only) -->
      <section>
        <h2 class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('leaderboard.trendTitle') }}
        </h2>
        <TokenUsageTrend :trend-data="trendData" :loading="trendLoading" />
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
          <!-- Top-3 -->
          <section v-if="topThree.length">
            <h2 class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('leaderboard.podium') }}
            </h2>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div
                v-for="entry in topThree"
                :key="entry.user_id"
                class="relative flex flex-col gap-1 rounded-lg border bg-white p-4 shadow-sm dark:bg-dark-800"
                :class="
                  isMe(entry)
                    ? 'border-blue-500 dark:border-blue-500'
                    : 'border-gray-200 dark:border-dark-700'
                "
              >
                <div class="flex items-center justify-between">
                  <span class="text-2xl leading-none">{{ medal(entry.rank) }}</span>
                  <span class="font-mono text-xs text-gray-400 dark:text-gray-500">#{{ entry.rank }}</span>
                </div>
                <div
                  class="mt-1 truncate text-base font-semibold text-gray-900 dark:text-white"
                  :title="displayName(entry.display_name)"
                >
                  {{ displayName(entry.display_name) }}
                </div>
                <div class="mt-2 flex items-baseline justify-between">
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('leaderboard.columns.tokens') }}</span>
                  <span class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ formatTokens(entry.total_tokens) }}</span>
                </div>
                <div class="flex items-baseline justify-between">
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('leaderboard.columns.credits') }}</span>
                  <span class="font-mono text-sm text-gray-600 dark:text-gray-300">{{ formatCredits(entry.total_cost) }}</span>
                </div>
                <span
                  v-if="isMe(entry)"
                  class="absolute right-3 top-3 inline-flex items-center rounded bg-blue-50 px-1.5 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-900/20 dark:text-blue-300"
                >
                  {{ t('leaderboard.yourRank') }}
                </span>
              </div>
            </div>
          </section>

          <!-- Ranked list (rank 4..) -->
          <section
            v-if="restEntries.length"
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
                v-for="entry in restEntries"
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
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { useAuthStore } from '@/stores/auth'
import { usageAPI } from '@/api/usage'
import {
  leaderboardAPI,
  type LeaderboardEntry,
  type LeaderboardMe,
  type LeaderboardPeriod,
} from '@/api/leaderboard'
import type { TrendDataPoint } from '@/types'
import { formatCompactNumber, formatCostFixed } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()

const periods: LeaderboardPeriod[] = ['today', 'week', 'month']
const period = ref<LeaderboardPeriod>('today')

const loading = ref(false)
const error = ref(false)
const entries = ref<LeaderboardEntry[]>([])
const me = ref<LeaderboardMe | null>(null)

// Personal token usage trend (current user only)
const trendLoading = ref(false)
const trendData = ref<TrendDataPoint[]>([])

const topThree = computed(() => entries.value.filter((e) => e.rank <= 3))
const restEntries = computed(() => entries.value.filter((e) => e.rank > 3))

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

// Credits (cost): same convention the dashboard uses for consumed/actual cost (4 decimals).
function formatCredits(cost: number): string {
  return formatCostFixed(cost, 4)
}

function medal(rank: number): string {
  return rank === 1 ? '🥇' : rank === 2 ? '🥈' : rank === 3 ? '🥉' : `#${rank}`
}

function formatLocalDate(d: Date): string {
  return d.toISOString().split('T')[0]
}

async function load() {
  loading.value = true
  error.value = false
  try {
    const data = await leaderboardAPI.getUsageLeaderboard(period.value)
    entries.value = data.entries ?? []
    me.value = data.me ?? null
  } catch (err) {
    console.error('Failed to load usage leaderboard:', err)
    error.value = true
  } finally {
    loading.value = false
  }
}

async function loadTrend() {
  trendLoading.value = true
  try {
    const res = await usageAPI.getDashboardTrend({
      start_date: formatLocalDate(new Date(Date.now() - 29 * 86400000)),
      end_date: formatLocalDate(new Date()),
      granularity: 'day',
    })
    trendData.value = res.trend ?? []
  } catch (err) {
    console.error('Failed to load token usage trend:', err)
    trendData.value = []
  } finally {
    trendLoading.value = false
  }
}

function setPeriod(p: LeaderboardPeriod) {
  if (period.value === p) return
  period.value = p
  load()
}

onMounted(() => {
  load()
  loadTrend()
})
</script>
