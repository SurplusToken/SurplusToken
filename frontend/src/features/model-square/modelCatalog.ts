import type {
  UserAvailableChannel,
  UserAvailableGroup,
  UserModelGroupPerformance,
  UserModelPerformance,
  UserAccountPricingOverride,
  UserSupportedModelPricing
} from '@/api/channels'

export interface ModelCatalogRoute {
  key: string
  channelName: string
  channelDescription: string
  platform: string
  groups: UserAvailableGroup[]
  pricing: UserSupportedModelPricing | null
  accountPricingOverrides: UserAccountPricingOverride[]
  performance: UserModelPerformance | null
}

export interface ModelCatalogItem {
  key: string
  name: string
  platform: string
  routes: ModelCatalogRoute[]
  groups: UserAvailableGroup[]
  bestRoute: ModelCatalogRoute | null
  performance: UserModelPerformance | null
  searchableText: string
}

type PriceField =
  | 'input_price'
  | 'output_price'
  | 'cache_write_price'
  | 'cache_read_price'
  | 'image_input_price'
  | 'image_output_price'
  | 'per_request_price'

export function minimumPricingValue(
  pricing: UserSupportedModelPricing | null,
  field: PriceField
): number | null {
  if (!pricing) return null

  const values: number[] = []
  const baseValue = pricing[field]
  if (typeof baseValue === 'number' && Number.isFinite(baseValue)) values.push(baseValue)

  if (
    field === 'input_price' ||
    field === 'output_price' ||
    field === 'cache_write_price' ||
    field === 'cache_read_price' ||
    field === 'per_request_price'
  ) {
    for (const interval of pricing.intervals ?? []) {
      const intervalValue = interval[field]
      if (typeof intervalValue === 'number' && Number.isFinite(intervalValue)) {
        values.push(intervalValue)
      }
    }
  }

  return values.length > 0 ? Math.min(...values) : null
}

function pricingScore(pricing: UserSupportedModelPricing | null): number {
  if (!pricing) return Number.POSITIVE_INFINITY

  const candidates =
    pricing.billing_mode === 'per_request'
      ? [minimumPricingValue(pricing, 'per_request_price')]
      : pricing.billing_mode === 'image'
        ? [
            minimumPricingValue(pricing, 'image_output_price'),
            minimumPricingValue(pricing, 'image_input_price'),
            minimumPricingValue(pricing, 'per_request_price')
          ]
        : [
            minimumPricingValue(pricing, 'input_price'),
            minimumPricingValue(pricing, 'output_price')
          ]

  return candidates.find((value): value is number => value !== null) ?? Number.POSITIVE_INFINITY
}

function selectBestRoute(routes: ModelCatalogRoute[]): ModelCatalogRoute | null {
  if (routes.length === 0) return null
  return [...routes].sort((a, b) => {
    const scoreDifference = pricingScore(a.pricing) - pricingScore(b.pricing)
    if (Number.isFinite(scoreDifference) && scoreDifference !== 0) return scoreDifference
    if (a.pricing && !b.pricing) return -1
    if (!a.pricing && b.pricing) return 1
    return a.channelName.localeCompare(b.channelName)
  })[0]
}

function aggregatePerformance(
  performances: Array<UserModelPerformance | null | undefined>
): UserModelPerformance | null {
  const groupsByID = new Map<number, UserModelGroupPerformance>()
  let windowHours = 24
  for (const performance of performances) {
    if (!performance) continue
    windowHours = performance.window_hours || windowHours
    for (const group of performance.groups ?? []) groupsByID.set(group.group_id, group)
  }

  const groups = [...groupsByID.values()].sort((a, b) => a.group_id - b.group_id)
  const successCount = groups.reduce((sum, group) => sum + group.success_count, 0)
  const failureCount = groups.reduce((sum, group) => sum + group.failure_count, 0)
  const sampleCount = successCount + failureCount
  if (sampleCount <= 0) return null

  const latencyGroups = groups.filter(
    (group) => group.avg_latency_ms !== null && group.latency_sample_count > 0
  )
  const latencySampleCount = latencyGroups.reduce(
    (sum, group) => sum + group.latency_sample_count,
    0
  )
  const ttftGroups = groups.filter(
    (group) => group.avg_ttft_ms !== null && group.ttft_sample_count > 0
  )
  const ttftSampleCount = ttftGroups.reduce((sum, group) => sum + group.ttft_sample_count, 0)

  return {
    window_hours: windowHours,
    success_rate: Math.round((successCount / sampleCount) * 10_000) / 100,
    success_count: successCount,
    failure_count: failureCount,
    sample_count: sampleCount,
    latency_sample_count: latencySampleCount,
    ttft_sample_count: ttftSampleCount,
    avg_latency_ms:
      latencySampleCount > 0
        ? Math.round(
            latencyGroups.reduce(
              (sum, group) => sum + (group.avg_latency_ms ?? 0) * group.latency_sample_count,
              0
            ) / latencySampleCount
          )
        : null,
    avg_ttft_ms:
      ttftSampleCount > 0
        ? Math.round(
            ttftGroups.reduce(
              (sum, group) => sum + (group.avg_ttft_ms ?? 0) * group.ttft_sample_count,
              0
            ) / ttftSampleCount
          )
        : null,
    groups
  }
}

