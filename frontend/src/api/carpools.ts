import { apiClient } from './client'

export type CarpoolStatus = 'recruiting' | 'confirmed' | 'starting' | 'active' | 'cancelled' | 'ended'
export type CarpoolVisibility = 'public' | 'invite_only'
export type CarpoolRole = 'owner' | 'member'
export type CarpoolType = 'small' | 'large'
export type CarpoolRecommendationBasis = 'usage_history' | 'anchor'

interface CarpoolResponse {
  id: number
  name: string
  description: string
  organizer: string
  owner_user_id?: number
  platform: string
  plan_type: string
  car_type: CarpoolType
  level: number
  capacity: number
  member_count: number
  base_fee_cny: number
  usage_pool_cny_per_account: number
  visibility: CarpoolVisibility
  status: CarpoolStatus
  join_locked: boolean
  scheduled_start_at?: string
  launched_at?: string
  group_id?: number
  group_name?: string
  member_role: CarpoolRole | null
  created_at: string
  // 两段确认发车与入群信息
  admin_wechat: string
  has_group_qr_code: boolean
  launch_notified_at?: string
  confirmed_at?: string
  // 自定义规则车（含平台升级前建立的老车）：不走申报制，按 rule_note 人工结算
  pricing_model?: string
  rule_note?: string
  // 额度预约制参数（设计文档 §3）
  weekly_limit_usd: number
  seat_fee_cny: number
  usage_pool_cny: number
  reserve_ratio: number
  launch_min_ratio: number
  launch_max_ratio: number
  // 车外展示指标（设计文档 §4.6）
  declared_total_usd: number
  remaining_joinable_usd: number
  plus_equivalents: number
  avg_price_cny: number
}

export interface Carpool {
  id: number
  name: string
  description: string
  organizer: string
  ownerUserId?: number
  platform: string
  planType: string
  carType: CarpoolType
  level: number
  capacity: number
  memberCount: number
  baseFeeCny: number
  usagePoolCnyPerAccount: number
  visibility: CarpoolVisibility
  status: CarpoolStatus
  joinLocked: boolean
  scheduledStartAt: string
  launchedAt?: string
  groupId?: number
  groupName: string | null
  memberRole: CarpoolRole | null
  createdAt: string
  adminWechat: string
  hasGroupQrCode: boolean
  launchNotifiedAt?: string
  confirmedAt?: string
  pricingModel: string
  ruleNote: string
  weeklyLimitUsd: number
  seatFeeCny: number
  usagePoolCny: number
  reserveRatio: number
  launchMinRatio: number
  launchMaxRatio: number
  declaredTotalUsd: number
  remainingJoinableUsd: number
  plusEquivalents: number
  avgPriceCny: number
}

export interface CreateCarpoolRequest {
  name: string
  description: string
  visibility: CarpoolVisibility
  scheduled_start_at: string
  // 两项强制确认：已添加管理员微信（必须为 true）+ 群二维码（base64/data URL，≤2MB png/jpeg/webp）
  added_admin_wechat: boolean
  group_qr_code: string
  // 以下额度池/价格参数仅 admin 可传，普通用户传任何一个都会被后端 403
  // （CARPOOL_CUSTOM_PARAMS_FORBIDDEN）；缺省时使用默认值 2400/400/1000/0.8/0.95/1.05。
  weekly_limit_usd?: number
  seat_fee_cny?: number
  usage_pool_cny?: number
  reserve_ratio?: number
  launch_min_ratio?: number
  launch_max_ratio?: number
  // owner 本人的申报（可选，0 = 仅发起不占额度）
  declared_weekly_quota_usd?: number
}

export interface JoinCarpoolResult {
  carpool: Carpool
  prepaidAmountCny: number
}

export interface DeclarationRecommendation {
  recommendedWeeklyQuotaUsd: number
  rawWeeklyUsageUsd: number
  bufferRatio: number
  daysWithRecords: number
  basis: CarpoolRecommendationBasis
  message: string
}

export interface SettlementMember {
  userId: number
  // 仅 owner/admin 的全量视图会带上，用于把结算行对应到微信群里的真人收款
  email?: string
  username?: string
  role: CarpoolRole
  declaredWeeklyQuotaUsd: number
  floorUsageUsd: number
  actualUsageUsd: number
  billableUsageUsd: number
  floorTriggered: boolean
  prepaidAmountCny: number
  quotedPrepaidCny: number
  usagePrepaidCny: number
  usageFinalShareCny: number
  usageDeltaCny: number
  seatFeePrepaidCny: number
  seatFeeFinalCny: number
  seatFeeDeltaCny: number
  totalDeltaCny: number
}

