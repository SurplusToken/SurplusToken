<template>
  <div class="border-t border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900">
    <!-- Model row -->
    <div class="mb-2 flex flex-wrap items-center gap-x-4 gap-y-2">
      <div class="flex min-w-0 items-center gap-2">
        <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('chat.model') }}</label>
        <select
          :value="currentModelId"
          :disabled="disabled || models.length === 0"
          class="max-w-[240px] truncate rounded-lg border border-gray-200 bg-gray-50 py-1 pl-2 pr-7 text-xs text-gray-800 focus:border-primary-400 focus:outline-none disabled:opacity-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
          @change="onModelChange"
        >
          <option v-if="models.length === 0" value="">{{ t('chat.noModels') }}</option>
          <option v-for="m in modelOptions" :key="m.id" :value="m.id">{{ m.label }}</option>
        </select>
      </div>

      <div class="flex items-center gap-2">
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('chat.reasoning') }}</span>
        <select
          :value="reasoningEffort"
          :disabled="disabled || sending"
          class="h-7 rounded-md border border-gray-200 bg-gray-50 py-1 pl-2 pr-7 text-xs text-gray-800 focus:border-primary-400 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
          @change="onReasoningChange"
        >
          <option
            v-for="option in availableReasoningOptions"
            :key="option.value"
            :value="option.value"
            :title="option.description"
          >
            {{ option.label }}
          </option>
        </select>
      </div>
    </div>

    <!-- Attachment chips -->
    <div v-if="attachments.length" class="mb-2 flex flex-wrap gap-2">
      <div
        v-for="a in attachments"
        :key="a.id"
        class="group relative flex items-center gap-1.5 rounded-lg border border-gray-200 bg-gray-50 py-1 pl-1.5 pr-6 text-xs dark:border-dark-600 dark:bg-dark-800"
      >
        <img v-if="a.kind === 'image'" :src="a.dataUrl" class="h-6 w-6 rounded object-cover" alt="" />
        <svg v-else class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.6">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
        </svg>
        <span class="max-w-[120px] truncate text-gray-700 dark:text-gray-200">{{ a.name }}</span>
        <button
          type="button"
          class="absolute right-1 top-1/2 -translate-y-1/2 text-gray-400 hover:text-red-500"
          @click="removeAttachment(a.id)"
        >
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </div>
    </div>

    <!-- Input row -->
    <div class="flex items-end gap-2">
      <button
        type="button"
        :disabled="disabled"
        class="flex h-[42px] w-[42px] flex-shrink-0 items-center justify-center rounded-xl border border-gray-200 text-gray-500 transition-colors hover:bg-gray-100 disabled:opacity-40 dark:border-dark-600 dark:text-gray-400 dark:hover:bg-dark-800"
        :title="t('chat.attach')"
        @click="fileInput?.click()"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.6">
          <path stroke-linecap="round" stroke-linejoin="round" d="M18.375 12.739l-7.693 7.693a4.5 4.5 0 01-6.364-6.364l10.94-10.94A3 3 0 1119.5 7.372L8.552 18.32m.009-.01l-.01.01m5.699-9.941l-7.81 7.81a1.5 1.5 0 002.112 2.13" />
        </svg>
      </button>
      <input ref="fileInput" type="file" multiple class="hidden" @change="onFilesPicked" />

      <textarea
        ref="taRef"
        v-model="draft"
        :placeholder="disabled ? t('chat.disabledPlaceholder') : t('chat.placeholder')"
        :disabled="disabled"
        rows="1"
        class="max-h-40 min-h-[42px] flex-1 resize-none rounded-xl border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm text-gray-800 focus:border-primary-400 focus:outline-none disabled:opacity-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
        @input="autoGrow"
        @keydown.enter.exact.prevent="submit"
      ></textarea>

      <button
        type="button"
        class="flex h-[42px] w-[42px] flex-shrink-0 items-center justify-center rounded-xl bg-primary-600 text-white transition-colors hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-50"
        :disabled="disabled || sending || (!draft.trim() && attachments.length === 0)"
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
import { reasoningEffortsForPlatform } from '@/api/chat'
import type {
  Attachment,
  AttachmentKind,
  ChatModelOption,
  ReasoningEffort,
  ReasoningOption,
} from '@/api/chat'

const props = defineProps<{
  models: string[]
  modelCatalog?: ChatModelOption[]
  currentModelId: string
  platform?: string
  reasoningOptions?: ReasoningOption[]
  reasoningEffort: ReasoningEffort
  sending: boolean
  disabled?: boolean
}>()
const emit = defineEmits<{
  (e: 'update:model', id: string): void
  (e: 'update:reasoning', effort: ReasoningEffort): void
  (e: 'send', payload: { text: string; attachments: Attachment[] }): void
}>()

const { t } = useI18n()
const modelOptions = computed<ChatModelOption[]>(() =>
  props.modelCatalog?.length
    ? props.modelCatalog
    : props.models.map((id) => ({ id, label: id, reasoningOptions: [] })),
)
const availableReasoningOptions = computed<ReasoningOption[]>(() =>
  props.reasoningOptions?.length
    ? props.reasoningOptions
    : reasoningEffortsForPlatform(props.platform, props.currentModelId).map((value) => ({
        value,
        label: value || 'Default',
      })),
)
const draft = ref('')
const taRef = ref<HTMLTextAreaElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const attachments = ref<Attachment[]>([])

const MAX_BYTES = 10 * 1024 * 1024
const TEXT_EXT = /\.(txt|md|markdown|json|csv|tsv|log|ya?ml|xml|html?|css|js|ts|tsx|jsx|py|go|rs|java|c|cpp|h|sh|sql|toml|ini|env)$/i

function uid(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}

function kindOf(file: File): AttachmentKind {
  if (file.type.startsWith('image/')) return 'image'
  if (file.type.startsWith('text/') || TEXT_EXT.test(file.name)) return 'text'
  return 'file'
}

function readAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => resolve(String(r.result))
    r.onerror = () => reject(r.error)
    r.readAsDataURL(file)
  })
}
function readAsText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => resolve(String(r.result))
    r.onerror = () => reject(r.error)
    r.readAsText(file)
  })
}

async function onFilesPicked(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  for (const file of files) {
    if (file.size > MAX_BYTES) continue
    const kind = kindOf(file)
    try {
      const dataUrl = await readAsDataURL(file)
      const att: Attachment = { id: uid(), name: file.name, mime: file.type, kind, size: file.size, dataUrl }
      if (kind === 'text') att.text = await readAsText(file)
      attachments.value.push(att)
    } catch {
      /* skip unreadable file */
    }
  }
  input.value = '' // allow re-picking the same file
}

function removeAttachment(id: string) {
  attachments.value = attachments.value.filter((a) => a.id !== id)
}

function onModelChange(e: Event) {
  emit('update:model', (e.target as HTMLSelectElement).value)
}

function onReasoningChange(e: Event) {
  emit('update:reasoning', (e.target as HTMLSelectElement).value as ReasoningEffort)
}

function autoGrow() {
  const el = taRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 160) + 'px'
}

function submit() {
  const text = draft.value.trim()
  if ((!text && attachments.value.length === 0) || props.sending || props.disabled) return
  emit('send', { text, attachments: attachments.value.slice() })
  draft.value = ''
  attachments.value = []
  void nextTick(() => autoGrow())
}
</script>
