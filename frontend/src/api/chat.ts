/**
 * Chat (playground) API types + client.
 *
 * NOTE: the ChatView currently runs in DEMO mode (see stores/chat.ts) so it can
 * be previewed without any backend. The endpoints below are the intended wiring
 * for when the `chatgpt-web` channel + dashboard chat endpoint land:
 *   - normal models  -> real API channels (billed by real token usage)
 *   - web-only models -> ChatGPT-web bridge (billed by estimate / per-message)
 * Routing + billing are decided server-side; the frontend just sends {model, content}.
 */

import { apiClient } from './client'

export type ChatRoute = 'api' | 'web'

export interface ChatModel {
  /** value sent to the backend, e.g. "gpt-5" or "web:o3" */
  id: string
  /** display name, e.g. "o3" */
  label: string
  /** which upstream this model is served by (drives the badge + billing) */
  route: ChatRoute
  /** optgroup label */
  group: string
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  /** model label for assistant turns */
  model?: string
  /** true while the assistant reply is still streaming in */
  pending?: boolean
  createdAt: number
}

export interface Conversation {
  id: string
  title: string
  /** a conversation is locked to one route so context never mixes API <-> web */
  route: ChatRoute
  /** currently selected model id */
  model: string
  updatedAt: number
}

export interface SendMessagePayload {
  conversationId: string
  content: string
  model: string
}

export interface SendMessageResult {
  text: string
  model: string
}

export const chatAPI = {
  async getModels(): Promise<ChatModel[]> {
    return (await apiClient.get<ChatModel[]>('/chat/models')).data
  },
  async listConversations(): Promise<Conversation[]> {
    return (await apiClient.get<Conversation[]>('/chat/conversations')).data
  },
  async sendMessage(payload: SendMessagePayload): Promise<SendMessageResult> {
    return (await apiClient.post<SendMessageResult>('/chat/completions', payload)).data
  },
}
