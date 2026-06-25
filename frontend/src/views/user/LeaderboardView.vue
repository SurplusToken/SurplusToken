<template>
  <AppLayout>
    <div class="lb-root space-y-6">
      <!-- Header -->
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="lb-title text-2xl font-bold">{{ t('leaderboard.title') }}</h1>
          <p class="mt-1 text-sm text-gray-400">{{ t('leaderboard.description') }}</p>
        </div>

        <!-- Period segmented control -->
        <div class="lb-segment inline-flex rounded-xl p-1">
          <button
            v-for="p in periods"
            :key="p"
            type="button"
            class="lb-segment-btn"
            :class="{ 'lb-segment-btn-active': period === p }"
            @click="setPeriod(p)"
          >
            {{ t(`leaderboard.periods.${p}`) }}
          </button>
        </div>
      </div>

      <!-- Loading skeleton -->
      <div v-if="loading" class="space-y-6">
        <div class="flex items-center justify-center py-16">
          <LoadingSpinner />
        </div>
        <div class="space-y-2">
          <div v-for="n in 6" :key="n" class="lb-skeleton-row" />
        </div>
      </div>

      <!-- Error state -->
      <div v-else-if="error" class="lb-panel flex flex-col items-center justify-center gap-4 py-16 text-center">
        <p class="text-sm text-red-300">{{ t('leaderboard.loadError') }}</p>
        <button type="button" class="lb-retry-btn" @click="load">{{ t('leaderboard.retry') }}</button>
      </div>

      <!-- Content -->
      <template v-else>
        <!-- Empty state -->
        <div v-if="entries.length === 0" class="lb-panel flex items-center justify-center py-16 text-center">
          <p class="text-sm text-gray-400">{{ t('leaderboard.empty') }}</p>
        </div>

        <template v-else>
          <!-- Top-3 podium -->
          <section v-if="topThree.length" aria-label="podium">
            <h2 class="lb-section-label mb-3">{{ t('leaderboard.podium') }}</h2>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div
                v-for="entry in topThree"
                :key="entry.user_id"
                class="lb-podium-card"
                :class="[`lb-podium-${entry.rank}`, { 'lb-podium-me': isMe(entry) }]"
              >
                <div class="lb-podium-medal">{{ medal(entry.rank) }}</div>
                <div class="lb-podium-name" :title="entry.display_name">{{ entry.display_name }}</div>
                <div class="lb-podium-tokens">{{ formatTokens(entry.total_tokens) }}</div>
                <div class="lb-podium-tokens-label">{{ t('leaderboard.columns.tokens') }}</div>
                <div class="lb-podium-credits">{{ formatCredits(entry.total_cost) }}</div>
                <div class="lb-podium-credits-label">{{ t('leaderboard.columns.credits') }}</div>
                <span v-if="isMe(entry)" class="lb-me-tag">{{ t('leaderboard.yourRank') }}</span>
              </div>
            </div>
          </section>

          <!-- Ranked list (rank 4..) -->
          <section v-if="restEntries.length" class="lb-panel overflow-hidden">
            <div class="lb-list-head">
              <span class="lb-col-rank">{{ t('leaderboard.columns.rank') }}</span>
              <span class="lb-col-user">{{ t('leaderboard.columns.user') }}</span>
              <span class="lb-col-num">{{ t('leaderboard.columns.tokens') }}</span>
              <span class="lb-col-num">{{ t('leaderboard.columns.credits') }}</span>
            </div>
            <ul>
              <li
                v-for="entry in restEntries"
                :key="entry.user_id"
                class="lb-list-row"
                :class="{ 'lb-list-row-me': isMe(entry) }"
              >
                <span class="lb-col-rank lb-rank-badge">{{ entry.rank }}</span>
                <span class="lb-col-user truncate" :title="entry.display_name">
                  {{ entry.display_name }}
                  <span v-if="isMe(entry)" class="lb-me-tag-inline">{{ t('leaderboard.yourRank') }}</span>
                </span>
                <span class="lb-col-num lb-num-tokens">{{ formatTokens(entry.total_tokens) }}</span>
                <span class="lb-col-num lb-num-credits">{{ formatCredits(entry.total_cost) }}</span>
              </li>
            </ul>
          </section>
        </template>

        <!-- Sticky "Your rank" card -->
        <div class="lb-sticky">
          <div class="lb-yourrank-card">
            <span class="lb-yourrank-label">{{ t('leaderboard.yourRank') }}</span>
            <template v-if="me">
              <span class="lb-yourrank-rank">#{{ me.rank }}</span>
              <span class="lb-yourrank-sep" />
              <span class="lb-yourrank-metric">
                <span class="lb-yourrank-value">{{ formatTokens(me.total_tokens) }}</span>
                <span class="lb-yourrank-unit">{{ t('leaderboard.columns.tokens') }}</span>
              </span>
              <span class="lb-yourrank-metric">
                <span class="lb-yourrank-value">{{ formatCredits(me.total_cost) }}</span>
                <span class="lb-yourrank-unit">{{ t('leaderboard.columns.credits') }}</span>
              </span>
            </template>
            <span v-else class="lb-yourrank-empty">{{ t('leaderboard.noUsage') }}</span>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAuthStore } from '@/stores/auth'
