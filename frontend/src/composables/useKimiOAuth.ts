import { ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { KimiTokenInfo } from '@/api/admin/kimi'

const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

interface KimiOAuthAPI {
  startDeviceAuthorization: typeof adminAPI.kimi.startDeviceAuthorization
  pollDeviceToken: typeof adminAPI.kimi.pollDeviceToken
}

export function useKimiOAuth(api: KimiOAuthAPI = adminAPI.kimi) {
  const authUrl = ref('')
  const userCode = ref('')
  const sessionId = ref('')
  const loading = ref(false)
  const polling = ref(false)
  const error = ref('')
  let generation = 0
  let authPopup: Window | null = null

  const resetState = () => {
    generation++
    authPopup?.close()
    authPopup = null
    authUrl.value = ''
    userCode.value = ''
    sessionId.value = ''
    loading.value = false
    polling.value = false
    error.value = ''
  }

  const authorize = async (proxyId?: number | null): Promise<KimiTokenInfo | null> => {
    const currentGeneration = ++generation
    const popup = window.open('', '_blank')
    if (popup) popup.opener = null
    authPopup = popup
    loading.value = true
    polling.value = false
    error.value = ''
    authUrl.value = ''
    userCode.value = ''

    try {
      const authorization = await api.startDeviceAuthorization(proxyId)
      if (currentGeneration !== generation) {
        popup?.close()
        return null
      }
      sessionId.value = authorization.session_id
      userCode.value = authorization.user_code
      authUrl.value = authorization.verification_uri_complete || authorization.verification_uri
      if (popup) popup.location.href = authUrl.value
      loading.value = false
      polling.value = true

      popup?.focus()
      const intervalMs = Math.max(authorization.interval || 5, 2) * 1000
      const deadline = Date.now() + Math.max(authorization.expires_in || 900, 60) * 1000

      while (currentGeneration === generation && Date.now() < deadline) {
        await delay(intervalMs)
        if (currentGeneration !== generation) {
          popup?.close()
          return null
        }
        const result = await api.pollDeviceToken(authorization.session_id)
        if (result.status === 'pending') continue
        if (result.status === 'success' && result.token) return result.token
        error.value = result.description || (result.status === 'denied' ? 'Authorization denied' : 'Authorization expired')
        return null
      }
      if (currentGeneration === generation) error.value = 'Authorization expired'
      return null
    } catch (err: any) {
      popup?.close()
      if (currentGeneration === generation) {
        error.value = err.response?.data?.detail || err.response?.data?.message || err.message || 'Kimi authorization failed'
      }
      return null
    } finally {
      popup?.close()
      if (authPopup === popup) authPopup = null
      if (currentGeneration === generation) {
        loading.value = false
        polling.value = false
      }
    }
  }

  return { authUrl, userCode, sessionId, loading, polling, error, resetState, authorize }
}
