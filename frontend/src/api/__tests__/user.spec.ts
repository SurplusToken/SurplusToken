import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, put, post: vi.fn(), delete: vi.fn() },
}))

describe('user api oauth binding urls', () => {
  beforeEach(() => {
    vi.resetModules()
    get.mockReset()
    put.mockReset()
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example.com/api/v1')
  })

  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('builds third-party bind urls against the bind start endpoint', async () => {
    const { buildOAuthBindingStartURL } = await import('@/api/user')

    expect(buildOAuthBindingStartURL('linuxdo', { redirectTo: '/settings/profile' })).toBe(
      'https://api.example.com/api/v1/auth/oauth/linuxdo/bind/start?redirect=%2Fsettings%2Fprofile&intent=bind_current_user'
    )
    expect(
      buildOAuthBindingStartURL('wechat', {
        redirectTo: '/settings/profile',
        wechatOAuthSettings: {
          wechat_oauth_open_enabled: true,
          wechat_oauth_mp_enabled: false,
          wechat_oauth_mobile_enabled: false
        }
      })
    ).toBe(
      'https://api.example.com/api/v1/auth/oauth/wechat/bind/start?redirect=%2Fsettings%2Fprofile&intent=bind_current_user&mode=open'
    )
  })

  it('gets and fully replaces the accepted sharing-rate range', async () => {
    get.mockResolvedValue({ data: { min: 0, max: 1.5 } })
    put.mockResolvedValue({ data: { min: null, max: 2 } })
    const { getSharingRateRange, updateSharingRateRange } = await import('@/api/user')

    await expect(getSharingRateRange()).resolves.toEqual({ min: 0, max: 1.5 })
    await expect(updateSharingRateRange({ min: null, max: 2 })).resolves.toEqual({
      min: null,
      max: 2,
    })

    expect(get).toHaveBeenCalledWith('/user/sharing-rate-range')
    expect(put).toHaveBeenCalledWith('/user/sharing-rate-range', { min: null, max: 2 })
  })

  it('keeps both nullable keys in a fully unbounded update', async () => {
    put.mockResolvedValue({ data: { min: null, max: null } })
    const { updateSharingRateRange } = await import('@/api/user')

    await updateSharingRateRange({ min: null, max: null })

    expect(put).toHaveBeenCalledWith('/user/sharing-rate-range', {
      min: null,
      max: null,
    })
  })
})