import {
  leaderboardAPI,
  type LeaderboardEntry,
  type LeaderboardMe,
  type LeaderboardPeriod,
} from '@/api/leaderboard'
import { formatCompactNumber, formatCostFixed } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()

const periods: LeaderboardPeriod[] = ['today', 'week', 'month']
const period = ref<LeaderboardPeriod>('today')

const loading = ref(false)
const error = ref(false)
const entries = ref<LeaderboardEntry[]>([])
const me = ref<LeaderboardMe | null>(null)

const topThree = computed(() => entries.value.filter((e) => e.rank <= 3))
const restEntries = computed(() => entries.value.filter((e) => e.rank > 3))

const currentUserId = computed(() => authStore.user?.id)
function isMe(entry: LeaderboardEntry): boolean {
  return currentUserId.value != null && entry.user_id === currentUserId.value
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

function setPeriod(p: LeaderboardPeriod) {
  if (period.value === p) return
  period.value = p
  load()
}

onMounted(load)
</script>

<style scoped>
.lb-root {
  position: relative;
  border-radius: 1rem;
  padding: 1.5rem;
  background:
    radial-gradient(circle at 12% 0%, rgba(34, 211, 238, 0.12), transparent 42%),
    radial-gradient(circle at 88% 8%, rgba(217, 70, 239, 0.12), transparent 45%),
    linear-gradient(160deg, #0b1020 0%, #0d1326 60%, #0a0f1f 100%);
  overflow: hidden;
}

/* Subtle grid / scanline vibe */
.lb-root::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(56, 189, 248, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(56, 189, 248, 0.05) 1px, transparent 1px);
  background-size: 32px 32px;
  pointer-events: none;
  z-index: 0;
}

.lb-root > * {
  position: relative;
  z-index: 1;
}

.lb-title {
  color: #e2e8f0;
  text-shadow: 0 0 18px rgba(34, 211, 238, 0.45);
}

.lb-section-label {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #67e8f9;
}

/* Segmented control */
.lb-segment {
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(56, 189, 248, 0.2);
}

.lb-segment-btn {
  padding: 0.45rem 1rem;
  border-radius: 0.6rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #94a3b8;
  transition: all 0.18s ease;
}

.lb-segment-btn:hover {
  color: #e2e8f0;
}

.lb-segment-btn-active {
  color: #0b1020;
  background: linear-gradient(135deg, #22d3ee, #d946ef);
  box-shadow: 0 0 16px rgba(34, 211, 238, 0.5);
}

/* Panels */
.lb-panel {
  border-radius: 0.9rem;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(56, 189, 248, 0.15);
}

/* Skeleton */
.lb-skeleton-row {
  height: 3rem;
  border-radius: 0.75rem;
  background: linear-gradient(90deg, rgba(30, 41, 59, 0.4) 25%, rgba(51, 65, 85, 0.6) 37%, rgba(30, 41, 59, 0.4) 63%);
  background-size: 400% 100%;
  animation: lb-shimmer 1.4s ease infinite;
}

@keyframes lb-shimmer {
  0% { background-position: 100% 0; }
  100% { background-position: 0 0; }
}

/* Podium */
.lb-podium-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
  padding: 1.5rem 1rem 1.25rem;
  border-radius: 1rem;
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(148, 163, 184, 0.2);
  transition: transform 0.2s ease;
}

.lb-podium-card:hover {
  transform: translateY(-3px);
}

.lb-podium-1 {
  border-color: rgba(250, 204, 21, 0.55);
  box-shadow: 0 0 28px rgba(250, 204, 21, 0.28);
  order: 0;
}

.lb-podium-2 {
  border-color: rgba(203, 213, 225, 0.5);
  box-shadow: 0 0 22px rgba(203, 213, 225, 0.2);
}

.lb-podium-3 {
  border-color: rgba(217, 119, 6, 0.5);
  box-shadow: 0 0 22px rgba(217, 119, 6, 0.22);
}

.lb-podium-me {
  outline: 2px solid #22d3ee;
  outline-offset: 2px;
}

.lb-podium-medal {
  font-size: 2.25rem;
  line-height: 1;
}

.lb-podium-name {
  margin-top: 0.35rem;
  max-width: 100%;
  font-size: 1rem;
  font-weight: 700;
  color: #f1f5f9;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lb-podium-tokens {
  margin-top: 0.5rem;
  font-size: 1.5rem;
  font-weight: 800;
  color: #67e8f9;
  text-shadow: 0 0 14px rgba(34, 211, 238, 0.45);
}

.lb-podium-tokens-label,
.lb-podium-credits-label {
  font-size: 0.7rem;
  letter-spacing: 0.05em;
  color: #64748b;
  text-transform: uppercase;
}

.lb-podium-credits {
  margin-top: 0.4rem;
  font-size: 1rem;
  font-weight: 600;
  color: #f0abfc;
}

/* List */
.lb-list-head {
  display: grid;
  grid-template-columns: 4rem 1fr 7rem 7rem;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #64748b;
  border-bottom: 1px solid rgba(56, 189, 248, 0.12);
}

.lb-list-row {
  display: grid;
  grid-template-columns: 4rem 1fr 7rem 7rem;
  gap: 0.5rem;
  align-items: center;
  padding: 0.75rem 1rem;
  font-size: 0.9rem;
  color: #cbd5e1;
  border-bottom: 1px solid rgba(148, 163, 184, 0.08);
  transition: background 0.15s ease;
}

.lb-list-row:hover {
  background: rgba(56, 189, 248, 0.06);
}

.lb-list-row-me {
  background: linear-gradient(90deg, rgba(34, 211, 238, 0.14), rgba(217, 70, 239, 0.1));
  box-shadow: inset 3px 0 0 #22d3ee;
}

.lb-col-num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.lb-num-tokens {
  color: #67e8f9;
  font-weight: 600;
}

.lb-num-credits {
  color: #f0abfc;
}

.lb-rank-badge {
  font-weight: 700;
  color: #94a3b8;
}

.lb-me-tag {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  padding: 0.1rem 0.45rem;
  font-size: 0.65rem;
  font-weight: 600;
  border-radius: 0.4rem;
  color: #0b1020;
  background: #22d3ee;
}

.lb-me-tag-inline {
  margin-left: 0.4rem;
  padding: 0.05rem 0.4rem;
  font-size: 0.65rem;
  font-weight: 600;
  border-radius: 0.35rem;
  color: #0b1020;
  background: #22d3ee;
}

/* Retry */
.lb-retry-btn {
  padding: 0.5rem 1.25rem;
  border-radius: 0.6rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: #0b1020;
  background: linear-gradient(135deg, #22d3ee, #d946ef);
  box-shadow: 0 0 16px rgba(34, 211, 238, 0.4);
}

/* Sticky "your rank" */
.lb-sticky {
  position: sticky;
  bottom: 1rem;
  z-index: 2;
}

.lb-yourrank-card {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 1rem;
  padding: 0.85rem 1.25rem;
  border-radius: 0.9rem;
  background: rgba(11, 16, 32, 0.92);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(34, 211, 238, 0.4);
  box-shadow: 0 0 24px rgba(34, 211, 238, 0.25);
}

.lb-yourrank-label {
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: #67e8f9;
  text-transform: uppercase;
}

.lb-yourrank-rank {
  font-size: 1.5rem;
  font-weight: 800;
  color: #f1f5f9;
  text-shadow: 0 0 14px rgba(34, 211, 238, 0.5);
}

.lb-yourrank-sep {
  width: 1px;
  height: 1.5rem;
  background: rgba(148, 163, 184, 0.3);
}

.lb-yourrank-metric {
  display: flex;
  flex-direction: column;
  line-height: 1.15;
}

.lb-yourrank-value {
  font-size: 1rem;
  font-weight: 700;
  color: #e2e8f0;
  font-variant-numeric: tabular-nums;
}

.lb-yourrank-unit {
  font-size: 0.65rem;
  letter-spacing: 0.05em;
  color: #64748b;
  text-transform: uppercase;
}

.lb-yourrank-empty {
  font-size: 0.9rem;
  color: #94a3b8;
}
</style>