export interface CarpoolSettlement {
  carpoolId: number
  status: CarpoolStatus
  weeklyLimitUsd: number
  seatFeeCny: number
  usagePoolCny: number
  reserveRatio: number
  memberCount: number
  fullView: boolean
  periodStart?: string
  periodEnd?: string
  members: SettlementMember[]
  // settled=true 时 members 是结算时冻结的快照，不再随用量变化；
  // false 时是实时预览。settleBlockedFor 给出不能结算的原因。
  settled: boolean
  settledAt?: string
  settledByUserId?: number
  canSettle: boolean
  settleBlockedFor: string
  // 自定义规则车：members 只带实际用量，金额字段全为零（不适用，而非算出来是 0）
  manualSettlement: boolean
  pricingModel: string
  ruleNote: string
}

interface CarpoolMutationResponse {
  carpool: CarpoolResponse
  invite_token?: string
  declared_weekly_quota_usd?: number
  prepaid_amount_cny?: number
}

interface DeclarationRecommendationResponse {
  recommended_weekly_quota_usd: number
  raw_weekly_usage_usd: number
  buffer_ratio: number
  days_with_records: number
  basis: CarpoolRecommendationBasis
  message: string
}

interface SettlementMemberResponse {
  user_id: number
  email?: string
  username?: string
  role: CarpoolRole
  declared_weekly_quota_usd: number
  floor_usage_usd: number
  actual_usage_usd: number
  billable_usage_usd: number
  floor_triggered: boolean
  prepaid_amount_cny: number
  quoted_prepaid_cny: number
  usage_prepaid_cny: number
  usage_final_share_cny: number
  usage_delta_cny: number
  seat_fee_prepaid_cny: number
  seat_fee_final_cny: number
  seat_fee_delta_cny: number
  total_delta_cny: number
}

interface CarpoolSettlementResponse {
  carpool_id: number
  status: CarpoolStatus
  weekly_limit_usd: number
  seat_fee_cny: number
  usage_pool_cny: number
  reserve_ratio: number
  member_count: number
  full_view: boolean
  period_start?: string
  period_end?: string
  members: SettlementMemberResponse[]
  settled?: boolean
  settled_at?: string
  settled_by_user_id?: number
  can_settle?: boolean
  settle_blocked_for?: string
  manual_settlement?: boolean
  pricing_model?: string
  rule_note?: string
}

function mapCarpool(item: CarpoolResponse): Carpool {
  return {
    id: item.id,
    name: item.name,
    description: item.description,
    organizer: item.organizer,
    ownerUserId: item.owner_user_id,
    platform: item.platform,
    planType: item.plan_type,
    carType: item.car_type,
    level: item.level,
    capacity: item.capacity,
    memberCount: item.member_count,
    baseFeeCny: item.base_fee_cny,
    usagePoolCnyPerAccount: item.usage_pool_cny_per_account,
    visibility: item.visibility,
    status: item.status,
    joinLocked: item.join_locked,
    scheduledStartAt: item.scheduled_start_at?.slice(0, 10) || '',
    launchedAt: item.launched_at,
    groupId: item.group_id,
    groupName: item.group_name || null,
    memberRole: item.member_role,
    createdAt: item.created_at,
    adminWechat: item.admin_wechat || '',
    hasGroupQrCode: !!item.has_group_qr_code,
    launchNotifiedAt: item.launch_notified_at,
    confirmedAt: item.confirmed_at,
    pricingModel: item.pricing_model || 'quota',
    ruleNote: item.rule_note || '',
    weeklyLimitUsd: item.weekly_limit_usd,
    seatFeeCny: item.seat_fee_cny,
    usagePoolCny: item.usage_pool_cny,
    reserveRatio: item.reserve_ratio,
    launchMinRatio: item.launch_min_ratio,
    launchMaxRatio: item.launch_max_ratio,
    declaredTotalUsd: item.declared_total_usd,
    remainingJoinableUsd: item.remaining_joinable_usd,
    plusEquivalents: item.plus_equivalents,
    avgPriceCny: item.avg_price_cny,
  }
}