export function buildModelCatalog(channels: UserAvailableChannel[]): ModelCatalogItem[] {
  const catalog = new Map<
    string,
    Omit<ModelCatalogItem, 'groups' | 'bestRoute' | 'performance' | 'searchableText'>
  >()

  for (const channel of channels) {
    for (const section of channel.platforms ?? []) {
      for (const supportedModel of section.supported_models ?? []) {
        const platform = supportedModel.platform || section.platform
        const modelKey = `${platform}\u0000${supportedModel.name}`
        let item = catalog.get(modelKey)
        if (!item) {
          item = {
            key: modelKey,
            name: supportedModel.name,
            platform,
            routes: []
          }
          catalog.set(modelKey, item)
        }

        const routeKey = `${channel.name}\u0000${section.platform}`
        const existingRoute = item.routes.find((route) => route.key === routeKey)
        if (existingRoute) {
          const knownGroupIds = new Set(existingRoute.groups.map((group) => group.id))
          existingRoute.groups.push(...section.groups.filter((group) => !knownGroupIds.has(group.id)))
          if (!existingRoute.pricing && supportedModel.pricing) {
            existingRoute.pricing = supportedModel.pricing
          }
          for (const override of supportedModel.account_pricing_overrides ?? []) {
            const duplicate = existingRoute.accountPricingOverrides.some(
              (entry) => entry.group_id === override.group_id && entry.account_name === override.account_name
            )
            if (!duplicate) existingRoute.accountPricingOverrides.push(override)
          }
          existingRoute.performance = aggregatePerformance([
            existingRoute.performance,
            supportedModel.performance
          ])
          continue
        }

        item.routes.push({
          key: routeKey,
          channelName: channel.name,
          channelDescription: channel.description || '',
          platform: section.platform,
          groups: [...section.groups],
          pricing: supportedModel.pricing,
          accountPricingOverrides: [...(supportedModel.account_pricing_overrides ?? [])],
          performance: supportedModel.performance ?? null
        })
      }
    }
  }

  return [...catalog.values()]
    .map((item): ModelCatalogItem => {
      const groupsById = new Map<number, UserAvailableGroup>()
      for (const route of item.routes) {
        for (const group of route.groups) groupsById.set(group.id, group)
      }
      const groups = [...groupsById.values()]
      const searchableText = [
        item.name,
        item.platform,
        ...item.routes.flatMap((route) => [
          route.channelName,
          route.channelDescription,
          ...route.groups.map((group) => group.name)
        ])
      ]
        .join(' ')
        .toLocaleLowerCase()

      return {
        ...item,
        routes: item.routes.sort((a, b) => a.channelName.localeCompare(b.channelName)),
        groups,
        bestRoute: selectBestRoute(item.routes),
        performance: aggregatePerformance(item.routes.map((route) => route.performance)),
        searchableText
      }
    })
    .sort((a, b) => a.name.localeCompare(b.name) || a.platform.localeCompare(b.platform))
}

export function modelStartingPrice(model: ModelCatalogItem): number | null {
  const pricing = model.bestRoute?.pricing ?? null
  if (!pricing) return null
  if (pricing.billing_mode === 'per_request') {
    return minimumPricingValue(pricing, 'per_request_price')
  }
  if (pricing.billing_mode === 'image') {
    return (
      minimumPricingValue(pricing, 'image_output_price') ??
      minimumPricingValue(pricing, 'image_input_price') ??
      minimumPricingValue(pricing, 'per_request_price')
    )
  }
  return (
    minimumPricingValue(pricing, 'input_price') ?? minimumPricingValue(pricing, 'output_price')
  )
}
