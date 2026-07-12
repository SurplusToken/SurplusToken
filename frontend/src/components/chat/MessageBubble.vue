<template>
  <div class="group flex gap-3" :class="isUser ? 'flex-row-reverse' : 'flex-row'">
    <!-- Avatar -->
    <div
      class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-xs font-semibold"
      :class="isUser
        ? 'bg-primary-600 text-white'
        : 'bg-gray-200 text-gray-700 dark:bg-dark-700 dark:text-gray-200'"
    >
      {{ isUser ? 'You' : 'AI' }}
    </div>

    <!-- Bubble -->
    <div class="min-w-0 max-w-[80%]">
      <div
        v-if="!isUser && message.model"
        class="mb-1 flex items-center gap-2 text-xs text-gray-400 dark:text-gray-500"
      >
        <span>{{ message.model }}</span>
        <button
          v-if="message.content && !message.pending"
          type="button"
          class="opacity-0 transition-opacity hover:text-primary-500 group-hover:opacity-100"
          :title="t('chat.copy')"
          @click="copyContent"
        >
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.6"><path stroke-linecap="round" stroke-linejoin="round" d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 011.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 00-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 01-1.125-1.125v-9.25m11.25 5.5H16.5a1.125 1.125 0 01-1.125-1.125v-3.5m5.25 5.625h-.008" /></svg>
        </button>
      </div>

      <!-- Attachments (user) -->
      <div v-if="message.attachments && message.attachments.length" class="mb-1.5 flex flex-wrap gap-2" :class="isUser ? 'justify-end' : ''">
        <template v-for="a in message.attachments" :key="a.id">
          <a
            v-if="a.kind === 'image'"
            :href="a.dataUrl"
            :download="a.name"
            target="_blank"
            rel="noopener"
            class="block overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600"
          >
            <img :src="a.dataUrl" :alt="a.name" class="h-24 w-24 object-cover" />
          </a>
          <a
            v-else
            :href="a.dataUrl"
            :download="a.name"
            class="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700 hover:border-primary-400 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200"
          >
            <svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.6"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" /></svg>
            <span class="max-w-[140px] truncate">{{ a.name }}</span>
          </a>
        </template>
      </div>

      <div
        v-if="message.content || message.pending || (message.attachments && message.attachments.length)"
        class="rounded-2xl px-4 py-2.5 text-sm leading-relaxed"
        :class="bubbleClass"
      >
        <!-- User: plain text -->
        <p v-if="isUser" class="whitespace-pre-wrap break-words">{{ message.content }}</p>

        <!-- Assistant: markdown -->
        <template v-else>
          <div v-if="message.content" class="md break-words" v-html="renderedHtml"></div>
          <span v-if="message.pending" class="typing-cursor" aria-hidden="true"></span>
          <span v-if="message.pending && !message.content" class="text-gray-400 dark:text-gray-500">{{ t('chat.thinking') }}</span>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useI18n } from 'vue-i18n'
import type { ChatMessage } from '@/api/chat'

const props = defineProps<{ message: ChatMessage }>()
const { t } = useI18n()

marked.setOptions({ gfm: true, breaks: true })

const isUser = computed(() => props.message.role === 'user')

const bubbleClass = computed(() => {
  if (isUser.value) return 'bg-primary-600 text-white'
  if (props.message.error) return 'bg-red-50 text-red-700 ring-1 ring-red-200 dark:bg-red-500/10 dark:text-red-300 dark:ring-red-500/30'
  return 'bg-white text-gray-800 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-gray-100 dark:ring-dark-700'
})

const renderedHtml = computed(() => {
  const raw = marked.parse(props.message.content, { async: false }) as string
  return DOMPurify.sanitize(raw)
})

async function copyContent() {
  try {
    await navigator.clipboard.writeText(props.message.content)
  } catch {
    /* clipboard unavailable */
  }
}
</script>

<style scoped>
.typing-cursor {
  display: inline-block;
  width: 7px;
  height: 1em;
  vertical-align: text-bottom;
  margin-left: 2px;
  background: currentColor;
  opacity: 0.7;
  animation: blink 1s steps(2, start) infinite;
}
@keyframes blink {
  50% {
    opacity: 0;
  }
}

.md :deep(p) {
  margin: 0 0 0.6em;
}
.md :deep(p:last-child) {
  margin-bottom: 0;
}
.md :deep(h1),
.md :deep(h2),
.md :deep(h3) {
  font-weight: 600;
  margin: 0.8em 0 0.4em;
  line-height: 1.3;
}
.md :deep(h1) {
  font-size: 1.25em;
}
.md :deep(h2) {
  font-size: 1.15em;
}
.md :deep(h3) {
  font-size: 1.05em;
}
.md :deep(ul),
.md :deep(ol) {
  margin: 0.4em 0 0.6em;
  padding-left: 1.4em;
}
.md :deep(li) {
  margin: 0.2em 0;
}
.md :deep(a) {
  color: rgb(37 99 235);
  text-decoration: underline;
}
.dark .md :deep(a) {
  color: rgb(96 165 250);
}
.md :deep(img) {
  max-width: 100%;
  border-radius: 8px;
  margin: 0.4em 0;
}
.md :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.88em;
  background: rgba(120, 120, 120, 0.16);
  padding: 0.12em 0.35em;
  border-radius: 4px;
}
.md :deep(pre) {
  background: rgb(17 24 39);
  color: rgb(229 231 235);
  border-radius: 10px;
  padding: 0.85em 1em;
  overflow-x: auto;
  margin: 0.6em 0;
}
.md :deep(pre code) {
  background: transparent;
  padding: 0;
  font-size: 0.85em;
  color: inherit;
}
.md :deep(blockquote) {
  border-left: 3px solid rgba(120, 120, 120, 0.4);
  padding-left: 0.8em;
  margin: 0.6em 0;
  color: rgba(120, 120, 120, 1);
}
.md :deep(table) {
  border-collapse: collapse;
  margin: 0.6em 0;
  font-size: 0.9em;
}
.md :deep(th),
.md :deep(td) {
  border: 1px solid rgba(120, 120, 120, 0.3);
  padding: 0.35em 0.6em;
}
</style>