function mapSettlement(data: CarpoolSettlementResponse): CarpoolSettlement {
  return {
    carpoolId: data.carpool_id,
    status: data.status,
    weeklyLimitUsd: data.weekly_limit_usd,
    seatFeeCny: data.seat_fee_cny,
    usagePoolCny: data.usage_pool_cny,
    reserveRatio: data.reserve_ratio,
    memberCount: data.member_count,
    fullView: data.full_view,
    periodStart: data.period_start,
    periodEnd: data.period_end,
    settled: !!data.settled,
    settledAt: data.settled_at,
    settledByUserId: data.settled_by_user_id,
    canSettle: !!data.can_settle,
    settleBlockedFor: data.settle_blocked_for || '',
    manualSettlement: !!data.manual_settlement,
    pricingModel: data.pricing_model || 'quota',
    ruleNote: data.rule_note || '',
    members: (data.members || []).map((member) => ({
      userId: member.user_id,
      email: member.email,
      username: member.username,
      role: member.role,
      declaredWeeklyQuotaUsd: member.declared_weekly_quota_usd,
      floorUsageUsd: member.floor_usage_usd,
      actualUsageUsd: member.actual_usage_usd,
      billableUsageUsd: member.billable_usage_usd,
      floorTriggered: member.floor_triggered,
      prepaidAmountCny: member.prepaid_amount_cny,
      quotedPrepaidCny: member.quoted_prepaid_cny ?? member.prepaid_amount_cny,
      usagePrepaidCny: member.usage_prepaid_cny,
      usageFinalShareCny: member.usage_final_share_cny,
      usageDeltaCny: member.usage_delta_cny,
      seatFeePrepaidCny: member.seat_fee_prepaid_cny,
      seatFeeFinalCny: member.seat_fee_final_cny,
      seatFeeDeltaCny: member.seat_fee_delta_cny,
      totalDeltaCny: member.total_delta_cny,
    })),
  }
}

export async function list(): Promise<Carpool[]> {
  const { data } = await apiClient.get<CarpoolResponse[]>('/carpools')
  return data.map(mapCarpool)
}

export async function create(payload: CreateCarpoolRequest): Promise<{ carpool: Carpool; inviteToken: string }> {
  const { data } = await apiClient.post<CarpoolMutationResponse>('/carpools', payload)
  return { carpool: mapCarpool(data.carpool), inviteToken: data.invite_token || '' }
}

export async function resolveInvite(token: string): Promise<Carpool> {
  const { data } = await apiClient.get<CarpoolResponse>(`/carpools/invites/${encodeURIComponent(token)}`)
  return mapCarpool(data)
}

export async function createInvite(id: number): Promise<string> {
  const { data } = await apiClient.post<{ token: string }>(`/carpools/${id}/invites`)
  return data.token
}

export async function join(id: number, declaredWeeklyQuotaUsd: number): Promise<JoinCarpoolResult> {
  const { data } = await apiClient.post<CarpoolMutationResponse>(`/carpools/${id}/join`, {
    declared_weekly_quota_usd: declaredWeeklyQuotaUsd,
    joined_wechat_group: true,
  })
  return { carpool: mapCarpool(data.carpool), prepaidAmountCny: data.prepaid_amount_cny || 0 }
}

export async function joinByInvite(token: string, declaredWeeklyQuotaUsd: number): Promise<JoinCarpoolResult> {
  const { data } = await apiClient.post<CarpoolMutationResponse>('/carpools/join-by-invite', {
    token,
    declared_weekly_quota_usd: declaredWeeklyQuotaUsd,
    joined_wechat_group: true,
  })
  return { carpool: mapCarpool(data.carpool), prepaidAmountCny: data.prepaid_amount_cny || 0 }
}

export async function leave(id: number): Promise<Carpool> {
  const { data } = await apiClient.post<CarpoolMutationResponse>(`/carpools/${id}/leave`)
  return mapCarpool(data.carpool)
}

export async function confirm(id: number): Promise<Carpool> {
  const { data } = await apiClient.post<CarpoolMutationResponse>(`/carpools/${id}/confirm`)
  return mapCarpool(data.carpool)
}

