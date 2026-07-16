import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { ApiKey } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  chatCompletion,
  createConversation as createConversationAPI,
  deleteConversation as deleteConversationAPI,
  fetchModelCatalog,
  listActiveKeys,
  listConversations,
  listMessages,
  reasoningEffortsForPlatform,
  updateConversation,
  type Attachment,
  type ChatModelOption,
  type ChatMessage,
  type Conversation,
  type ReasoningEffort,
  type ReasoningOption,
} from '@/api/chat'

function uid(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 10)
}

function messageKey(id: number): string {
  return String(id)
}

function contentWithTextAttachments(text: string, attachments: Attachment[]): string {
  let result = text.trim()
  for (const attachment of attachments) {
    if (attachment.kind === 'text' && attachment.text) {
      result += `\n\n[File ${attachment.name}]\n${attachment.text}`
    } else {
      result += `\n\n[Attachment: ${attachment.name} (${attachment.mime || 'file'})]`
    }
  }
  return result.trim()
}

export const useChatStore = defineStore('chat', () => {
  const keys = ref<ApiKey[]>([])
  const selectedKeyId = ref<number | null>(null)
  const models = ref<string[]>([])
  const modelCatalog = ref<ChatModelOption[]>([])
  const currentModelId = ref('')
  const currentReasoningEffort = ref<ReasoningEffort>('')
  const conversations = ref<Conversation[]>([])
  const messagesByConv = ref<Record<string, ChatMessage[]>>({})
  const currentConvId = ref<number | null>(null)
  const sending = ref(false)
  const loadingModels = ref(false)
  const initialized = ref(false)
  const notice = ref('')

  const selectedKey = computed(() => keys.value.find((key) => key.id === selectedKeyId.value))
  const hasKey = computed(() => !!selectedKey.value)
  const currentConversation = computed(() => conversations.value.find((item) => item.id === currentConvId.value))
  const currentMessages = computed<ChatMessage[]>(() =>
    currentConvId.value === null ? [] : messagesByConv.value[messageKey(currentConvId.value)] ?? [],
  )
  const currentReasoningOptions = computed<ReasoningOption[]>(() => {
    const manifestOptions = modelCatalog.value.find(
      (model) => model.id === currentModelId.value,
    )?.reasoningOptions
    if (manifestOptions?.length) {
      return [{ value: '', label: 'Default' }, ...manifestOptions]
    }
    return reasoningEffortsForPlatform(
      selectedKey.value?.group?.platform,
      currentModelId.value,
    ).map((value) => ({ value, label: value || 'Default' }))
  })

  function normalizeCurrentReasoningEffort(): void {
    const allowed = currentReasoningOptions.value.map((option) => option.value)
    if (!allowed.includes(currentReasoningEffort.value)) setReasoningEffort('')
  }

  async function newConversation(): Promise<Conversation | null> {
    try {
      const conversation = await createConversationAPI({
        title: 'New chat',
        model: currentModelId.value,
        apiKeyId: selectedKeyId.value ?? undefined,
        reasoningEffort: currentReasoningEffort.value,
      })
      conversations.value.unshift(conversation)
      messagesByConv.value[messageKey(conversation.id)] = []
      currentConvId.value = conversation.id
      return conversation
    } catch (error) {
      notice.value = `创建会话失败: ${extractApiErrorMessage(error)}`
      return null
    }
  }

  async function selectConversation(id: number): Promise<void> {
    const conversation = conversations.value.find((item) => item.id === id)
    if (!conversation) return
    currentConvId.value = id
    if (conversation.model) currentModelId.value = conversation.model
    currentReasoningEffort.value = conversation.reasoningEffort
    let shouldReloadModels = models.value.length === 0
    if (conversation.apiKeyId && keys.value.some((key) => key.id === conversation.apiKeyId)) {
      shouldReloadModels = shouldReloadModels || selectedKeyId.value !== conversation.apiKeyId
      selectedKeyId.value = conversation.apiKeyId
    }
    if (shouldReloadModels) await loadModels()
    else normalizeCurrentReasoningEffort()
    const key = messageKey(id)
    if (messagesByConv.value[key] !== undefined) return
    try {
      messagesByConv.value[key] = await listMessages(id)
    } catch (error) {
      notice.value = `加载会话失败: ${extractApiErrorMessage(error)}`
    }
  }

  async function removeConversation(id: number): Promise<void> {
    if (sending.value && currentConvId.value === id) return
    try {
      await deleteConversationAPI(id)
      conversations.value = conversations.value.filter((item) => item.id !== id)
      delete messagesByConv.value[messageKey(id)]
      if (currentConvId.value !== id) return
      currentConvId.value = null
      const next = conversations.value[0]
      if (next) await selectConversation(next.id)
      else await newConversation()
    } catch (error) {
      notice.value = `删除会话失败: ${extractApiErrorMessage(error)}`
    }
  }

  function setModel(id: string): void {
    currentModelId.value = id
    normalizeCurrentReasoningEffort()
    const conversation = currentConversation.value
    if (!conversation) return
    conversation.model = id
    void updateConversation(conversation.id, { model: id }).catch(() => undefined)
  }

  function setReasoningEffort(effort: ReasoningEffort): void {
    const allowed = currentReasoningOptions.value.map((option) => option.value)
    const nextEffort = allowed.includes(effort) ? effort : ''
    currentReasoningEffort.value = nextEffort
    const conversation = currentConversation.value
    if (!conversation) return
    conversation.reasoningEffort = nextEffort
    void updateConversation(conversation.id, { reasoningEffort: nextEffort }).catch(() => undefined)
  }

  async function selectKey(id: number): Promise<void> {
    selectedKeyId.value = id
    const conversation = currentConversation.value
    if (conversation) {
      conversation.apiKeyId = id
      void updateConversation(conversation.id, { apiKeyId: id }).catch(() => undefined)
    }
    await loadModels()
  }

  async function loadModels(): Promise<void> {
    const key = selectedKey.value
    if (!key) return
    loadingModels.value = true
    notice.value = ''
    try {
      modelCatalog.value = await fetchModelCatalog(key.key, key.group?.platform)
      models.value = modelCatalog.value.map((model) => model.id)
      if (models.value.length === 0) {
        notice.value = '该 Key 没有可用模型(检查分组/渠道配置)'
      } else if (!models.value.includes(currentModelId.value)) {
        setModel(models.value[0])
      }
      normalizeCurrentReasoningEffort()
    } catch (error) {
      models.value = []
      modelCatalog.value = []
      notice.value = `加载模型失败: ${extractApiErrorMessage(error)}`
    } finally {
      loadingModels.value = false
    }
  }

  async function sendMessage(text: string, attachments: Attachment[] = []): Promise<void> {
    const content = contentWithTextAttachments(text, attachments)
    if (!content || sending.value) return
    const key = selectedKey.value
    if (!key) {
      notice.value = '请先选择一个可用的 API Key'
      return
    }
    let conversation: Conversation | null | undefined = currentConversation.value
    if (!conversation) conversation = await newConversation()
    if (!conversation) return

    const conversationID = conversation.id
    const listKey = messageKey(conversationID)
    const list = messagesByConv.value[listKey] ?? (messagesByConv.value[listKey] = [])
    const clientMessageID = uid()
    list.push({
      id: clientMessageID,
      role: 'user',
      content: text.trim(),
      attachments: attachments.length ? attachments : undefined,
      createdAt: Date.now(),
    })
    const assistant: ChatMessage = {
      id: `${clientMessageID}-assistant`,
      role: 'assistant',
      content: '',
      model: currentModelId.value,
      pending: true,
      createdAt: Date.now(),
    }
    list.push(assistant)
    if (conversation.title === 'New chat' || conversation.title === '新对话') {
      conversation.title = text.trim().slice(0, 80) || 'New chat'
    }
    conversation.updatedAt = Date.now()
    sending.value = true
    notice.value = ''
    try {
      const result = await chatCompletion(
        conversationID,
        key.id,
        currentModelId.value,
        currentReasoningEffort.value,
        content,
        clientMessageID,
        attachments,
        (delta, model) => {
          assistant.content += delta
          if (model) assistant.model = model
        },
      )
      assistant.content = result.content || assistant.content || '(空回复)'
      assistant.model = result.model
    } catch (error) {
      assistant.content = extractApiErrorMessage(error)
      assistant.error = true
    } finally {
      assistant.pending = false
      sending.value = false
      try {
        messagesByConv.value[listKey] = await listMessages(conversationID)
        const refreshed = await listConversations()
        conversations.value = refreshed
      } catch {
        // Keep the optimistic messages when the history refresh fails.
      }
    }
  }

  async function init(): Promise<void> {
    if (initialized.value) return
    initialized.value = true
    try {
      const [activeKeys, savedConversations] = await Promise.all([listActiveKeys(), listConversations()])
      keys.value = activeKeys
      conversations.value = savedConversations
      if (keys.value.length > 0) {
        selectedKeyId.value = keys.value[0].id
        await loadModels()
      } else {
        notice.value = 'noKeys'
      }
      if (conversations.value.length > 0) {
        await selectConversation(conversations.value[0].id)
      } else {
        await newConversation()
      }
    } catch (error) {
      notice.value = `加载聊天失败: ${extractApiErrorMessage(error)}`
    }
  }

  return {
    keys,
    selectedKeyId,
    selectedKey,
    models,
    modelCatalog,
    currentModelId,
    currentReasoningEffort,
    conversations,
    currentConvId,
    sending,
    loadingModels,
    notice,
    hasKey,
    currentConversation,
    currentMessages,
    currentReasoningOptions,
    newConversation,
    selectConversation,
    removeConversation,
    setModel,
    setReasoningEffort,
    selectKey,
    sendMessage,
    init,
  }
})
