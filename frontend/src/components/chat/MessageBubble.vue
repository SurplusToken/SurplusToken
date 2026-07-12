<template>
  <div class="flex gap-3" :class="isUser ? 'flex-row-reverse' : 'flex-row'">
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
        class="mb-1 text-xs text-gray-400 dark:text-gray-500"
      >
        {{ message.model }}
      </div>
      <div
        class="rounded-2xl px-4 py-2.5 text-sm leading-relaxed"
        :class="isUser
          ? 'bg-primary-600 text-white'
          : 'bg-white text-gray-800 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-gray-100 dark:ring-dark-700'"
      >
        <!-- User: plain text, preserve newlines -->
        <p v-if="isUser" class="whitespace-pre-wrap break-words">{{ message.content }}</p>

        <!-- Assistant: rendered markdown -->
        <template v-else>
          <div v-if="message.content" class="md break-words" v-html="renderedHtml"></div>
          <span v-if="message.pending" class="typing-cursor" aria-hidden="true"></span>
          <span
            v-if="message.pending && !message.content"
            class="text-gray-400 dark:text-gray-500"
          >思考中…</span>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import type { ChatMessage } from '@/api/chat'

const props = defineProps<{ message: ChatMessage }>()

marked.setOptions({ gfm: true, breaks: true })

const isUser = computed(() => props.message.role === 'user')

const renderedHtml = computed(() => {
  const raw = marked.parse(props.message.content, { async: false }) as string
  return DOMPurify.sanitize(raw)
})
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

/* Minimal markdown styling (no @tailwindcss/typography dependency) */
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
