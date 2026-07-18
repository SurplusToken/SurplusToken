import { apiClient } from './client'

export type CarpoolStatus = 'recruiting' | 'starting' | 'active' | 'cancelled' | 'ended'
export type CarpoolVisibility = 'public' | 'invite_only'
export type CarpoolRole = 'owner' | 'member'
export type CarpoolType = 'small' | 'large'

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
}

export interface CreateCarpoolRequest {
  name: string
  description: string
  car_type: CarpoolType
  level: number
  visibility: CarpoolVisibility
  scheduled_start_at: string
}

interface CarpoolMutationResponse {
  carpool: CarpoolResponse
  invite_token?: string
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

export async function join(id: number): Promise<Carpool> {
  const { data } = await apiClient.post<CarpoolMutationResponse>(`/carpools/${id}/join`)
  return mapCarpool(data.carpool)
}

export async function joinByInvite(token: string): Promise<Carpool> {
  const { data } = await apiClient.post<CarpoolMutationResponse>('/carpools/join-by-invite', { token })
  return mapCarpool(data.carpool)
}

export async function cancel(id: number): Promise<void> {
  await apiClient.post(`/carpools/${id}/cancel`)
}

export async function setJoinLocked(id: number, locked: boolean): Promise<void> {
  await apiClient.patch(`/carpools/${id}/join-lock`, { locked })
}

export default { list, create, resolveInvite, createInvite, join, joinByInvite, cancel, setJoinLocked }
