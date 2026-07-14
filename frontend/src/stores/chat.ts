/**
 * Chat (playground) store — real chat through the platform gateway using the
 * logged-in user's OWN API key. No ChatGPT-web bridge here (that comes later).
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ApiKey } from '@/types'
import {
  listActiveKeys,
  fetchModels,
  chatCompletion,
  type ChatMessage,
  type Conversation,
  type Attachment,
  type OAIMessage,
} from '@/api/chat'

function uid(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}

export const useChatStore = defineStore('chat', () => {
  const keys = ref<ApiKey[]>([])
  const selectedKeyId = ref<number | null>(null)
  const models = ref<string[]>([])
  const currentModelId = ref<string>('')
  const conversations = ref<Conversation[]>([])
  const messagesByConv = ref<Record<string, ChatMessage[]>>({})
  const currentConvId = ref<string>('')
  const sending = ref(false)
  const loadingModels = ref(false)
  const initialized = ref(false)
  const notice = ref<string>('') // non-fatal banner (no keys / model load failure)

  const selectedKey = computed(() => keys.value.find((k) => k.id === selectedKeyId.value))
  const hasKey = computed(() => !!selectedKey.value)
  const currentConversation = computed(() => conversations.value.find((c) => c.id === currentConvId.value))
  const currentMessages = computed<ChatMessage[]>(() =>
    currentConvId.value ? messagesByConv.value[currentConvId.value] ?? [] : [],
  )

  function newConversation(): Conversation {
    const conv: Conversation = { id: uid(), title: '新对话', model: currentModelId.value, updatedAt: Date.now() }
    conversations.value.unshift(conv)
    messagesByConv.value[conv.id] = []
    currentConvId.value = conv.id
    return conv
  }

  function selectConversation(id: string): void {
    if (!messagesByConv.value[id]) return
    currentConvId.value = id
    const c = currentConversation.value
    if (c?.model) currentModelId.value = c.model
  }

  function setModel(id: string): void {
    currentModelId.value = id
    const c = currentConversation.value
    if (c) c.model = id
  }

  async function selectKey(id: number): Promise<void> {
    selectedKeyId.value = id
    await loadModels()
  }

  async function loadModels(): Promise<void> {
    const k = selectedKey.value
    if (!k) return
    loadingModels.value = true
    notice.value = ''
    try {
      models.value = await fetchModels(k.key)
      if (models.value.length === 0) {
        notice.value = '该 Key 没有可用模型(检查分组/渠道配置)'
      } else if (!models.value.includes(currentModelId.value)) {
        currentModelId.value = models.value[0]
      }
    } catch (e) {
      models.value = []
      notice.value = `加载模型失败: ${e instanceof Error ? e.message : String(e)}`
    } finally {
      loadingModels.value = false
    }
  }

  // Turn our stored history into an OpenAI messages array (attachments -> content parts).
  function buildOAIMessages(convId: string): OAIMessage[] {
    const list = messagesByConv.value[convId] ?? []
    const out: OAIMessage[] = []
    for (const m of list) {
      if (m.pending) continue
      if (m.role === 'user' && m.attachments && m.attachments.length > 0) {
        const parts: Exclude<OAIMessage['content'], string> = []
        if (m.content) parts.push({ type: 'text', text: m.content })
        for (const a of m.attachments) {
          if (a.kind === 'image') parts.push({ type: 'image_url', image_url: { url: a.dataUrl } })
          else if (a.kind === 'text' && a.text) parts.push({ type: 'text', text: `\n[文件 ${a.name}]\n${a.text}` })
          else parts.push({ type: 'text', text: `\n[附件: ${a.name} (${a.mime || 'file'})]` })
        }
        out.push({ role: 'user', content: parts })
      } else {
        out.push({ role: m.role, content: m.content })
      }
    }
    return out
  }

  async function sendMessage(text: string, attachments: Attachment[] = []): Promise<void> {
    const content = text.trim()
    if ((!content && attachments.length === 0) || sending.value) return
    const k = selectedKey.value
    if (!k) {
      notice.value = '请先选择一个可用的 API Key'
      return
    }

    let conv = currentConversation.value
    if (!conv) conv = newConversation()
    const convId = conv.id
    const model = currentModelId.value

    const list = messagesByConv.value[convId] ?? (messagesByConv.value[convId] = [])
    list.push({
      id: uid(),
      role: 'user',
      content,
      attachments: attachments.length ? attachments : undefined,
      createdAt: Date.now(),
    })
    if (conv.title === '新对话' && content) conv.title = content.length > 24 ? content.slice(0, 24) + '…' : content
    conv.updatedAt = Date.now()

    const assistant: ChatMessage = {
      id: uid(),
      role: 'assistant',
      content: '',
      model,
      pending: true,
      createdAt: Date.now(),
    }
    list.push(assistant)
    sending.value = true
    try {
      const msgs = buildOAIMessages(convId)
      const res = await chatCompletion(k.key, model, msgs)
      assistant.content = res.content || '(空回复)'
      assistant.model = res.model
    } catch (e) {
      assistant.content = `⚠️ ${e instanceof Error ? e.message : String(e)}`
      assistant.error = true
    } finally {
      assistant.pending = false
      sending.value = false
      conv.updatedAt = Date.now()
    }
  }

  async function init(): Promise<void> {
    if (initialized.value) return
    initialized.value = true
    try {
      keys.value = await listActiveKeys()
      if (keys.value.length > 0) {
        selectedKeyId.value = keys.value[0].id
        await loadModels()
      } else {
        notice.value = 'noKeys'
      }
    } catch (e) {
      notice.value = `加载 Key 失败: ${e instanceof Error ? e.message : String(e)}`
    }
    if (conversations.value.length === 0) newConversation()
  }

  return {
    keys,
    selectedKeyId,
    selectedKey,
    models,
    currentModelId,
    conversations,
    currentConvId,
    sending,
    loadingModels,
    notice,
    hasKey,
    currentConversation,
    currentMessages,
    newConversation,
    selectConversation,
    setModel,
    selectKey,
    sendMessage,
    init,
  }
})
