import { describe, expect, it } from 'vitest'
import { KIMI_API_MODELS, KIMI_CODE_MODELS, ZHIPU_CODING_MODELS } from '@/constants/openAICompatibleProviders'
import { getModelsByPlatform } from '@/composables/useModelWhitelist'

describe('OpenAI-compatible provider model catalogs', () => {
  it('keeps Coding Plan and pay-as-you-go model catalogs distinct', () => {
    expect(getModelsByPlatform('kimi-code')).toEqual([...KIMI_CODE_MODELS])
    expect(getModelsByPlatform('kimi-api')).toEqual([...KIMI_API_MODELS])
    expect(KIMI_CODE_MODELS).not.toEqual(expect.arrayContaining([...KIMI_API_MODELS]))
    expect(getModelsByPlatform('zhipu')).toEqual(expect.arrayContaining([...ZHIPU_CODING_MODELS]))
  })
})
