<template>
  <Teleport to="body">
    <div v-if="show && account && position">
      <div class="fixed inset-0 z-[9998]" @click="emit('close')"></div>
      <div
        class="fixed z-[9999] max-h-[calc(100vh-1rem)] w-52 overflow-y-auto rounded-xl bg-white shadow-lg ring-1 ring-black/5 dark:bg-dark-800"
        :style="{ top: position.top + 'px', left: position.left + 'px' }"
        @click.stop
      >
        <div class="py-1">
          <template v-if="account.is_mine">
            <button
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
              @click="emit('edit', account); emit('close')"
            >
              <Icon name="edit" size="sm" class="text-primary-500" />
              {{ t('accountPool.actions.editSettings') }}
            </button>
            <button
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
              @click="emit('toggle-scheduling', account, !account.schedulable); emit('close')"
            >
              <Icon :name="account.schedulable ? 'ban' : 'play'" size="sm" :class="account.schedulable ? 'text-amber-500' : 'text-green-500'" />
              {{ account.schedulable ? t('accountPool.actions.pauseSharing') : t('accountPool.actions.resumeSharing') }}
            </button>
            <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
            <button
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
              @click="emit('test', account); emit('close')"
            >
              <Icon name="play" size="sm" class="text-green-500" />
              {{ t('admin.accounts.testConnection') }}
            </button>
            <button
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
              @click="emit('stats', account); emit('close')"
            >
              <Icon name="chart" size="sm" class="text-indigo-500" />
              {{ t('admin.accounts.viewStats') }}
            </button>
            <button
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
              @click="emit('schedule', account); emit('close')"
            >
              <Icon name="clock" size="sm" class="text-orange-500" />
              {{ t('admin.scheduledTests.schedule') }}
            </button>
            <button
              v-if="supportsReAuth"
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-blue-600 hover:bg-gray-100 dark:text-blue-300 dark:hover:bg-dark-700"
              @click="emit('reauth', account); emit('close')"
            >
              <Icon name="link" size="sm" />
              {{ t('admin.accounts.reAuthorize') }}
            </button>
            <button
              v-if="supportsRefresh"
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-purple-600 hover:bg-gray-100 dark:text-purple-300 dark:hover:bg-dark-700"
              @click="emit('refresh-token', account); emit('close')"
            >
              <Icon name="refresh" size="sm" />
              {{ t('admin.accounts.refreshToken') }}
            </button>
            <button
              v-if="supportsPrivacy"
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-emerald-600 hover:bg-gray-100 dark:text-emerald-300 dark:hover:bg-dark-700"
              @click="emit('set-privacy', account); emit('close')"
            >
              <Icon name="shield" size="sm" />
              {{ t('admin.accounts.setPrivacy') }}
            </button>
            <button
              v-if="hasRecoverableState"
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-emerald-600 hover:bg-gray-100 dark:text-emerald-300 dark:hover:bg-dark-700"
              @click="emit('recover-state', account); emit('close')"
            >
              <Icon name="sync" size="sm" />
              {{ t('admin.accounts.recoverState') }}
            </button>
            <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
            <button
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
              @click="emit('delete', account); emit('close')"
            >
              <Icon name="trash" size="sm" />
              {{ t('common.delete') }}
            </button>
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserAccountPoolItem } from '@/types'

const props = defineProps<{
  show: boolean
  account: UserAccountPoolItem | null
  position: { top: number; left: number } | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'test', account: UserAccountPoolItem): void
  (e: 'stats', account: UserAccountPoolItem): void
  (e: 'schedule', account: UserAccountPoolItem): void
  (e: 'reauth', account: UserAccountPoolItem): void
  (e: 'refresh-token', account: UserAccountPoolItem): void
  (e: 'set-privacy', account: UserAccountPoolItem): void
  (e: 'recover-state', account: UserAccountPoolItem): void
  (e: 'toggle-scheduling', account: UserAccountPoolItem, schedulable: boolean): void
  (e: 'edit', account: UserAccountPoolItem): void
  (e: 'delete', account: UserAccountPoolItem): void
}>()

const { t } = useI18n()

const supportsReAuth = computed(() => props.account?.platform === 'openai' && props.account?.type === 'oauth')
const supportsRefresh = computed(() => props.account?.platform === 'openai' && props.account?.type === 'oauth')
const supportsPrivacy = computed(() => {
  const account = props.account
  return account?.type === 'oauth' && (account.platform === 'openai' || account.platform === 'antigravity')
})
const isRateLimited = computed(() => {
  if (props.account?.rate_limit_reset_at && new Date(props.account.rate_limit_reset_at) > new Date()) {
    return true
  }
  const modelLimits = props.account?.extra?.model_rate_limits as
    | Record<string, { rate_limit_reset_at: string }>
    | undefined
  if (!modelLimits) return false
  const now = new Date()
  return Object.values(modelLimits).some((info) => new Date(info.rate_limit_reset_at) > now)
})
const isOverloaded = computed(() => props.account?.overload_until && new Date(props.account.overload_until) > new Date())
const isTempUnschedulable = computed(() => props.account?.temp_unschedulable_until && new Date(props.account.temp_unschedulable_until) > new Date())
const hasRecoverableState = computed(() =>
  props.account?.status === 'error' ||
  Boolean(isRateLimited.value) ||
  Boolean(isOverloaded.value) ||
  Boolean(isTempUnschedulable.value)
)

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      window.addEventListener('keydown', handleKeydown)
    } else {
      window.removeEventListener('keydown', handleKeydown)
    }
  },
  { immediate: true },
)

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>
