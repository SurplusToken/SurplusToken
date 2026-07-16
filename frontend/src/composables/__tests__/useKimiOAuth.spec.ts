import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    kimi: {
      startDeviceAuthorization: vi.fn(),
      pollDeviceToken: vi.fn()
    }
  }
}))

import { adminAPI } from '@/api/admin'
import { useKimiOAuth } from '@/composables/useKimiOAuth'

describe('useKimiOAuth', () => {
  const popup = {
    opener: window,
    location: { href: '' },
    close: vi.fn(),
    focus: vi.fn()
  }

  beforeEach(() => {
    vi.useFakeTimers()
    popup.opener = window
    popup.location.href = ''
    popup.close.mockClear()
    popup.focus.mockClear()
    vi.spyOn(window, 'open').mockReturnValue(popup as unknown as Window)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('opens the device page and polls until Kimi returns a token', async () => {
    vi.mocked(adminAPI.kimi.startDeviceAuthorization).mockResolvedValueOnce({
      session_id: 'session-1',
      user_code: 'ABCD-EFGH',
      verification_uri: 'https://auth.kimi.com/device',
      verification_uri_complete: 'https://auth.kimi.com/device?code=ABCD-EFGH',
      expires_in: 600,
      interval: 2
    })
    vi.mocked(adminAPI.kimi.pollDeviceToken)
      .mockResolvedValueOnce({ status: 'pending' })
      .mockResolvedValueOnce({
        status: 'success',
        token: {
          access_token: 'access-token',
          refresh_token: 'refresh-token',
          expires_in: 3600,
          expires_at: 1_900_000_000
        }
      })

    const oauth = useKimiOAuth()
    const resultPromise = oauth.authorize(12)
    await Promise.resolve()

    expect(window.open).toHaveBeenCalledWith('', '_blank')
    expect(popup.location.href).toBe('https://auth.kimi.com/device?code=ABCD-EFGH')
    expect(oauth.userCode.value).toBe('ABCD-EFGH')

    await vi.advanceTimersByTimeAsync(2000)
    expect(adminAPI.kimi.pollDeviceToken).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(2000)

    await expect(resultPromise).resolves.toMatchObject({ access_token: 'access-token' })
    expect(popup.close).toHaveBeenCalled()
    expect(oauth.polling.value).toBe(false)
  })
})
