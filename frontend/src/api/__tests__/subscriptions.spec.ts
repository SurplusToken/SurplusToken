import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import subscriptionsAPI, { getCarpoolUsage } from '@/api/subscriptions'

describe('subscriptions api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('gets and maps current-week carpool usage snapshots', async () => {
    get.mockResolvedValue({
      data: [
        {
          subscription_id: 42,
          window_start: '2026-08-03T00:00:00Z',
          window_end: '2026-08-10T00:00:00Z',
          total_usage_usd: 1697.2,
          total_capacity_usd: 2400,
          shared_pool: {
            usage_usd: 28,
            capacity_usd: 480,
            remaining_usd: 452,
          },
          members: [
            {
              member_number: 0,
              is_current_user: true,
              declared_quota_usd: 700,
              reserved_quota_usd: 560,
              usage_usd: 588,
              shared_pool_usage_usd: 28,
            },
            {
              member_number: 1,
              is_current_user: false,
              declared_quota_usd: 500,
              reserved_quota_usd: 400,
              usage_usd: 310,
              shared_pool_usage_usd: 0,
            },
          ],
        },
      ],
    })

    await expect(getCarpoolUsage()).resolves.toEqual([
      {
        subscriptionId: 42,
        windowStart: '2026-08-03T00:00:00Z',
        windowEnd: '2026-08-10T00:00:00Z',
        totalUsageUsd: 1697.2,
        totalCapacityUsd: 2400,
        sharedPool: {
          usageUsd: 28,
          capacityUsd: 480,
          remainingUsd: 452,
        },
        members: [
          {
            memberNumber: 0,
            isCurrentUser: true,
            declaredQuotaUsd: 700,
            reservedQuotaUsd: 560,
            usageUsd: 588,
            sharedPoolUsageUsd: 28,
          },
          {
            memberNumber: 1,
            isCurrentUser: false,
            declaredQuotaUsd: 500,
            reservedQuotaUsd: 400,
            usageUsd: 310,
            sharedPoolUsageUsd: 0,
          },
        ],
      },
    ])

    expect(get).toHaveBeenCalledOnce()
    expect(get).toHaveBeenCalledWith('/subscriptions/carpool-usage')
  })

  it('preserves an empty snapshot list', async () => {
    get.mockResolvedValue({ data: [] })

    await expect(getCarpoolUsage()).resolves.toEqual([])
  })

  it('exposes carpool usage through the default api export', () => {
    expect(subscriptionsAPI.getCarpoolUsage).toBe(getCarpoolUsage)
  })
})
