import { apiClient } from './client'
import type {
  AccountPlatform,
  AccountUsageStatsResponse,
  ClaudeModel,
  CodexSessionImportRequest,
  CodexSessionImportResult,
  CreateScheduledTestPlanRequest,
  PaginatedResponse,
  Proxy,
  ScheduledTestPlan,
  ScheduledTestResult,
  UpdateScheduledTestPlanRequest,
  UserAccountPoolItem,
} from '@/types'

export type ContributionProbeFailurePolicy = 'continue' | 'pause' | 'local'

export interface CreateUserOAuthAccountRequest {
  name: string
  platform: AccountPlatform
  type?: 'oauth'
  credentials: Record<string, unknown>
  model_mapping?: Record<string, string>
  extra?: Record<string, unknown>
  proxy_id?: number | null
  schedulable?: boolean
  group_ids?: number[]
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  contribution_5h_reserve_percent?: number
  contribution_weekly_reserve_percent?: number
  contribution_probe_failure_policy?: ContributionProbeFailurePolicy
}

export interface UpdateUserAccountScopeRequest {
  group_ids?: number[]
  proxy_id?: number | null
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  model_mapping?: Record<string, string>
  codex_cli_only?: boolean
  contribution_5h_reserve_percent?: number
  contribution_weekly_reserve_percent?: number
  contribution_probe_failure_policy?: ContributionProbeFailurePolicy
}

export interface UserOAuthAuthUrlRequest {
  platform: AccountPlatform
  redirect_uri?: string
  project_id?: string
  oauth_type?: 'code_assist' | 'google_one' | 'ai_studio'
  tier_id?: string
  proxy_id?: number | null
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
  proxy_id?: number | null
}

export type UserOAuthTokenInfo = Record<string, unknown>

export type UserCodexSessionImportRequest = Omit<
  CodexSessionImportRequest,
  | 'notes'
  | 'concurrency'
  | 'priority'
  | 'rate_multiplier'
  | 'load_factor'
  | 'skip_default_group_bind'
  | 'confirm_mixed_channel_risk'
> & {
  schedulable?: boolean
  codex_cli_only?: boolean
  contribution_5h_reserve_percent?: number
  contribution_weekly_reserve_percent?: number
  contribution_probe_failure_policy?: ContributionProbeFailurePolicy
}

export interface ApplyUserOAuthCredentialsRequest {
  type: 'oauth'
  credentials: Record<string, unknown>
  extra?: Record<string, unknown>
}

export async function listPool(
  page = 1,
  pageSize = 50,
  filters?: {
    platform?: AccountPlatform | ''
    plan_type?: '' | 'plus' | 'pro'
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

export async function listProxies(): Promise<Proxy[]> {
  const { data } = await apiClient.get<Proxy[]>('/accounts/proxies')
  return data
}

export async function testProxy(id: number): Promise<{
  success: boolean
  message: string
  latency_ms?: number
  ip_address?: string
  city?: string
  region?: string
  country?: string
  country_code?: string
}> {
  const { data } = await apiClient.post<{
    success: boolean
    message: string
    latency_ms?: number
    ip_address?: string
    city?: string
    region?: string
    country?: string
    country_code?: string
  }>(`/accounts/proxies/${id}/test`)
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

export async function refreshOpenAIToken(
  refreshToken: string,
  proxyId?: number | null,
  clientId?: string,
): Promise<UserOAuthTokenInfo> {
  const payload: { refresh_token: string; proxy_id?: number; client_id?: string } = {
    refresh_token: refreshToken,
  }
  if (proxyId) {
    payload.proxy_id = proxyId
  }
  if (clientId) {
    payload.client_id = clientId
  }
  const { data } = await apiClient.post<UserOAuthTokenInfo>('/accounts/oauth/refresh-token', payload)
  return data
}

export async function importCodexSession(
  payload: UserCodexSessionImportRequest,
): Promise<CodexSessionImportResult> {
  const { data } = await apiClient.post<CodexSessionImportResult>(
    '/accounts/oauth/import/codex-session',
    payload,
  )
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

export async function deleteAccount(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/accounts/${id}`)
  return data
}

export async function getStats(id: number, days: number = 30): Promise<AccountUsageStatsResponse> {
  const { data } = await apiClient.get<AccountUsageStatsResponse>(`/accounts/${id}/stats`, {
    params: { days },
  })
  return data
}

export async function getAvailableModels(id: number): Promise<ClaudeModel[]> {
  const { data } = await apiClient.get<ClaudeModel[]>(`/accounts/${id}/models`)
  return data
}

export async function refreshCredentials(id: number): Promise<UserAccountPoolItem> {
  const { data } = await apiClient.post<UserAccountPoolItem>(`/accounts/${id}/refresh`)
  return data
}

export async function recoverState(id: number): Promise<UserAccountPoolItem> {
  const { data } = await apiClient.post<UserAccountPoolItem>(`/accounts/${id}/recover-state`)
  return data
}

export async function setPrivacy(id: number): Promise<UserAccountPoolItem> {
  const { data } = await apiClient.post<UserAccountPoolItem>(`/accounts/${id}/set-privacy`)
  return data
}

export async function listScheduledTestPlans(accountId: number): Promise<ScheduledTestPlan[]> {
  const { data } = await apiClient.get<ScheduledTestPlan[]>(
    `/accounts/${accountId}/scheduled-test-plans`,
  )
  return data ?? []
}

export async function createScheduledTestPlan(
  payload: CreateScheduledTestPlanRequest,
): Promise<ScheduledTestPlan> {
  const { data } = await apiClient.post<ScheduledTestPlan>('/scheduled-test-plans', payload)
  return data
}

export async function updateScheduledTestPlan(
  id: number,
  payload: UpdateScheduledTestPlanRequest,
): Promise<ScheduledTestPlan> {
  const { data } = await apiClient.put<ScheduledTestPlan>(`/scheduled-test-plans/${id}`, payload)
  return data
}

export async function deleteScheduledTestPlan(id: number): Promise<void> {
  await apiClient.delete(`/scheduled-test-plans/${id}`)
}

export async function listScheduledTestResults(
  planId: number,
  limit?: number,
): Promise<ScheduledTestResult[]> {
  const { data } = await apiClient.get<ScheduledTestResult[]>(
    `/scheduled-test-plans/${planId}/results`,
    { params: limit ? { limit } : undefined },
  )
  return data ?? []
}

export const scheduledTestsAPI = {
  listByAccount: listScheduledTestPlans,
  create: createScheduledTestPlan,
  update: updateScheduledTestPlan,
  delete: deleteScheduledTestPlan,
  listResults: listScheduledTestResults,
}

export async function applyOAuthCredentials(
  id: number,
  payload: ApplyUserOAuthCredentialsRequest,
): Promise<UserAccountPoolItem> {
  const { data } = await apiClient.post<UserAccountPoolItem>(
    `/accounts/${id}/apply-oauth-credentials`,
    payload,
  )
  return data
}

export const accountsAPI = {
  listPool,
  listProxies,
  testProxy,
  createOAuth,
  generateOAuthAuthUrl,
  exchangeOAuthCode,
  refreshOpenAIToken,
  importCodexSession,
  setSchedulable,
  updateScope,
  deleteAccount,
  getStats,
  getAvailableModels,
  refreshCredentials,
  recoverState,
  setPrivacy,
  scheduledTests: scheduledTestsAPI,
  applyOAuthCredentials,
}

export default accountsAPI
