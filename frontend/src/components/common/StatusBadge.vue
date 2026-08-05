<template>
  <span
    :class="[
      'inline-flex h-6 items-center rounded-lg px-2 text-xs font-semibold ring-1 ring-inset',
      variantClass
    ]"
  >
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
  label: string
}>()

// Helicone 状态徽章公式:bg-{c}-50 dark:bg-{c}-900 text-{c}-700 dark:text-{c}-300 ring-{c}-600/20
const variantClass = computed(() => {
  switch (props.status) {
    case 'active':
    case 'success':
      return 'bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-900 dark:text-green-300'
    case 'error':
    case 'danger':
      return 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900 dark:text-red-300'
    case 'warning':
    case 'threat':
      return 'bg-yellow-50 text-yellow-700 ring-yellow-600/20 dark:bg-yellow-900 dark:text-yellow-300'
    case 'cached':
    case 'cancelled':
      return 'bg-orange-50 text-orange-700 ring-orange-600/20 dark:bg-orange-900 dark:text-orange-300'
    case 'disabled':
    case 'inactive':
    case 'pending':
    case 'unknown':
    default:
      return 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900 dark:text-gray-300'
  }
})
</script>
