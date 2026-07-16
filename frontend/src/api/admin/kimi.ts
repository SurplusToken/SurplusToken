import { apiClient } from '../client'

export interface KimiDeviceAuthorization {
  session_id: string
  user_code: string
  verification_uri: string
  verification_uri_complete: string
  expires_in: number
  interval: number
}

export interface KimiTokenInfo {
  access_token: string
  refresh_token?: string
  token_type?: string
  scope?: string
  expires_in: number
  expires_at: number
}

export interface KimiDeviceTokenResult {
  status: 'pending' | 'success' | 'expired' | 'denied'
  error?: string
  description?: string
  token?: KimiTokenInfo
}

export interface CreateKimiOAuthAccountRequest {
  name?: string
  notes?: string | null
  token: KimiTokenInfo
  proxy_id?: number | null
  concurrency?: number
  priority?: number
  rate_multiplier?: number
  load_factor?: number
  group_ids?: number[]
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  credential_extras?: Record<string, unknown>
}

export async function startDeviceAuthorization(proxyId?: number | null): Promise<KimiDeviceAuthorization> {
  const payload: Record<string, unknown> = {}
  if (proxyId) payload.proxy_id = proxyId
  const { data } = await apiClient.post<KimiDeviceAuthorization>('/admin/kimi/oauth/device-authorization', payload)
  return data
}

export async function pollDeviceToken(sessionId: string): Promise<KimiDeviceTokenResult> {
  const { data } = await apiClient.post<KimiDeviceTokenResult>('/admin/kimi/oauth/device-token', {
    session_id: sessionId
  })
  return data
}

export async function createOAuthAccount(payload: CreateKimiOAuthAccountRequest): Promise<void> {
  await apiClient.post('/admin/kimi/oauth/create-account', payload)
}

export default { startDeviceAuthorization, pollDeviceToken, createOAuthAccount }
