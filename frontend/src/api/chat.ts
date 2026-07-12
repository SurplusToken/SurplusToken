/**
 * Chat (playground) API — talks to the platform's OpenAI-compatible gateway using
 * the logged-in user's OWN API key (Design A: zero backend changes).
 *
 *   listActiveKeys()  -> JWT GET /api/v1/keys, filtered to active keys (raw sk-... included)
 *   fetchModels(key)  -> GET  {origin}/v1/models          (Bearer key)
 *   chatCompletion()  -> POST {origin}/v1/chat/completions (Bearer key, stream:false)
 *
 * Billing / quota / group routing / rate limits are all enforced by the gateway's
 * api-key middleware — nothing to reimplement here.
 */

import { buildGatewayUrl } from './url'
import { list as listKeys } from './keys'
import type { ApiKey } from '@/types'

export type AttachmentKind = 'image' | 'text' | 'file'

export interface Attachment {
  id: string
  name: string
  mime: string
  kind: AttachmentKind
  size: number
  /** data: URL — used for preview, download, and (images) the vision payload */
  dataUrl: string
  /** extracted text for text-like files */
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
  id: string
  title: string
  model: string
  updatedAt: number
}

// ---- OpenAI-compatible message shapes ----
export type OAIContent =
  | string
  | Array<{ type: 'text'; text: string } | { type: 'image_url'; image_url: { url: string } }>
export interface OAIMessage {
  role: 'user' | 'assistant' | 'system'
  content: OAIContent
}
export interface ChatResult {
  content: string
  model: string
  usage?: unknown
}

/** The user's active API keys (raw key value is returned by the JWT keys API). */
export async function listActiveKeys(): Promise<ApiKey[]> {
  const res = await listKeys(1, 100)
  return (res.items ?? []).filter((k) => k.status === 'active' && !!k.key)
}

/** Models the given key can serve, via the gateway's /v1/models. */
export async function fetchModels(key: string): Promise<string[]> {
  const r = await fetch(buildGatewayUrl('/v1/models'), {
    headers: { Authorization: `Bearer ${key}` },
  })
  if (!r.ok) throw new Error(`models HTTP ${r.status}`)
  const j = (await r.json()) as { data?: unknown }
  if (!Array.isArray(j.data)) return []
  return j.data
    .map((m) => (m && typeof m === 'object' ? (m as { id?: unknown }).id : undefined))
    .filter((id): id is string => typeof id === 'string')
}

/** One non-streaming chat turn through the gateway. */
export async function chatCompletion(key: string, model: string, messages: OAIMessage[]): Promise<ChatResult> {
  const r = await fetch(buildGatewayUrl('/v1/chat/completions'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${key}` },
    body: JSON.stringify({ model, messages, stream: false }),
  })
  const raw = await r.text()
  let j: Record<string, unknown> = {}
  try {
    j = JSON.parse(raw) as Record<string, unknown>
  } catch {
    /* non-JSON error body */
  }
  if (!r.ok) {
    const err = (j.error as { message?: string } | undefined)?.message || (j.message as string | undefined)
    throw new Error(err || `HTTP ${r.status}: ${raw.slice(0, 200)}`)
  }
  const choices = j.choices as Array<{ message?: { content?: unknown } }> | undefined
  const c = choices?.[0]?.message?.content
  const content =
    typeof c === 'string'
      ? c
      : Array.isArray(c)
        ? c.map((p) => (p && typeof p === 'object' ? String((p as { text?: unknown }).text ?? '') : '')).join('')
        : ''
  return { content, model: (j.model as string) || model, usage: j.usage }
}
