<template>
  <span
    class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset"
    :class="statusClass"
  >
    {{ statusLabel }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OrderStatus } from '@/types/payment'

const props = defineProps<{
  status: OrderStatus
}>()

const { t } = useI18n()

const statusMap: Record<OrderStatus, { key: string; class: string }> = {
  PENDING: { key: 'payment.status.pending', class: 'bg-yellow-50 text-yellow-700 ring-yellow-600/20 dark:bg-yellow-900 dark:text-yellow-300' },
  PAID: { key: 'payment.status.paid', class: 'bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-900 dark:text-blue-300' },
  RECHARGING: { key: 'payment.status.recharging', class: 'bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-900 dark:text-blue-300' },
  COMPLETED: { key: 'payment.status.completed', class: 'bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-900 dark:text-green-300' },
  EXPIRED: { key: 'payment.status.expired', class: 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900 dark:text-gray-300' },
  CANCELLED: { key: 'payment.status.cancelled', class: 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900 dark:text-gray-300' },
  FAILED: { key: 'payment.status.failed', class: 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900 dark:text-red-300' },
  REFUND_REQUESTED: { key: 'payment.status.refund_requested', class: 'bg-orange-50 text-orange-700 ring-orange-600/20 dark:bg-orange-900 dark:text-orange-300' },
  REFUNDING: { key: 'payment.status.refunding', class: 'bg-orange-50 text-orange-700 ring-orange-600/20 dark:bg-orange-900 dark:text-orange-300' },
  REFUND_PENDING: { key: 'payment.status.refund_pending', class: 'bg-orange-50 text-orange-700 ring-orange-600/20 dark:bg-orange-900 dark:text-orange-300' },
  REFUNDED: { key: 'payment.status.refunded', class: 'bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-900 dark:text-purple-300' },
  PARTIALLY_REFUNDED: { key: 'payment.status.partially_refunded', class: 'bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-900 dark:text-purple-300' },
  REFUND_FAILED: { key: 'payment.status.refund_failed', class: 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900 dark:text-red-300' },
}

const statusLabel = computed(() => {
  const entry = statusMap[props.status]
  return entry ? t(entry.key) : props.status
})

const statusClass = computed(() => {
  const entry = statusMap[props.status]
  return entry?.class ?? 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900 dark:text-gray-300'
})
</script>