// 群二维码。invite_only 的车在上车前必须带上邀请 token，否则后端 403
// （二维码 = 入场券，不能对任意登录用户开放）。
export async function groupQrCode(id: number, inviteToken?: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/carpools/${id}/qr-code`, {
    responseType: 'blob',
    params: inviteToken ? { token: inviteToken } : undefined,
  })
  return data
}

// 撤回确认（confirmed → recruiting）：车主或 admin，给"等管理员启动"这段状态一个出口。
export async function unconfirm(id: number): Promise<Carpool> {
  const { data } = await apiClient.post<CarpoolMutationResponse>(`/carpools/${id}/unconfirm`)
  return mapCarpool(data.carpool)
}

export interface PendingLaunchCarpool {
  carpoolId: number
  name: string
  ownerUserId?: number
  ownerEmail: string
  memberCount: number
  declaredTotalUsd: number
  weeklyLimitUsd: number
  confirmedAt: string
  pendingHours: number
  overdue: boolean
}

interface PendingLaunchResponse {
  carpool_id: number
  name: string
  owner_user_id?: number
  owner_email?: string
  member_count: number
  declared_total_usd: number
  weekly_limit_usd: number
  confirmed_at: string
  pending_hours: number
  overdue: boolean
}

// 待启动列表（仅 admin）：车主已确认、等管理员启动的车，含等待时长与超时标记。
export async function pendingLaunch(): Promise<PendingLaunchCarpool[]> {
  const { data } = await apiClient.get<PendingLaunchResponse[]>('/carpools/pending-launch')
  return (data || []).map((item) => ({
    carpoolId: item.carpool_id,
    name: item.name,
    ownerUserId: item.owner_user_id,
    ownerEmail: item.owner_email || '',
    memberCount: item.member_count,
    declaredTotalUsd: item.declared_total_usd,
    weeklyLimitUsd: item.weekly_limit_usd,
    confirmedAt: item.confirmed_at,
    pendingHours: item.pending_hours,
    overdue: item.overdue,
  }))
}

export async function launch(id: number, force = false): Promise<Carpool> {
  const { data } = await apiClient.post<CarpoolMutationResponse>(`/carpools/${id}/launch`, { force })
  return mapCarpool(data.carpool)
}

export async function declarationRecommendation(): Promise<DeclarationRecommendation> {
  const { data } = await apiClient.get<DeclarationRecommendationResponse>('/carpools/declaration-recommendation')
  return {
    recommendedWeeklyQuotaUsd: data.recommended_weekly_quota_usd,
    rawWeeklyUsageUsd: data.raw_weekly_usage_usd,
    bufferRatio: data.buffer_ratio,
    daysWithRecords: data.days_with_records,
    basis: data.basis,
    message: data.message,
  }
}

// 自定义规则咨询入口：通知全部 admin（邮件），不创建车辆；后端在邮件不可用时优雅降级。
export async function notifyCustomRuleInterest(note?: string): Promise<void> {
  await apiClient.post('/carpools/custom-rule-interest', note ? { note } : {})
}

export async function settlement(id: number): Promise<CarpoolSettlement> {
  const { data } = await apiClient.get<CarpoolSettlementResponse>(`/carpools/${id}/settlement`)
  return mapSettlement(data)
}

// 冻结结算单（车主或 admin）：把当下的金额写死，之后读到的就是这份快照。
export async function settle(id: number): Promise<CarpoolSettlement> {
  const { data } = await apiClient.post<CarpoolSettlementResponse>(`/carpools/${id}/settlement/settle`)
  return mapSettlement(data)
}

// 撤销结算（仅 admin）：结算算错了的受控改回路径。
export async function unsettle(id: number): Promise<void> {
  await apiClient.post(`/carpools/${id}/settlement/unsettle`)
}

export async function cancel(id: number): Promise<void> {
  await apiClient.post(`/carpools/${id}/cancel`)
}

export async function setJoinLocked(id: number, locked: boolean): Promise<void> {
  await apiClient.patch(`/carpools/${id}/join-lock`, { locked })
}

export default {
  list,
  create,
  resolveInvite,
  createInvite,
  join,
  joinByInvite,
  leave,
  confirm,
  unconfirm,
  pendingLaunch,
  groupQrCode,
  launch,
  declarationRecommendation,
  notifyCustomRuleInterest,
  settlement,
  settle,
  unsettle,
  cancel,
  setJoinLocked,
}
