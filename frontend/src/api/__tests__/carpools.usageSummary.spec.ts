import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

import carpoolAPI, {
  mapUsageSummary,
  usageSummary,
  type CarpoolUsageSummaryResponse,
} from '@/api/carpools'

function sampleResponse(): CarpoolUsageSummaryResponse {
  return {
    usage_summary: {
      week_start: '2026-10-12T00:00:00Z',
      week_end: '2026-10-19T00:00:00Z',
      me: {
        week_usd: 12.34,
        declared_usd: 20.0,
        reserved_usd: 16.96,
        estimated_bill_usd: 16.96,
      },
      total: { week_usd: 45.6, member_count: 4 },
      weekly_limit_usd: 2400,
    },
  }
}

describe('carpool usage summary api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('maps the snake_case payload to camelCase', () => {
    expect(mapUsageSummary(sampleResponse())).toEqual({
      weekStart: '2026-10-12T00:00:00Z',
      weekEnd: '2026-10-19T00:00:00Z',
      me: {
        weekUsd: 12.34,
        declaredUsd: 20.0,
        reservedUsd: 16.96,
        estimatedBillUsd: 16.96,
      },
      total: { weekUsd: 45.6, memberCount: 4 },
      weeklyLimitUsd: 2400,
    })
  })

  it('maps a null me block for viewers who are not on the car', () => {
    const response = sampleResponse()
    response.usage_summary.me = null
    const mapped = mapUsageSummary(response)
    expect(mapped.me).toBeNull()
    expect(mapped.total.memberCount).toBe(4)
    expect(mapped.weekStart).toBe('2026-10-12T00:00:00Z')
  })

  it('maps null reserved/estimated bill for legacy pay-as-you-go cars', () => {
    const response = sampleResponse()
    if (!response.usage_summary.me) throw new Error('fixture should have me')
    response.usage_summary.me.declared_usd = null
    response.usage_summary.me.reserved_usd = null
    response.usage_summary.me.estimated_bill_usd = null
    const mapped = mapUsageSummary(response)
    expect(mapped.me).toEqual({
      weekUsd: 12.34,
      declaredUsd: null,
      reservedUsd: null,
      estimatedBillUsd: null,
    })
  })

  it('maps null week anchors when the car has no membership window yet', () => {
    const response = sampleResponse()
    response.usage_summary.week_start = null
    response.usage_summary.week_end = null
    const mapped = mapUsageSummary(response)
    expect(mapped.weekStart).toBeNull()
    expect(mapped.weekEnd).toBeNull()
  })

  it('GETs /carpools/:id/usage-summary and returns the mapped summary', async () => {
    get.mockResolvedValue({ data: sampleResponse() })
    const result = await usageSummary(7)
    expect(get).toHaveBeenCalledWith('/carpools/7/usage-summary')
    expect(result.me?.weekUsd).toBe(12.34)
    expect(result.me?.estimatedBillUsd).toBe(16.96)
    expect(result.total).toEqual({ weekUsd: 45.6, memberCount: 4 })
    expect(result.weeklyLimitUsd).toBe(2400)
  })

  it('exposes the function on the default export', () => {
    expect(carpoolAPI.usageSummary).toBe(usageSummary)
  })
})
