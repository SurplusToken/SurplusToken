/**
 * Usage leaderboard API endpoints
 * Provides the user-facing usage ranking for today / week / month.
 */

import { apiClient } from './client'

export type LeaderboardPeriod = 'today' | 'week' | 'month'

export interface LeaderboardEntry {
  rank: number
  user_id: number
  display_name: string
  total_tokens: number
  total_cost: number
}

export interface LeaderboardMe {
  rank: number
  total_tokens: number
  total_cost: number
}

export interface LeaderboardResponse {
  period: LeaderboardPeriod
  entries: LeaderboardEntry[]
  me: LeaderboardMe | null
}

/**
 * Get the usage leaderboard for the given period.
 * @param period - Time period ('today' | 'week' | 'month')
 * @returns Leaderboard entries plus the current user's own ranking (may be null)
 */
export async function getUsageLeaderboard(
  period: LeaderboardPeriod = 'today'
): Promise<LeaderboardResponse> {
  const { data } = await apiClient.get<LeaderboardResponse>('/usage/leaderboard', {
    params: { period }
  })
  return data
}

export interface LeaderboardTrendPoint {
  user_id: number
  display_name: string
  /** "YYYY-MM-DD" for day granularity, "YYYY-MM-DD HH:00" for hour granularity */
  date: string
  tokens: number
}

export interface LeaderboardTrendResponse {
  period: LeaderboardPeriod
  granularity: 'day' | 'hour'
  users_trend: LeaderboardTrendPoint[]
}

/**
 * Get the multi-user token-usage trend for the leaderboard (one series per user).
 * @param period - Time period ('today' | 'week' | 'month')
 * @returns Per-user trend points; display_name is already masked by the backend
 */
export async function getUsageLeaderboardTrend(
  period: LeaderboardPeriod = 'today'
): Promise<LeaderboardTrendResponse> {
  const { data } = await apiClient.get<LeaderboardTrendResponse>('/usage/leaderboard/trend', {
    params: { period }
  })
  return data
}

export const leaderboardAPI = {
  getUsageLeaderboard,
  getUsageLeaderboardTrend,
}

export default leaderboardAPI
