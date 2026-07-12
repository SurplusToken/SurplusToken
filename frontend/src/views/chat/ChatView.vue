<template>
  <AppLayout>
    <div
      class="flex overflow-hidden rounded-2xl border border-gray-200 bg-gray-50 shadow-sm dark:border-dark-700 dark:bg-dark-950"
      :style="{ height: 'calc(100vh - 9rem)', minHeight: '520px' }"
    >
      <!-- Left: conversations -->
      <aside class="hidden w-64 flex-shrink-0 flex-col border-r border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900 md:flex">
        <ConversationList
          :conversations="store.conversations"
          :current-id="store.currentConvId"
          @new="store.newConversation()"
          @select="store.selectConversation"
        />
      </aside>

      <!-- Right: chat column -->
      <section class="flex min-w-0 flex-1 flex-col">
        <!-- Header -->
        <header class="flex items-center justify-between border-b border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-900">
          <div class="min-w-0">
            <h2 class="truncate text-sm font-semibold text-gray-900 dark:text-white">
              {{ store.currentConversation?.title || t('chat.title') }}
            </h2>
            <p class="text-xs text-gray-400 dark:text-gray-500">{{ t('chat.subtitle') }}</p>
          </div>
          <span class="rounded-md bg-amber-100 px-2 py-1 text-[10px] font-medium text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">
            {{ t('chat.demoBadge') }}
          </span>
        </header>

        <!-- Messages -->
        <div ref="scrollRef" class="min-h-0 flex-1 space-y-5 overflow-y-auto px-4 py-5">
          <div
            v-if="store.currentMessages.length === 0"
            class="flex h-full flex-col items-center justify-center text-center text-gray-400 dark:text-gray-500"
          >
            <svg class="mb-3 h-10 w-10 opacity-60" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.4">
              <path stroke-linecap="round" stroke-linejoin="round" d="M8.625 12a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H8.25m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0H12m4.125 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 01-2.555-.337A5.972 5.972 0 015.41 20.97a5.969 5.969 0 01-.474-.065 4.48 4.48 0 00.978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25z" />
            </svg>
            <p class="text-sm">{{ t('chat.emptyState') }}</p>
          </div>

          <MessageBubble
            v-for="msg in store.currentMessages"
            :key="msg.id"
            :message="msg"
          />
        </div>

        <!-- Composer -->
        <ChatComposer
          :models="store.models"
          :current-model-id="store.currentModelId"
          :sending="store.sending"
          @update:model="store.setModel"
          @send="store.sendMessage"
        />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConversationList from '@/components/chat/ConversationList.vue'
import MessageBubble from '@/components/chat/MessageBubble.vue'
import ChatComposer from '@/components/chat/ChatComposer.vue'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()
const store = useChatStore()
const scrollRef = ref<HTMLElement | null>(null)

function scrollToBottom() {
  void nextTick(() => {
    const el = scrollRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

// Keep pinned to the bottom as messages arrive / stream in.
watch(
  () => {
    const msgs = store.currentMessages
    const last = msgs[msgs.length - 1]
    return `${store.currentConvId}:${msgs.length}:${last?.content.length ?? 0}`
  },
  scrollToBottom,
)

onMounted(() => {
  store.init()
  scrollToBottom()
})
</script>
