import type { OpenAIEndpointCapability, OpenAIResponsesMode } from '@/types'

export type OpenAICompatibleProvider = 'openai' | 'kimi' | 'zhipu'

export const KIMI_CODE_MODELS = [
  'k3',
  'kimi-for-coding',
  'kimi-for-coding-highspeed'
] as const

export const KIMI_API_MODELS = [
  'kimi-k3',
  'kimi-k2.7-code',
  'kimi-k2.7-code-highspeed',
  'kimi-k2.6',
  'kimi-k2.5',
  'moonshot-v1-8k',
  'moonshot-v1-32k',
  'moonshot-v1-128k',
  'moonshot-v1-8k-vision-preview',
  'moonshot-v1-32k-vision-preview',
  'moonshot-v1-128k-vision-preview'
] as const

export const ZHIPU_CODING_MODELS = [
  'glm-5.1',
  'glm-5-turbo',
  'glm-4.7',
  'glm-4.5-air'
] as const

interface OpenAICompatibleProviderPreset {
  label: string
  baseUrl: string
  oauthBaseUrl?: string
  apiKeyPlaceholder: string
  modelCatalogPlatform: string
  oauthModelCatalogPlatform?: string
  responsesMode: OpenAIResponsesMode
  endpointCapabilities: OpenAIEndpointCapability[]
}

export const OPENAI_COMPATIBLE_PROVIDER_PRESETS: Record<
  OpenAICompatibleProvider,
  OpenAICompatibleProviderPreset
> = {
  openai: {
    label: 'OpenAI',
    baseUrl: 'https://api.openai.com',
    apiKeyPlaceholder: 'sk-proj-...',
    modelCatalogPlatform: 'openai',
    responsesMode: 'auto',
    endpointCapabilities: ['chat_completions', 'embeddings']
  },
  kimi: {
    label: 'Kimi',
    baseUrl: 'https://api.moonshot.cn/v1',
    oauthBaseUrl: 'https://api.kimi.com/coding/v1',
    apiKeyPlaceholder: 'sk-...',
    modelCatalogPlatform: 'kimi-api',
    oauthModelCatalogPlatform: 'kimi-code',
    responsesMode: 'force_chat_completions',
    endpointCapabilities: ['chat_completions']
  },
  zhipu: {
    label: '智谱 GLM',
    baseUrl: 'https://open.bigmodel.cn/api/coding/paas/v4',
    apiKeyPlaceholder: 'API Key',
    modelCatalogPlatform: 'zhipu',
    responsesMode: 'force_chat_completions',
    endpointCapabilities: ['chat_completions']
  }
}
