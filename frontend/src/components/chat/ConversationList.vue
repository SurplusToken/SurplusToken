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
      <button
        v-for="conv in conversations"
        :key="conv.id"
        type="button"
        class="group flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm transition-colors"
        :class="conv.id === currentId
          ? 'bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300'
          : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-800'"
        @click="$emit('select', conv.id)"
      >
        <span
          class="inline-block h-1.5 w-1.5 flex-shrink-0 rounded-full"
          :class="conv.route === 'web' ? 'bg-emerald-500' : 'bg-sky-500'"
          :title="conv.route === 'web' ? t('chat.badgeWeb') : t('chat.badgeApi')"
        ></span>
        <span class="min-w-0 flex-1 truncate">{{ conv.title }}</span>
      </button>

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

defineProps<{ conversations: Conversation[]; currentId: string }>()
defineEmits<{ (e: 'new'): void; (e: 'select', id: string): void }>()

const { t } = useI18n()
</script>
