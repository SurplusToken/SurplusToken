<template>
  <section
    data-testid="carpool-usage-panel"
    class="min-h-[18rem] min-w-0 space-y-5"
  >
    <div
      v-if="loading"
      data-testid="carpool-usage-loading"
      class="space-y-5"
      aria-busy="true"
      aria-live="polite"
    >
      <span class="sr-only">{{ t('userSubscriptions.carpoolUsage.loading') }}</span>
      <div v-for="index in 4" :key="index" class="animate-pulse space-y-2" aria-hidden="true">
        <div class="flex justify-between gap-4">
          <div class="h-4 w-24 rounded bg-gray-200 dark:bg-dark-600"></div>
          <div class="h-4 w-36 rounded bg-gray-200 dark:bg-dark-600"></div>
        </div>
        <div class="h-2 rounded-full bg-gray-200 dark:bg-dark-600"></div>
      </div>
    </div>

    <div
      v-else-if="error"
      class="flex min-h-[18rem] flex-col items-start justify-center gap-3"
      role="alert"
    >
      <p class="font-medium text-red-600 dark:text-red-400">
        {{ t('userSubscriptions.carpoolUsage.loadFailed') }}
      </p>
      <p class="max-w-full break-words text-sm text-gray-500 dark:text-dark-400">
        {{ error }}
      </p>
      <button
        type="button"
        class="rounded-md bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
        @click="emit('retry')"
      >
        {{ t('userSubscriptions.carpoolUsage.retry') }}
      </button>
    </div>

    <div v-else-if="snapshot" class="space-y-5">
      <div
        v-for="row in usageRows"
        :key="row.kind"
        data-testid="usage-row"
        :data-usage-kind="row.kind"
        class="min-w-0 space-y-2"
      >
        <div
          data-testid="usage-row-header"
          class="flex min-w-0 flex-wrap items-start justify-between gap-x-4 gap-y-1"
        >
          <span
            data-testid="usage-label"
            class="min-w-0 break-words text-sm font-medium [overflow-wrap:anywhere]"
            :class="row.warning ? 'text-red-700 dark:text-red-300' : 'text-gray-700 dark:text-gray-300'"
          >
            {{ row.label }}
          </span>
          <span
            data-testid="usage-amount"
            class="min-w-0 break-words text-right text-sm tabular-nums [overflow-wrap:anywhere]"
            :class="row.warning ? 'text-red-700 dark:text-red-300' : 'text-gray-500 dark:text-dark-400'"
          >
            {{ row.amount }}
          </span>
        </div>

        <div
          class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600"
          role="progressbar"
          :aria-label="row.label"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="row.percentage"
        >
          <div
            data-testid="progress-fill"
            class="absolute inset-y-0 left-0 rounded-full transition-[width] duration-300"
            :class="row.fillClass"
            :style="{ width: `${row.percentage}%` }"
          ></div>
        </div>

        <div
          v-if="row.details.length > 0"
          class="flex min-w-0 flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400"
        >
          <span v-for="detail in row.details" :key="detail" class="break-words [overflow-wrap:anywhere]">
            {{ detail }}
          </span>
        </div>
        <p
          v-if="row.sharedPoolUsage"
          class="break-words text-xs font-medium text-red-700 [overflow-wrap:anywhere] dark:text-red-300"
        >
          {{ row.sharedPoolUsage }}
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CarpoolUsageMember, CarpoolUsageSnapshot } from '@/types'

interface UsageRow {
  kind: string
  label: string
  amount: string
  percentage: number
  fillClass: string
  warning: boolean
  details: string[]
  sharedPoolUsage: string
}

const props = withDefaults(
  defineProps<{
    snapshot?: CarpoolUsageSnapshot | null
    loading?: boolean
    error?: string | null
  }>(),
  {
    snapshot: null,
    loading: false,
    error: null,
  },
)

const emit = defineEmits<{
  retry: []
}>()

