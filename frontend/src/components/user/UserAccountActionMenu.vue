<template>
  <Teleport to="body">
    <div v-if="show && account && position">
      <div class="fixed inset-0 z-[9998]" @click="emit('close')"></div>
      <div
        class="fixed z-[9999] w-52 overflow-hidden rounded-xl bg-white shadow-lg ring-1 ring-black/5 dark:bg-dark-800"
        :style="{ top: position.top + 'px', left: position.left + 'px' }"
        @click.stop
      >
        <div class="py-1">
          <button
            class="flex w-full items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
            @click="emit('stats', account); emit('close')"
          >
            <Icon name="chart" size="sm" class="text-indigo-500" />
            {{ t('admin.accounts.viewStats') }}
          </button>

          <template v-if="account.is_mine">
            <button
              class="flex w-full items-center gap-2 px-4 py-2 text-sm text-primary-600 hover:bg-gray-100 dark:text-primary-300 dark:hover:bg-dark-700"
              @click="emit('edit', account); emit('close')"
            >
              <Icon name="edit" size="sm" />
              {{ t('common.edit') }}
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
import { onUnmounted, watch } from 'vue'
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
  (e: 'stats', account: UserAccountPoolItem): void
  (e: 'edit', account: UserAccountPoolItem): void
  (e: 'delete', account: UserAccountPoolItem): void
}>()

const { t } = useI18n()

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
