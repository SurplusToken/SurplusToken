import { apiClient } from './client'
import { buildApiUrl, buildGatewayUrl } from './url'
import { list as listKeys } from './keys'
import type { ApiKey, PaginatedResponse } from '@/types'

export type AttachmentKind = 'image' | 'text' | 'file'
export type ReasoningEffort =
  | ''
  | 'none'
  | 'minimal'
  | 'low'
  | 'medium'
  | 'high'
  | 'xhigh'
  | 'max'
  | 'ultra'

export interface ReasoningOption {
  value: ReasoningEffort
  label: string
  description?: string
}

export interface ChatModelOption {
  id: string
  label: string
  reasoningOptions: ReasoningOption[]
}

export function reasoningEffortsForPlatform(platform?: string, model = ''): ReasoningEffort[] {
  if (platform === 'antigravity' && model.trim().toLowerCase().startsWith('gemini-')) {
    platform = 'gemini'
  }
  switch (platform) {
    case 'openai':
    case 'grok':
      return ['', 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max']
    case 'anthropic':
    case 'antigravity':
      return ['', 'low', 'medium', 'high', 'xhigh', 'max']
    case 'gemini':
      return ['', 'minimal', 'low', 'medium', 'high']
    default:
      return ['']
  }
}

const reasoningEffortValues = new Set<ReasoningEffort>([
  'none',
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
  'ultra',
])

function asReasoningEffort(value: unknown): ReasoningEffort | undefined {
  if (typeof value !== 'string') return undefined
  const normalized = value.trim().toLowerCase() as ReasoningEffort
  return reasoningEffortValues.has(normalized) ? normalized : undefined
}

export function parseCodexModelsManifest(body: unknown): ChatModelOption[] {
  if (!body || typeof body !== 'object') return []
  const models = (body as { models?: unknown }).models
  if (!Array.isArray(models)) return []

  return models.flatMap((rawModel): ChatModelOption[] => {
    if (!rawModel || typeof rawModel !== 'object') return []
    const model = rawModel as Record<string, unknown>
    const visibility = typeof model.visibility === 'string' ? model.visibility : ''
    if (visibility && visibility !== 'list') return []
    const idValue = typeof model.slug === 'string' ? model.slug : model.id
    if (typeof idValue !== 'string' || !idValue.trim()) return []
    const id = idValue.trim()
    const displayName = typeof model.display_name === 'string' ? model.display_name.trim() : ''
    const levels = Array.isArray(model.supported_reasoning_levels)
      ? model.supported_reasoning_levels
      : []
    const seen = new Set<ReasoningEffort>()
    const reasoningOptions = levels.flatMap((rawLevel): ReasoningOption[] => {
      const level: Record<string, unknown> = rawLevel && typeof rawLevel === 'object'
        ? (rawLevel as Record<string, unknown>)
        : { effort: rawLevel }
      const effort = asReasoningEffort(level.effort)
      if (!effort || seen.has(effort)) return []
      seen.add(effort)
      return [{
        value: effort,
        label: effort,
        description: typeof level.description === 'string' ? level.description : undefined,
      }]
    })
    return [{ id, label: displayName || id, reasoningOptions }]
  })
}

export interface Attachment {
  id: string
  name: string
  mime: string
  kind: AttachmentKind
  size: number
  dataUrl: string
  text?: string
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  model?: string
  attachments?: Attachment[]
  pending?: boolean
  error?: boolean
  createdAt: number
}

export interface Conversation {
  id: number
  title: string
  model: string
  apiKeyId?: number
  reasoningEffort: ReasoningEffort
  updatedAt: number
}

interface ServerConversation {
  id: number
  title: string
  model: string
  api_key_id?: number
  reasoning_effort?: ReasoningEffort
  updated_at: string
}

interface ServerMessage {
  id: number
  role: 'user' | 'assistant' | 'system'
  content: string
  model?: string
  status: 'pending' | 'completed' | 'error' | 'interrupted'
  error_message?: string
  created_at: string
}

export interface ChatResult {
  content: string
  model: string
  usage?: unknown
}

interface ResponsesCitation {
  type?: string
  url?: string
  title?: string
}

interface ResponsesOutputItem {
  type?: string
  content?: Array<{
    type?: string
    text?: string
    annotations?: ResponsesCitation[]
  }>
  action?: { sources?: ResponsesCitation[] }
}

interface ResponsesResult {
  id?: string
  model?: string
  status?: string
  usage?: unknown
  error?: { message?: string }
  output?: ResponsesOutputItem[]
}

interface MessagesContentBlock {
  type?: string
  text?: string
  citations?: ResponsesCitation[]
  content?: unknown
}

interface MessagesResult {
  id?: string
  model?: string
  content?: MessagesContentBlock[]
  usage?: unknown
  error?: { message?: string }
}

interface MessagesDelta {
  type?: string
  text?: string
  citation?: ResponsesCitation
}

function toConversation(value: ServerConversation): Conversation {
  return {
    id: value.id,
    title: value.title,
    model: value.model,
    apiKeyId: value.api_key_id,
    reasoningEffort: value.reasoning_effort || '',
    updatedAt: Date.parse(value.updated_at) || Date.now(),
  }
}

function toMessage(value: ServerMessage): ChatMessage | null {
  if (value.role === 'system') return null
  const failed = value.status === 'error' || value.status === 'interrupted'
  return {
    id: String(value.id),
    role: value.role,
    content: failed && !value.content ? value.error_message || 'Generation failed' : value.content,
    model: value.model,
    pending: value.status === 'pending',
    error: failed,
    createdAt: Date.parse(value.created_at) || Date.now(),
  }
}

export async function listActiveKeys(): Promise<ApiKey[]> {
  const res = await listKeys(1, 100)
  return (res.items ?? []).filter((key) => key.status === 'active' && !!key.key)
}

function collectResponsesResult(result: ResponsesResult | undefined): {
  text: string
  citations: ResponsesCitation[]
} {
  let text = ''
  const citations: ResponsesCitation[] = []
  for (const output of result?.output ?? []) {
    for (const part of output.content ?? []) {
      if (part.type === 'output_text') text += part.text ?? ''
      citations.push(...(part.annotations ?? []))
    }
    citations.push(...(output.action?.sources ?? []))
  }
  return { text, citations }
}

function appendSourceLinks(content: string, citations: ResponsesCitation[]): string {
  const seen = new Set<string>()
  const links: string[] = []
  for (const citation of citations) {
    const rawUrl = citation.url?.trim()
    if (!rawUrl || seen.has(rawUrl)) continue
    try {
      const parsed = new URL(rawUrl)
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') continue
      seen.add(rawUrl)
      const title = (citation.title?.trim() || parsed.hostname)
        .replace(/\\/g, '\\\\')
        .replace(/\[/g, '\\[')
        .replace(/]/g, '\\]')
      links.push(`- [${title}](<${rawUrl.replace(/>/g, '%3E')}>)`)
    } catch {
      // Ignore malformed citation URLs.
    }
  }
  return links.length ? `${content.trim()}\n\n### Sources\n${links.join('\n')}` : content
}

function collectMessagesContent(blocks: MessagesContentBlock[] | undefined): {
  text: string
  citations: ResponsesCitation[]
} {
  let text = ''
  const citations: ResponsesCitation[] = []
  for (const block of blocks ?? []) {
    if (block.type === 'text') text += block.text ?? ''
    citations.push(...(block.citations ?? []))
    if (Array.isArray(block.content)) citations.push(...(block.content as ResponsesCitation[]))
    else if (block.content && typeof block.content === 'object') {
      citations.push(block.content as ResponsesCitation)
    }
  }
  return { text, citations }
}

function mergeUsage(current: unknown, incoming: unknown): unknown {
  if (incoming === undefined) return current
  if (
    current &&
    incoming &&
    typeof current === 'object' &&
    typeof incoming === 'object' &&
    !Array.isArray(current) &&
    !Array.isArray(incoming)
  ) {
    const merged = { ...(current as Record<string, unknown>) }
    for (const [key, value] of Object.entries(incoming as Record<string, unknown>)) {
      if (value === 0 && typeof merged[key] === 'number' && merged[key] !== 0) continue
      merged[key] = value
    }
    return merged
  }
  return incoming
}

async function fetchOpenAIModels(key: string): Promise<ChatModelOption[]> {
  const response = await fetch(buildGatewayUrl('/v1/models'), {
    headers: { Authorization: `Bearer ${key}` },
  })
  if (!response.ok) throw new Error(`models HTTP ${response.status}`)
  const body = (await response.json()) as { data?: unknown }
  if (!Array.isArray(body.data)) return []
  return body.data
    .map((model) => (model && typeof model === 'object' ? (model as { id?: unknown }).id : undefined))
    .filter((id): id is string => typeof id === 'string')
    .map((id) => ({ id, label: id, reasoningOptions: [] }))
}

export async function fetchModelCatalog(key: string, platform?: string): Promise<ChatModelOption[]> {
  if (platform === 'openai') {
    try {
      const response = await fetch(buildGatewayUrl('/v1/models?codex_manifest=1'), {
        headers: { Authorization: `Bearer ${key}` },
      })
      if (response.ok) {
        const catalog = parseCodexModelsManifest(await response.json())
        if (catalog.length > 0) return catalog
      }
    } catch {
      // API-key OpenAI groups do not have a subscription manifest; use /v1/models below.
    }
  }
  return fetchOpenAIModels(key)
}

export async function fetchModels(key: string): Promise<string[]> {
  return (await fetchOpenAIModels(key)).map((model) => model.id)
}

export async function listConversations(): Promise<Conversation[]> {
  const { data } = await apiClient.get<PaginatedResponse<ServerConversation>>('/chat/conversations', {
    params: { page: 1, page_size: 100 },
  })
  return (data.items ?? []).map(toConversation)
}

export async function createConversation(input: {
  title?: string
  model?: string
  apiKeyId?: number
  reasoningEffort?: ReasoningEffort
}): Promise<Conversation> {
  const { data } = await apiClient.post<ServerConversation>('/chat/conversations', {
    title: input.title || 'New chat',
    model: input.model || '',
    api_key_id: input.apiKeyId,
    reasoning_effort: input.reasoningEffort || '',
  })
  return toConversation(data)
}

export async function updateConversation(
  id: number,
  input: { title?: string; model?: string; apiKeyId?: number; reasoningEffort?: ReasoningEffort },
): Promise<Conversation> {
  const payload: Record<string, unknown> = {}
  if (input.title !== undefined) payload.title = input.title
  if (input.model !== undefined) payload.model = input.model
  if (input.apiKeyId !== undefined) payload.api_key_id = input.apiKeyId
  if (input.reasoningEffort !== undefined) payload.reasoning_effort = input.reasoningEffort
  const { data } = await apiClient.patch<ServerConversation>(`/chat/conversations/${id}`, payload)
  return toConversation(data)
}

export async function deleteConversation(id: number): Promise<void> {
  await apiClient.delete(`/chat/conversations/${id}`)
}

export async function listMessages(conversationId: number): Promise<ChatMessage[]> {
  const { data } = await apiClient.get<ServerMessage[]>(`/chat/conversations/${conversationId}/messages`)
  return data.map(toMessage).filter((message): message is ChatMessage => message !== null)
}

export async function chatCompletion(
  conversationId: number,
  apiKeyId: number,
  model: string,
  reasoningEffort: ReasoningEffort,
  content: string,
  clientMessageId: string,
  attachments: Attachment[] = [],
  onDelta?: (delta: string, model?: string) => void,
): Promise<ChatResult> {
  const token = localStorage.getItem('auth_token')
  const response = await fetch(buildApiUrl(`/chat/conversations/${conversationId}/completions`), {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({
      api_key_id: apiKeyId,
      model,
      ...(reasoningEffort ? { reasoning_effort: reasoningEffort } : {}),
      content,
      client_message_id: clientMessageId,
      attachments: attachments.map((attachment) => ({
        name: attachment.name,
        mime: attachment.mime,
        kind: attachment.kind,
        data_url: attachment.kind === 'image' ? attachment.dataUrl : undefined,
      })),
      stream: true,
    }),
  })

  if (!response.ok) {
    const raw = await response.text()
    let message = raw || `HTTP ${response.status}`
    try {
      const parsed = JSON.parse(raw) as { error?: { message?: string }; message?: string }
      message = parsed.error?.message || parsed.message || message
    } catch {
      // Keep the text response.
    }
    throw new Error(message)
  }

  if (!response.body) throw new Error('Streaming response is unavailable')
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let pending = ''
  let contentResult = ''
  let responseModel = model
  let usage: unknown
  const citations: ResponsesCitation[] = []

  const consumeLine = (line: string) => {
    const trimmed = line.trim()
    if (!trimmed.startsWith('data:')) return
    const payload = trimmed.slice(5).trim()
    if (!payload || payload === '[DONE]') return
    let chunk: {
      type?: string
      delta?: string | MessagesDelta
      model?: string
      usage?: unknown
      error?: { message?: string }
      message?: string | MessagesResult
      annotation?: ResponsesCitation
      response?: ResponsesResult
      content_block?: MessagesContentBlock
    }
    try {
      chunk = JSON.parse(payload) as typeof chunk
    } catch {
      return
    }
    if (chunk.error?.message) throw new Error(chunk.error.message)
    if (chunk.type === 'error' && typeof chunk.message === 'string') throw new Error(chunk.message)
    if (chunk.response?.error?.message) throw new Error(chunk.response.error.message)
    if (chunk.model) responseModel = chunk.model
    if (chunk.response?.model) responseModel = chunk.response.model
    if (chunk.usage !== undefined) usage = mergeUsage(usage, chunk.usage)
    if (chunk.response?.usage !== undefined) usage = mergeUsage(usage, chunk.response.usage)
    if (chunk.annotation) citations.push(chunk.annotation)
    const completed = collectResponsesResult(chunk.response)
    citations.push(...completed.citations)
    if (!contentResult && completed.text) contentResult = completed.text
    let textDelta =
      chunk.type === 'response.output_text.delta' && typeof chunk.delta === 'string' ? chunk.delta : ''

    const messagesMessage = typeof chunk.message === 'object' ? chunk.message : undefined
    if (messagesMessage?.error?.message) throw new Error(messagesMessage.error.message)
    if (messagesMessage?.model) responseModel = messagesMessage.model
    if (messagesMessage?.usage !== undefined) usage = mergeUsage(usage, messagesMessage.usage)
    const messageContent = collectMessagesContent(messagesMessage?.content)
    citations.push(...messageContent.citations)
    if (!contentResult && messageContent.text) contentResult = messageContent.text

    const blockContent = collectMessagesContent(chunk.content_block ? [chunk.content_block] : undefined)
    citations.push(...blockContent.citations)
    if (blockContent.text) textDelta += blockContent.text
    if (chunk.delta && typeof chunk.delta === 'object') {
      if (chunk.delta.type === 'text_delta') textDelta += chunk.delta.text ?? ''
      if (chunk.delta.citation) citations.push(chunk.delta.citation)
    }
    if (textDelta) {
      contentResult += textDelta
      onDelta?.(textDelta, messagesMessage?.model || chunk.response?.model || chunk.model)
    }
  }

  while (true) {
    const { value, done } = await reader.read()
    pending += decoder.decode(value, { stream: !done })
    const lines = pending.split(/\r?\n/)
    pending = lines.pop() || ''
    for (const line of lines) consumeLine(line)
    if (done) break
  }
  if (pending) consumeLine(pending)
  return { content: appendSourceLinks(contentResult, citations), model: responseModel, usage }
}
