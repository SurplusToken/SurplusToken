import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  KIMI_API_MODELS,
  KIMI_CODE_MODELS,
  OPENAI_COMPATIBLE_PROVIDER_PRESETS,
  ZHIPU_CODING_MODELS
} from '@/constants/openAICompatibleProviders'
import { getModelsByPlatform } from '@/composables/useModelWhitelist'

const modalSource = readFileSync(
  resolve(process.cwd(), 'src/components/account/CreateAccountModal.vue'),
  'utf8'
)

const badgeSource = readFileSync(
  resolve(process.cwd(), 'src/components/common/PlatformTypeBadge.vue'),
  'utf8'
)

describe('CreateAccountModal OpenAI-compatible provider presets', () => {
  it('uses the coding-plan endpoints with Chat Completions only', () => {
    expect(OPENAI_COMPATIBLE_PROVIDER_PRESETS.kimi).toMatchObject({
      baseUrl: 'https://api.moonshot.cn/v1',
      oauthBaseUrl: 'https://api.kimi.com/coding/v1',
      modelCatalogPlatform: 'kimi-api',
      oauthModelCatalogPlatform: 'kimi-code',
      responsesMode: 'force_chat_completions',
      endpointCapabilities: ['chat_completions']
    })
    expect(OPENAI_COMPATIBLE_PROVIDER_PRESETS.zhipu).toMatchObject({
      baseUrl: 'https://open.bigmodel.cn/api/coding/paas/v4',
      responsesMode: 'force_chat_completions',
      endpointCapabilities: ['chat_completions']
    })
  })

  it('exposes the current coding model catalogs', () => {
    expect(getModelsByPlatform('kimi-code')).toEqual([...KIMI_CODE_MODELS])
    expect(getModelsByPlatform('kimi-api')).toEqual([...KIMI_API_MODELS])
    expect(KIMI_CODE_MODELS).not.toEqual(expect.arrayContaining([...KIMI_API_MODELS]))
    expect(getModelsByPlatform('zhipu')).toEqual(expect.arrayContaining([...ZHIPU_CODING_MODELS]))
  })

  it('wires both provider choices and persists their display identity', () => {
    expect(modalSource).toContain('data-testid="platform-kimi"')
    expect(modalSource).toContain('data-testid="platform-zhipu"')
    expect(modalSource).toContain('extra.openai_compatible_provider = openAICompatibleProvider.value')
    expect(badgeSource).toContain("props.compatibleProvider === 'kimi'")
    expect(badgeSource).toContain("props.compatibleProvider === 'zhipu'")
  })

  it('offers both Coding Plan OAuth and API Key authentication for Kimi', () => {
    expect(modalSource).toContain('data-testid="kimi-account-type-oauth"')
    expect(modalSource).toContain('data-testid="kimi-account-type-api-key"')
    expect(modalSource).toContain('data-testid="kimi-device-authorization"')
    expect(modalSource).toContain("provider === 'kimi' && category === 'oauth-based'")
    expect(modalSource).toContain("const token = await kimiOAuth.authorize(form.proxy_id)")
    expect(modalSource).toContain('await adminAPI.kimi.createOAuthAccount')
  })
})
