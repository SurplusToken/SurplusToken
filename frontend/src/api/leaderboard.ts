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

export const leaderboardAPI = {
  getUsageLeaderboard,
}

export default leaderboardAPI
