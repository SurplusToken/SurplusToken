import { apiClient } from './client'
import type { AccountPlatform, PaginatedResponse, UserAccountPoolItem } from '@/types'

export interface CreateUserOAuthAccountRequest {
  name: string
  platform: AccountPlatform
  type?: 'oauth'
  credentials: Record<string, unknown>
  extra?: Record<string, unknown>
  schedulable?: boolean
  group_ids?: number[]
  expires_at?: number | null
  auto_pause_on_expired?: boolean
}

export interface UpdateUserAccountScopeRequest {
  group_ids?: number[]
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  model_mapping?: Record<string, string>
  codex_cli_only?: boolean
}

export interface UserOAuthAuthUrlRequest {
  platform: AccountPlatform
  redirect_uri?: string
  project_id?: string
  oauth_type?: 'code_assist' | 'google_one' | 'ai_studio'
  tier_id?: string
}

export interface UserOAuthAuthUrlResponse {
  auth_url: string
  session_id: string
  state?: string
}

export interface UserOAuthExchangeCodeRequest {
  platform: AccountPlatform
  session_id: string
  code: string
  state?: string
  project_id?: string
  oauth_type?: 'code_assist' | 'google_one' | 'ai_studio'
  tier_id?: string
}

export type UserOAuthTokenInfo = Record<string, unknown>

export async function listPool(
  page = 1,
  pageSize = 50,
  filters?: {
    platform?: AccountPlatform | ''
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
): Promise<PaginatedResponse<UserAccountPoolItem>> {
  const { data } = await apiClient.get<PaginatedResponse<UserAccountPoolItem>>('/accounts/pool', {
    params: {
      page,
      page_size: pageSize,
      ...filters,
    },
  })
  return data
}

export async function createOAuth(payload: CreateUserOAuthAccountRequest): Promise<UserAccountPoolItem> {
  const { data } = await apiClient.post<UserAccountPoolItem>('/accounts/oauth', payload)
  return data
}

export async function generateOAuthAuthUrl(
  payload: UserOAuthAuthUrlRequest
): Promise<UserOAuthAuthUrlResponse> {
  const { data } = await apiClient.post<UserOAuthAuthUrlResponse>('/accounts/oauth/auth-url', payload)
  return data
}

export async function exchangeOAuthCode(
  payload: UserOAuthExchangeCodeRequest
): Promise<UserOAuthTokenInfo> {
  const { data } = await apiClient.post<UserOAuthTokenInfo>('/accounts/oauth/exchange-code', payload)
  return data
}

export async function setSchedulable(id: number, schedulable: boolean): Promise<UserAccountPoolItem> {
  const { data } = await apiClient.patch<UserAccountPoolItem>(`/accounts/${id}/schedulable`, {
    schedulable,
  })
  return data
}

export async function updateScope(
  id: number,
  payload: UpdateUserAccountScopeRequest,
): Promise<UserAccountPoolItem> {
  const { data } = await apiClient.patch<UserAccountPoolItem>(`/accounts/${id}/scope`, payload)
  return data
}

export const accountsAPI = {
  listPool,
  createOAuth,
  generateOAuthAuthUrl,
  exchangeOAuthCode,
  setSchedulable,
  updateScope,
}

export default accountsAPI
