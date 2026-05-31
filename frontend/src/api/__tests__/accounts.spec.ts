import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get: vi.fn(),
    post,
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

import accountsAPI from '@/api/accounts'

describe('user accounts api', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({ data: {} })
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
})
