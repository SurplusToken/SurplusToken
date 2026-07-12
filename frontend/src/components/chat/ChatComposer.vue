<template>
  <div class="border-t border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900">
    <!-- Model row -->
    <div class="mb-2 flex items-center gap-2">
      <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('chat.model') }}</label>
      <div class="relative">
        <select
          :value="currentModelId"
          class="rounded-lg border border-gray-200 bg-gray-50 py-1 pl-2 pr-7 text-xs text-gray-800 focus:border-primary-400 focus:outline-none dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
          @change="onModelChange"
        >
          <optgroup v-for="group in groupedModels" :key="group.name" :label="group.name">
            <option v-for="m in group.items" :key="m.id" :value="m.id">{{ m.label }}</option>
          </optgroup>
        </select>
      </div>
      <span
        v-if="currentModel"
        class="rounded-full px-2 py-0.5 text-[10px] font-medium"
        :class="currentModel.route === 'web'
          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
          : 'bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300'"
      >
        {{ currentModel.route === 'web' ? t('chat.badgeWeb') : t('chat.badgeApi') }}
      </span>
    </div>

    <!-- Input row -->
    <div class="flex items-end gap-2">
      <textarea
        ref="taRef"
        v-model="draft"
        :placeholder="t('chat.placeholder')"
        rows="1"
        class="max-h-40 min-h-[42px] flex-1 resize-none rounded-xl border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm text-gray-800 focus:border-primary-400 focus:outline-none dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
        @input="autoGrow"
        @keydown.enter.exact.prevent="submit"
      ></textarea>
      <button
        type="button"
        class="flex h-[42px] w-[42px] flex-shrink-0 items-center justify-center rounded-xl bg-primary-600 text-white transition-colors hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-50"
        :disabled="sending || !draft.trim()"
        @click="submit"
      >
        <svg v-if="!sending" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 12L3.269 3.126A59.77 59.77 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
        </svg>
        <svg v-else class="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      </button>
    </div>
    <p class="mt-1.5 px-1 text-[11px] text-gray-400 dark:text-gray-500">{{ t('chat.hint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ChatModel } from '@/api/chat'

const props = defineProps<{ models: ChatModel[]; currentModelId: string; sending: boolean }>()
const emit = defineEmits<{ (e: 'update:model', id: string): void; (e: 'send', text: string): void }>()

const { t } = useI18n()
const draft = ref('')
const taRef = ref<HTMLTextAreaElement | null>(null)

const currentModel = computed(() => props.models.find((m) => m.id === props.currentModelId))

const groupedModels = computed(() => {
  const groups: { name: string; items: ChatModel[] }[] = []
  for (const m of props.models) {
    let g = groups.find((x) => x.name === m.group)
    if (!g) {
      g = { name: m.group, items: [] }
      groups.push(g)
    }
    g.items.push(m)
  }
  return groups
})

function onModelChange(e: Event) {
  emit('update:model', (e.target as HTMLSelectElement).value)
}

function autoGrow() {
  const el = taRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 160) + 'px'
}

function submit() {
  const text = draft.value.trim()
  if (!text || props.sending) return
  emit('send', text)
  draft.value = ''
  void nextTick(() => autoGrow())
}
</script>