const { locale, t } = useI18n()

function finite(value: number): number {
  return Number.isFinite(value) ? value : 0
}

function percentage(used: number, capacity: number): number {
  const safeCapacity = finite(capacity)
  if (safeCapacity <= 0) return 0

  const value = (finite(used) / safeCapacity) * 100
  return Math.round(Math.min(100, Math.max(0, value)) * 100) / 100
}

function formatUsd(value: number): string {
  const localeCode = locale.value === 'zh' ? 'zh-CN' : 'en-US'
  const number = new Intl.NumberFormat(localeCode, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(finite(value))
  return `$${number}`
}

function memberRow(member: CarpoolUsageMember): UsageRow {
  const zeroQuotaUsage = member.reservedQuotaUsd <= 0
    && (member.usageUsd > 0 || member.sharedPoolUsageUsd > 0)
  const warning = zeroQuotaUsage
    || (member.reservedQuotaUsd > 0 && member.usageUsd >= member.reservedQuotaUsd)
  const memberPercentage = member.reservedQuotaUsd > 0
    ? percentage(member.usageUsd, member.reservedQuotaUsd)
    : warning
      ? 100
      : 0
  const label = member.isCurrentUser
    ? t('userSubscriptions.carpoolUsage.me')
    : t('userSubscriptions.carpoolUsage.member', { number: member.memberNumber })

  return {
    kind: member.isCurrentUser ? 'current-user' : `member-${member.memberNumber}`,
    label,
    amount: `${formatUsd(member.usageUsd)} / ${formatUsd(member.reservedQuotaUsd)}`,
    percentage: memberPercentage,
    fillClass: warning
      ? 'bg-red-500'
      : member.isCurrentUser
        ? 'bg-emerald-500'
        : 'bg-gray-400 dark:bg-gray-500',
    warning,
    details: [
      t('userSubscriptions.carpoolUsage.declaredQuota', {
        amount: formatUsd(member.declaredQuotaUsd),
      }),
      t('userSubscriptions.carpoolUsage.memberQuota', {
        amount: formatUsd(member.reservedQuotaUsd),
      }),
    ],
    sharedPoolUsage:
      member.sharedPoolUsageUsd > 0
        ? t('userSubscriptions.carpoolUsage.sharedPoolUsed', {
            amount: formatUsd(member.sharedPoolUsageUsd),
          })
        : '',
  }
}

const usageRows = computed<UsageRow[]>(() => {
  if (!props.snapshot) return []

  const { sharedPool } = props.snapshot
  const members = [...props.snapshot.members].sort((left, right) => {
    if (left.isCurrentUser !== right.isCurrentUser) return left.isCurrentUser ? -1 : 1
    return left.memberNumber - right.memberNumber
  })

  return [
    {
      kind: 'total',
      label: t('userSubscriptions.carpoolUsage.totalUsage'),
      amount: `${formatUsd(props.snapshot.totalUsageUsd)} / ${formatUsd(props.snapshot.totalCapacityUsd)}`,
      percentage: percentage(props.snapshot.totalUsageUsd, props.snapshot.totalCapacityUsd),
      fillClass: 'bg-amber-500',
      warning: false,
      details: [],
      sharedPoolUsage: '',
    },
    {
      kind: 'shared-pool',
      label: t('userSubscriptions.carpoolUsage.sharedPool'),
      amount: `${formatUsd(sharedPool.usageUsd)} / ${formatUsd(sharedPool.capacityUsd)}`,
      percentage: percentage(sharedPool.usageUsd, sharedPool.capacityUsd),
      fillClass: 'bg-sky-500',
      warning: false,
      details: [
        t('userSubscriptions.carpoolUsage.remaining', {
          amount: formatUsd(Math.max(0, sharedPool.remainingUsd)),
        }),
      ],
      sharedPoolUsage: '',
    },
    ...members.map(memberRow),
  ]
})
</script>
