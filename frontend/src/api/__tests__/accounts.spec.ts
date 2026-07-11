import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post, patch } = vi.hoisted(() => ({
  post: vi.fn(),
  patch: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get: vi.fn(),
    post,
    patch,
    delete: vi.fn(),
  },
}))

import accountsAPI from '@/api/accounts'

describe('user accounts api', () => {
  beforeEach(() => {
    post.mockReset()
    patch.mockReset()
    post.mockResolvedValue({ data: {} })
    patch.mockResolvedValue({ data: {} })
  })

  it('validates OpenAI refresh tokens through the user account pool endpoint', async () => {
    await accountsAPI.refreshOpenAIToken('rt-1', 12, 'mobile-client')

    expect(post).toHaveBeenCalledWith('/accounts/oauth/refresh-token', {
      refresh_token: 'rt-1',
      proxy_id: 12,
      client_id: 'mobile-client',
    })
  })

  it('imports Codex JSON or access tokens through the user-owned import endpoint', async () => {
    const payload = {
      content: 'access-token',
      name: 'Plus',
      group_ids: [1],
      contribution_5h_reserve_percent: 20,
    }

    await accountsAPI.importCodexSession(payload)

    expect(post).toHaveBeenCalledWith('/accounts/oauth/import/codex-session', payload)
  })

  it('sends a zero sharing rate without replacing it with the default', async () => {
    await accountsAPI.updateScope(7, { sharing_rate_multiplier: 0 })

    expect(patch).toHaveBeenCalledWith('/accounts/7/scope', {
      sharing_rate_multiplier: 0,
    })
  })

  it('requires an explicit distribution mode and sends the caller idempotency key', async () => {
    const payload = {
      mode: 'custom' as const,
      allocations: [{ user_id: 9, amount: 1.25 }],
    }

    await accountsAPI.distributeContributionPool(7, payload, 'distribution-7-attempt-1')

    expect(post).toHaveBeenCalledWith(
      '/accounts/pool/7/contribution-pool/distribute',
      payload,
      { headers: { 'Idempotency-Key': 'distribution-7-attempt-1' } },
    )
  })
})
