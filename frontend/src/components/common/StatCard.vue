<template>
  <div class="flex h-full flex-col rounded-lg border border-border bg-card">
    <div class="flex flex-grow flex-col p-4">
      <div class="flex w-full items-center justify-between">
        <p class="truncate text-[13px] text-muted-foreground">{{ title }}</p>
        <component
          v-if="icon"
          :is="icon"
          class="h-6 w-6 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
      </div>
      <div class="mt-auto flex items-baseline gap-2">
        <p class="truncate text-xl font-semibold text-foreground" :title="String(formattedValue)">
          {{ formattedValue }}
        </p>
        <span v-if="change !== undefined" :class="['flex items-center gap-1 text-xs font-medium', trendClass]">
          <Icon
            v-if="changeType !== 'neutral'"
            name="arrowUp"
            size="xs"
            :class="changeType === 'down' && 'rotate-180'"
          />
          {{ formattedChange }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import Icon from '@/components/icons/Icon.vue'

type ChangeType = 'up' | 'down' | 'neutral'
type IconVariant = 'primary' | 'success' | 'warning' | 'danger'

interface Props {
  title: string
  value: number | string
  icon?: Component
  iconVariant?: IconVariant
  change?: number
  changeType?: ChangeType
  formatValue?: (value: number | string) => string
}

const props = withDefaults(defineProps<Props>(), {
  changeType: 'neutral',
  iconVariant: 'primary'
})

const formattedValue = computed(() => {
  if (props.formatValue) {
    return props.formatValue(props.value)
  }
  if (typeof props.value === 'number') {
    return props.value.toLocaleString()
  }
  return props.value
})

const formattedChange = computed(() => {
  if (props.change === undefined) return ''
  const absChange = Math.abs(props.change)
  return `${absChange}%`
})

const trendClass = computed(() => {
  const classes: Record<ChangeType, string> = {
    up: 'text-emerald-600 dark:text-emerald-400',
    down: 'text-red-600 dark:text-red-400',
    neutral: 'text-gray-500 dark:text-dark-400'
  }
  return classes[props.changeType]
})
</script>
