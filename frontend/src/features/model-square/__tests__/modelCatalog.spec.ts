import { describe, expect, it } from 'vitest'
import type {
  UserAvailableChannel,
  UserModelPerformance,
  UserSupportedModelPricing
} from '@/api/channels'
import { buildModelCatalog, minimumPricingValue, modelStartingPrice } from '../modelCatalog'

function tokenPricing(inputPrice: number): UserSupportedModelPricing {
  return {
    billing_mode: 'token',
    input_price: inputPrice,
    output_price: inputPrice * 2,
    cache_write_price: null,
    cache_read_price: null,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: []
  }
}

function performance(groupID: number, successCount: number, failureCount: number): UserModelPerformance {
  const sampleCount = successCount + failureCount
  return {
    window_hours: 24,
    success_rate: (successCount / sampleCount) * 100,
    success_count: successCount,
    failure_count: failureCount,
    sample_count: sampleCount,
    latency_sample_count: successCount,
    ttft_sample_count: successCount,
    avg_latency_ms: 800,
    avg_ttft_ms: 200,
    groups: [
      {
        group_id: groupID,
        success_rate: (successCount / sampleCount) * 100,
        success_count: successCount,
        failure_count: failureCount,
        sample_count: sampleCount,
        latency_sample_count: successCount,
        ttft_sample_count: successCount,
        avg_latency_ms: 800,
        avg_ttft_ms: 200
      }
    ]
  }
}

describe('modelCatalog', () => {
  it('aggregates the same platform model and keeps its callable routes', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'Primary',
        description: 'fast route',
        platforms: [
          {
            platform: 'openai',
            groups: [
              {
                id: 1,
                name: 'default',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 1,
                peak_rate_enabled: false,
                peak_start: '',
                peak_end: '',
                peak_rate_multiplier: 1,
                is_exclusive: false
              }
            ],
            supported_models: [{ name: 'gpt-5', platform: 'openai', pricing: tokenPricing(0.00001) }]
          }
        ]
      },
      {
        name: 'Economy',
        description: 'lower price',
        platforms: [
          {
            platform: 'openai',
            groups: [
              {
                id: 2,
                name: 'pro',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 0.8,
                peak_rate_enabled: false,
                peak_start: '',
                peak_end: '',
                peak_rate_multiplier: 1,
                is_exclusive: true
              }
            ],
            supported_models: [{ name: 'gpt-5', platform: 'openai', pricing: tokenPricing(0.000005) }]
          }
        ]
      }
    ]

    const [model] = buildModelCatalog(channels)

    expect(model.name).toBe('gpt-5')
    expect(model.routes).toHaveLength(2)
    expect(model.groups.map((group) => group.name)).toEqual(['default', 'pro'])
    expect(model.bestRoute?.channelName).toBe('Economy')
    expect(model.searchableText).toContain('fast route')
    expect(model.searchableText).toContain('pro')
    expect(modelStartingPrice(model)).toBe(0.000005)
  })

  it('does not merge the same model name across platforms', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'Mixed',
        description: '',
        platforms: [
          { platform: 'openai', groups: [], supported_models: [{ name: 'shared', platform: 'openai', pricing: null }] },
          { platform: 'anthropic', groups: [], supported_models: [{ name: 'shared', platform: 'anthropic', pricing: null }] }
        ]
      }
    ]

    expect(buildModelCatalog(channels).map((model) => model.platform)).toEqual([
      'anthropic',
      'openai'
    ])
  })

  it('uses the lowest configured tier price', () => {
    const pricing = tokenPricing(0.00001)
    pricing.intervals = [
      {
        min_tokens: 0,
        max_tokens: 1000,
        input_price: 0.000008,
        output_price: 0.000016,
        cache_write_price: null,
        cache_read_price: null,
        per_request_price: null
      }
    ]

    expect(minimumPricingValue(pricing, 'input_price')).toBe(0.000008)
  })

  it('aggregates passive performance without double-counting a repeated group', () => {
    const sharedPerformance = performance(1, 9, 1)
    const channels: UserAvailableChannel[] = [
      {
        name: 'Primary',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [
              {
                id: 1,
                name: 'default',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 1,
                peak_rate_enabled: false,
                peak_start: '',
                peak_end: '',
                peak_rate_multiplier: 1,
                is_exclusive: false
              }
            ],
            supported_models: [
              { name: 'gpt-5', platform: 'openai', pricing: null, performance: sharedPerformance }
            ]
          }
        ]
      },
      {
        name: 'Secondary',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [
              {
                id: 1,
                name: 'default',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 1,
                peak_rate_enabled: false,
                peak_start: '',
                peak_end: '',
                peak_rate_multiplier: 1,
                is_exclusive: false
              }
            ],
            supported_models: [
              { name: 'gpt-5', platform: 'openai', pricing: null, performance: sharedPerformance }
            ]
          }
        ]
      }
    ]

    const [model] = buildModelCatalog(channels)
    expect(model.performance?.success_rate).toBe(90)
    expect(model.performance?.sample_count).toBe(10)
    expect(model.performance?.groups).toHaveLength(1)
  })
})
