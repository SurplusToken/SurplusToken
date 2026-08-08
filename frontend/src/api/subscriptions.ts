/**
 * User Subscription API
 * API for regular users to view their own subscriptions and progress
 */

import { apiClient } from './client'
import type {
  CarpoolUsageSnapshot,
  SubscriptionProgress,
  UserSubscription,
} from '@/types'

interface CarpoolUsageMemberResponse {
  member_number: number
  is_current_user: boolean
  declared_quota_usd: number
  reserved_quota_usd: number
  usage_usd: number
  shared_pool_usage_usd: number
}

interface CarpoolUsageSnapshotResponse {
  subscription_id: number
  window_start: string
  window_end: string
  total_usage_usd: number
  total_capacity_usd: number
  shared_pool: {
    usage_usd: number
    capacity_usd: number
    remaining_usd: number
  }
  members: CarpoolUsageMemberResponse[]
}

/**
 * Subscription summary for user dashboard
 */
export interface SubscriptionSummary {
  active_count: number
  subscriptions: Array<{
    id: number
    group_name: string
    status: string
    daily_progress: number | null
    weekly_progress: number | null
    monthly_progress: number | null
    expires_at: string | null
    days_remaining: number | null
  }>
}

/**
 * Get list of current user's subscriptions
 */
export async function getMySubscriptions(): Promise<UserSubscription[]> {
  const response = await apiClient.get<UserSubscription[]>('/subscriptions')
  return response.data
}

/**
 * Get current user's active subscriptions
 */
export async function getActiveSubscriptions(): Promise<UserSubscription[]> {
  const response = await apiClient.get<UserSubscription[]>('/subscriptions/active')
  return response.data
}

/**
 * Get progress for all user's active subscriptions
 */
export async function getSubscriptionsProgress(): Promise<SubscriptionProgress[]> {
  const response = await apiClient.get<SubscriptionProgress[]>('/subscriptions/progress')
  return response.data
}

/**
 * Get subscription summary for dashboard display
 */
export async function getSubscriptionSummary(): Promise<SubscriptionSummary> {
  const response = await apiClient.get<SubscriptionSummary>('/subscriptions/summary')
  return response.data
}

/**
 * Get progress for a specific subscription
 */
export async function getSubscriptionProgress(
  subscriptionId: number
): Promise<SubscriptionProgress> {
  const response = await apiClient.get<SubscriptionProgress>(
    `/subscriptions/${subscriptionId}/progress`
  )
  return response.data
}

/**
 * Get current-week usage for the current user's carpool subscriptions
 */
export async function getCarpoolUsage(): Promise<CarpoolUsageSnapshot[]> {
  const response = await apiClient.get<CarpoolUsageSnapshotResponse[]>(
    '/subscriptions/carpool-usage'
  )

  return response.data.map((snapshot) => ({
    subscriptionId: snapshot.subscription_id,
    windowStart: snapshot.window_start,
    windowEnd: snapshot.window_end,
    totalUsageUsd: snapshot.total_usage_usd,
    totalCapacityUsd: snapshot.total_capacity_usd,
    sharedPool: {
      usageUsd: snapshot.shared_pool.usage_usd,
      capacityUsd: snapshot.shared_pool.capacity_usd,
      remainingUsd: snapshot.shared_pool.remaining_usd,
    },
    members: snapshot.members.map((member) => ({
      memberNumber: member.member_number,
      isCurrentUser: member.is_current_user,
      declaredQuotaUsd: member.declared_quota_usd,
      reservedQuotaUsd: member.reserved_quota_usd,
      usageUsd: member.usage_usd,
      sharedPoolUsageUsd: member.shared_pool_usage_usd,
    })),
  }))
}

export default {
  getMySubscriptions,
  getActiveSubscriptions,
  getSubscriptionsProgress,
  getSubscriptionSummary,
  getSubscriptionProgress,
  getCarpoolUsage
}
