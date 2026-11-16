import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post } }))

import carpoolAPI, {
  mapUpstreamQuota,
  refreshUpstreamQuota,
  upstreamQuota,
  type UpstreamQuotaResponse,
} from '@/api/carpools'

function sampleResponse(): UpstreamQuotaResponse {
  return {
    upstream_quota: {
      configured: true,
      carpool_id: 7,
      cpr_account_id: 'acct_abc123',
      generated_at: '2026-10-12T08:30:00Z',
      cached: false,
      weekly: {
        available: true,
        unavailable_reason: null,
        low_sample: true,
        used_percent: 7,
        used_percent_display: '7.0%',
        observed_at: '2026-10-12T08:00:00Z',
        reset_at: '2026-10-14T00:00:00Z',
        tokens_display: '116.6M',
        usd_display: '$163.06',
        estimated_tokens: 1536625298,
        estimated_tokens_display: '1.5B',
        estimated_usd: 2282.92,
        estimated_usd_display: '$2282.92',
      },
      reset_credits: { available_count: 3, next_expires_at: '2026-10-15T10:00:00Z' },
      account_expiry: { expires_at: '2026-10-20T00:00:00Z', will_renew: true },
      refresh: { cooldown_seconds: 60 },
    },
  }
}

describe('carpool upstream quota api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('maps the snake_case payload to camelCase', () => {
    expect(mapUpstreamQuota(sampleResponse())).toEqual({
      configured: true,
      carpoolId: 7,
      cprAccountId: 'acct_abc123',
      generatedAt: '2026-10-12T08:30:00Z',
      cached: false,
      weekly: {
        available: true,
        unavailableReason: null,
        lowSample: true,
        usedPercent: 7,
        usedPercentDisplay: '7.0%',
        observedAt: '2026-10-12T08:00:00Z',
        resetAt: '2026-10-14T00:00:00Z',
        tokensDisplay: '116.6M',
        usdDisplay: '$163.06',
        estimatedTokens: 1536625298,
        estimatedTokensDisplay: '1.5B',
        estimatedUsd: 2282.92,
        estimatedUsdDisplay: '$2282.92',
      },
      resetCredits: { availableCount: 3, nextExpiresAt: '2026-10-15T10:00:00Z' },
      accountExpiry: { expiresAt: '2026-10-20T00:00:00Z', willRenew: true },
      refreshCooldownSeconds: 60,
    })
  })

  it('maps a null account_expiry block when the upstream call failed or no subscription is on file', () => {
    const response = sampleResponse()
    response.upstream_quota.account_expiry = null
    const mapped = mapUpstreamQuota(response)
    expect(mapped.accountExpiry).toBeNull()
    expect(mapped.weekly?.usedPercent).toBe(7)
  })

  it('maps an account_expiry block with an unknown renewal state', () => {
    const response = sampleResponse()
    if (!response.upstream_quota.account_expiry) throw new Error('fixture should have account_expiry')
    response.upstream_quota.account_expiry.will_renew = null
    const mapped = mapUpstreamQuota(response)
    expect(mapped.accountExpiry).toEqual({ expiresAt: '2026-10-20T00:00:00Z', willRenew: null })
  })

  it('maps a null reset_credits block when the upstream call failed', () => {
    const response = sampleResponse()
    response.upstream_quota.reset_credits = null
    const mapped = mapUpstreamQuota(response)
    expect(mapped.resetCredits).toBeNull()
    expect(mapped.weekly?.usedPercent).toBe(7)
  })

  it('maps a null weekly block when cpr has no usage data yet', () => {
    const response = sampleResponse()
    response.upstream_quota.weekly = null
    const mapped = mapUpstreamQuota(response)
    expect(mapped.weekly).toBeNull()
    expect(mapped.configured).toBe(true)
  })

  it('maps an unavailable weekly block and keeps the reason', () => {
    const response = sampleResponse()
    const weekly = response.upstream_quota.weekly
    if (!weekly) throw new Error('fixture should have a weekly block')
    weekly.available = false
    weekly.unavailable_reason = 'usage data not ready'
    const mapped = mapUpstreamQuota(response)
    expect(mapped.weekly?.available).toBe(false)
    expect(mapped.weekly?.unavailableReason).toBe('usage data not ready')
  })

  it('GETs /carpools/:id/upstream-quota and returns the mapped quota', async () => {
    get.mockResolvedValue({ data: sampleResponse() })
    const result = await upstreamQuota(7)
    expect(get).toHaveBeenCalledWith('/carpools/7/upstream-quota')
    expect(result.carpoolId).toBe(7)
    expect(result.weekly?.tokensDisplay).toBe('116.6M')
    expect(result.weekly?.estimatedUsdDisplay).toBe('$2282.92')
    expect(result.refreshCooldownSeconds).toBe(60)
  })

  it('POSTs /carpools/:id/upstream-quota/refresh and returns the mapped quota', async () => {
    post.mockResolvedValue({ data: sampleResponse() })
    const result = await refreshUpstreamQuota(7)
    expect(post).toHaveBeenCalledWith('/carpools/7/upstream-quota/refresh')
    expect(result.carpoolId).toBe(7)
    expect(result.weekly?.usedPercent).toBe(7)
  })

  it('exposes both functions on the default export', () => {
    expect(carpoolAPI.upstreamQuota).toBe(upstreamQuota)
    expect(carpoolAPI.refreshUpstreamQuota).toBe(refreshUpstreamQuota)
  })
})
