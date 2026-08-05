<template>
  <div class="flex h-full flex-col rounded-lg border border-border bg-card p-4">
    <div class="flex w-full items-center justify-between">
      <p class="truncate text-[13px] text-muted-foreground">{{ title }}</p>
      <Icon v-if="icon" :name="icon" size="lg" class="shrink-0 text-muted-foreground" aria-hidden="true" />
    </div>
    <div class="mt-auto">
      <div class="flex items-baseline gap-1.5">
        <p class="truncate text-xl font-semibold text-foreground" :title="String(value)">{{ value }}</p>
        <slot name="value-extra" />
      </div>
      <p v-if="$slots.default" class="mt-0.5 truncate text-xs text-muted-foreground"><slot /></p>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

type IconName = InstanceType<typeof Icon>['$props']['name']

defineProps<{
  title: string
  value: string | number
  icon?: IconName
}>()
</script>
