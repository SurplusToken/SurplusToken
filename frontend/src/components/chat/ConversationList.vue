<template>
  <div class="flex h-full flex-col">
    <button
      type="button"
      class="mb-3 flex items-center justify-center gap-2 rounded-xl border border-dashed border-gray-300 px-3 py-2 text-sm font-medium text-gray-600 transition-colors hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:text-gray-300 dark:hover:border-primary-500 dark:hover:text-primary-400"
      @click="$emit('new')"
    >
      <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
      </svg>
      {{ t('chat.newChat') }}
    </button>

    <div class="min-h-0 flex-1 space-y-1 overflow-y-auto pr-1">
      <div
        v-for="conv in conversations"
        :key="conv.id"
        class="group flex w-full items-center rounded-lg text-sm transition-colors"
        :class="conv.id === currentId
          ? 'bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300'
          : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-800'"
      >
        <button type="button" class="flex min-w-0 flex-1 items-center gap-2 px-3 py-2 text-left" @click="$emit('select', conv.id)">
          <svg class="h-4 w-4 flex-shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.625 9.75a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H8.25m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H12m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0h-.375m-13.5 3.01c0 1.6 1.123 2.994 2.707 3.227 1.087.16 2.185.283 3.293.369V21l4.184-4.183a1.14 1.14 0 01.778-.332 48.294 48.294 0 005.83-.498c1.585-.233 2.708-1.626 2.708-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z" />
          </svg>
          <span class="min-w-0 flex-1 truncate">{{ conv.title }}</span>
        </button>
        <button
          type="button"
          class="mr-1 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded text-gray-400 opacity-0 hover:bg-red-50 hover:text-red-600 group-hover:opacity-100 focus:opacity-100 dark:hover:bg-red-500/10 dark:hover:text-red-300"
          :title="t('chat.deleteChat')"
          @click.stop="$emit('delete', conv.id)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.7">
            <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166M19.228 5.79L18.16 19.673A2.25 2.25 0 0115.916 21H8.084a2.25 2.25 0 01-2.244-1.327L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0V4.477c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
          </svg>
        </button>
      </div>

      <p
        v-if="conversations.length === 0"
        class="px-3 py-6 text-center text-xs text-gray-400 dark:text-gray-500"
      >
        {{ t('chat.noConversations') }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Conversation } from '@/api/chat'

defineProps<{ conversations: Conversation[]; currentId: number | null }>()
defineEmits<{ (e: 'new'): void; (e: 'select', id: number): void; (e: 'delete', id: number): void }>()

const { t } = useI18n()
</script>
