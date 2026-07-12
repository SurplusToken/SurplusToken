/**
 * Chat (playground) store.
 *
 * DEMO_MODE = true: fabricates assistant replies locally (typewriter effect) so
 * the ChatView can be previewed on staging without any backend. Flip to false
 * once the dashboard chat endpoint + chatgpt-web bridge are wired.
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { chatAPI, type ChatModel, type ChatMessage, type Conversation, type ChatRoute } from '@/api/chat'

const DEMO_MODE = true

// Merged model list: real API models + web-only (bridge) models, each tagged
// with its route so the UI can badge it and bill accordingly.
const DEMO_MODELS: ChatModel[] = [
  { id: 'claude-sonnet-4-5', label: 'Claude Sonnet 4.5', route: 'api', group: 'API' },
  { id: 'claude-opus-4-6', label: 'Claude Opus 4.6', route: 'api', group: 'API' },
  { id: 'gpt-5', label: 'GPT-5', route: 'api', group: 'API' },
  { id: 'gemini-2.5-pro', label: 'Gemini 2.5 Pro', route: 'api', group: 'API' },
  { id: 'web:pro', label: 'Pro', route: 'web', group: 'ChatGPT 网页版' },
  { id: 'web:extra-high', label: 'Extra High', route: 'web', group: 'ChatGPT 网页版' },
  { id: 'web:high', label: 'High', route: 'web', group: 'ChatGPT 网页版' },
  { id: 'web:instant', label: 'Instant 5.5', route: 'web', group: 'ChatGPT 网页版' },
  { id: 'web:gpt-5.6-sol', label: 'GPT-5.6 Sol', route: 'web', group: 'ChatGPT 网页版' },
  { id: 'web:o3', label: 'o3', route: 'web', group: 'ChatGPT 网页版' },
]

function uid(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}

// A canned, markdown-rich reply so the preview exercises headings / lists /
// inline code / fenced code-block rendering.
function demoReply(prompt: string, modelLabel: string): string {
  const trimmed = prompt.length > 60 ? prompt.slice(0, 60) + '…' : prompt
  return [
    `收到你的消息:「**${trimmed}**」。这是 **${modelLabel}** 的一条演示回复,用来预览渲染效果。`,
    '',
    '要点如下:',
    '',
    '1. 这是一个 **Markdown** 渲染演示(标题、列表、代码都能显示)',
    '2. 行内代码像这样:`const x = 42`',
    '3. 代码块会带独立样式:',
    '',
    '```ts',
    'function greet(name: string): string {',
    '  return `Hello, ${name}!`',
    '}',
    'console.log(greet("SurplusAI"))',
    '```',
    '',
    '> 说明:当前为演示模式,回复由前端本地生成;接上后端后这里就是真实模型输出。',
  ].join('\n')
}

export const useChatStore = defineStore('chat', () => {
  const models = ref<ChatModel[]>(DEMO_MODE ? DEMO_MODELS : [])
  const currentModelId = ref<string>(DEMO_MODELS[0].id)
  const conversations = ref<Conversation[]>([])
  const messagesByConv = ref<Record<string, ChatMessage[]>>({})
  const currentConvId = ref<string>('')
  const sending = ref(false)

  const currentModel = computed<ChatModel | undefined>(() =>
    models.value.find((m) => m.id === currentModelId.value),
  )
  const currentConversation = computed<Conversation | undefined>(() =>
    conversations.value.find((c) => c.id === currentConvId.value),
  )
  const currentMessages = computed<ChatMessage[]>(() =>
    currentConvId.value ? messagesByConv.value[currentConvId.value] ?? [] : [],
  )

  function routeOf(modelId: string): ChatRoute {
    return models.value.find((m) => m.id === modelId)?.route ?? 'api'
  }

  function newConversation(modelId = currentModelId.value): Conversation {
    const conv: Conversation = {
      id: uid(),
      title: '新对话',
      route: routeOf(modelId),
      model: modelId,
      updatedAt: Date.now(),
    }
    conversations.value.unshift(conv)
    messagesByConv.value[conv.id] = []
    currentConvId.value = conv.id
    currentModelId.value = modelId
    return conv
  }

  function selectConversation(id: string): void {
    const conv = conversations.value.find((c) => c.id === id)
    if (!conv) return
    currentConvId.value = id
    currentModelId.value = conv.model
  }

  function setModel(modelId: string): void {
    currentModelId.value = modelId
    const conv = currentConversation.value
    // Keep the current conversation only if the route matches; otherwise a fresh
    // send will spawn a new conversation (a conversation is locked to one route).
    if (conv && conv.route === routeOf(modelId)) {
      conv.model = modelId
    }
  }

  async function sendMessage(text: string): Promise<void> {
    const content = text.trim()
    if (!content || sending.value) return

    // Ensure we have a conversation whose route matches the chosen model.
    let conv = currentConversation.value
    if (!conv || conv.route !== routeOf(currentModelId.value)) {
      conv = newConversation(currentModelId.value)
    }
    const convId = conv.id
    const modelLabel = currentModel.value?.label ?? currentModelId.value

    const list = messagesByConv.value[convId] ?? (messagesByConv.value[convId] = [])
    list.push({ id: uid(), role: 'user', content, createdAt: Date.now() })
    if (conv.title === '新对话') conv.title = content.length > 24 ? content.slice(0, 24) + '…' : content
    conv.updatedAt = Date.now()

    const assistant: ChatMessage = {
      id: uid(),
      role: 'assistant',
      content: '',
      model: modelLabel,
      pending: true,
      createdAt: Date.now(),
    }
    list.push(assistant)
    sending.value = true

    try {
      if (DEMO_MODE) {
        await typewriter(assistant, demoReply(content, modelLabel))
      } else {
        const res = await chatAPI.sendMessage({ conversationId: convId, content, model: currentModelId.value })
        assistant.content = res.text
        assistant.model = res.model || modelLabel
      }
    } catch (e) {
      assistant.content = `⚠️ 出错了: ${e instanceof Error ? e.message : String(e)}`
    } finally {
      assistant.pending = false
      sending.value = false
      conv.updatedAt = Date.now()
    }
  }

  // Reveal `full` progressively to simulate streaming.
  function typewriter(msg: ChatMessage, full: string): Promise<void> {
    return new Promise((resolve) => {
      let i = 0
      const step = Math.max(2, Math.round(full.length / 120))
      const timer = setInterval(() => {
        i = Math.min(full.length, i + step)
        msg.content = full.slice(0, i)
        if (i >= full.length) {
          clearInterval(timer)
          resolve()
        }
      }, 16)
    })
  }

  async function init(): Promise<void> {
    if (!DEMO_MODE) {
      try {
        models.value = await chatAPI.getModels()
        if (models.value[0]) currentModelId.value = models.value[0].id
      } catch {
        // leave empty; UI will show no models
      }
    }
    if (conversations.value.length === 0) newConversation()
  }

  return {
    models,
    currentModelId,
    conversations,
    currentConvId,
    sending,
    currentModel,
    currentConversation,
    currentMessages,
    newConversation,
    selectConversation,
    setModel,
    sendMessage,
    init,
  }
})
