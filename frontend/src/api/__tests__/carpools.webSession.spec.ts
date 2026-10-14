import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))

import carpoolAPI, {
  mapWebSession,
  webSession,
  type CarpoolWebSessionResponse,
} from '@/api/carpools'

function sampleResponse(): CarpoolWebSessionResponse {
  return {
    web_session: {
      gateway_origin: 'http://100.118.150.10:4310',
      ticket: 'header.payload.signature',
    },
  }
}

describe('carpool web session api', () => {
  beforeEach(() => {
    post.mockReset()
  })

  it('maps the snake_case payload to camelCase', () => {
    expect(mapWebSession(sampleResponse())).toEqual({
      gatewayOrigin: 'http://100.118.150.10:4310',
      ticket: 'header.payload.signature',
    })
  })

  it('POSTs /carpools/:id/web-session and returns the mapped session', async () => {
    post.mockResolvedValue({ data: sampleResponse() })
    const result = await webSession(7)
    expect(post).toHaveBeenCalledWith('/carpools/7/web-session')
    expect(result.gatewayOrigin).toBe('http://100.118.150.10:4310')
    expect(result.ticket).toBe('header.payload.signature')
  })

  it('exposes the function on the default export', () => {
    expect(carpoolAPI.webSession).toBe(webSession)
  })
})
