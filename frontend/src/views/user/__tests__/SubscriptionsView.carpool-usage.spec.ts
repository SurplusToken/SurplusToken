import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { CarpoolUsageSnapshot, UserSubscription } from '@/types'
import SubscriptionsView from '../SubscriptionsView.vue'

const { getMySubscriptions, getCarpoolUsage, showError, push } = vi.hoisted(() => ({
  getMySubscriptions: vi.fn(),
  getCarpoolUsage: vi.fn(),
  showError: vi.fn(),
  push: vi.fn(),
}))

vi.mock('@/api/subscriptions', () => ({
  default: {
    getMySubscriptions,
    getCarpoolUsage,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: null,
    showError,
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const CarpoolUsagePanelStub = defineComponent({
  name: 'CarpoolUsagePanel',
  props: {
    snapshot: { type: Object, default: undefined },
    loading: Boolean,
    error: { type: String, default: undefined },
  },
  emits: ['retry'],
  template: `
    <section
      data-testid="carpool-panel-stub"
      :data-loading="String(loading)"
      :data-error="error || ''"
      :data-snapshot-id="snapshot ? String(snapshot.subscriptionId) : ''"
      :data-total-usage="snapshot ? String(snapshot.totalUsageUsd) : ''"
    >
      <button data-testid="retry-carpool" @click="$emit('retry')">retry</button>
    </section>
  `,
})

function makeSubscription(
  id: number,
  overrides: Partial<UserSubscription> & { group?: Record<string, unknown> } = {},
): UserSubscription {
  const group = {
    id,
    name: `Group ${id}`,
    description: '',
    platform: 'openai',
    rate_multiplier: 1,
    daily_limit_usd: null,
    weekly_limit_usd: 100,
    monthly_limit_usd: null,
    ...overrides.group,
  }

  return {
    id,
    user_id: 1,
    group_id: id,
    is_carpool: false,
    status: 'active',
    starts_at: '2026-08-03T00:00:00Z',
    daily_usage_usd: 0,
    weekly_usage_usd: 20,
    monthly_usage_usd: 0,
    daily_window_start: null,
    weekly_window_start: '2026-08-03T00:00:00Z',
    monthly_window_start: null,
    created_at: '2026-08-03T00:00:00Z',
    updated_at: '2026-08-03T00:00:00Z',
    expires_at: null,
    ...overrides,
    group,
  } as UserSubscription
}

function makeSnapshot(subscriptionId: number): CarpoolUsageSnapshot {
  return {
    subscriptionId,
    windowStart: '2026-08-03T00:00:00Z',
    windowEnd: '2026-08-10T00:00:00Z',
    totalUsageUsd: subscriptionId,
    totalCapacityUsd: 2400,
    sharedPool: {
      usageUsd: 28,
      capacityUsd: 480,
      remainingUsd: 452,
    },
    members: [],
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function mountView() {
  return mount(SubscriptionsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
        CarpoolUsagePanel: CarpoolUsagePanelStub,
      },
    },
  })
}

function subscriptionCard(wrapper: ReturnType<typeof mountView>, id: number) {
  return wrapper.get(`[data-testid="subscription-card"][data-subscription-id="${id}"]`)
}

describe('SubscriptionsView carpool usage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders subscription cards while the carpool detail request is still pending', async () => {
    const usageRequest = deferred<CarpoolUsageSnapshot[]>()
    getMySubscriptions.mockResolvedValue([makeSubscription(101, { is_carpool: true })])
    getCarpoolUsage.mockReturnValue(usageRequest.promise)

    const wrapper = mountView()
    await flushPromises()

    expect(subscriptionCard(wrapper, 101).text()).toContain('Group 101')
    expect(subscriptionCard(wrapper, 101).get('[data-testid="carpool-panel-stub"]').attributes('data-loading')).toBe('true')
    expect(getCarpoolUsage).toHaveBeenCalledTimes(1)

    usageRequest.resolve([makeSnapshot(101)])
    await flushPromises()
    expect(subscriptionCard(wrapper, 101).get('[data-testid="carpool-panel-stub"]').attributes('data-snapshot-id')).toBe('101')
  })

  it('loads one batch and maps each snapshot to its active carpool card', async () => {
    getMySubscriptions.mockResolvedValue([
      makeSubscription(101, { is_carpool: true }),
      makeSubscription(202, { is_carpool: true }),
    ])
    getCarpoolUsage.mockResolvedValue([makeSnapshot(202), makeSnapshot(101)])

    const wrapper = mountView()
    await flushPromises()

    expect(getCarpoolUsage).toHaveBeenCalledTimes(1)
    expect(subscriptionCard(wrapper, 101).get('[data-testid="carpool-panel-stub"]').attributes('data-snapshot-id')).toBe('101')
    expect(subscriptionCard(wrapper, 202).get('[data-testid="carpool-panel-stub"]').attributes('data-snapshot-id')).toBe('202')
  })

  it('replaces only the carpool weekly block and preserves non-carpool quota behavior', async () => {
    getMySubscriptions.mockResolvedValue([
      makeSubscription(101, { is_carpool: true }),
      makeSubscription(202),
      makeSubscription(303, {
        is_carpool: true,
        group: { weekly_limit_usd: null },
      }),
    ])
    getCarpoolUsage.mockResolvedValue([makeSnapshot(101), makeSnapshot(303)])

    const wrapper = mountView()
    await flushPromises()

    expect(subscriptionCard(wrapper, 101).find('[data-testid="weekly-usage"]').exists()).toBe(false)
    expect(subscriptionCard(wrapper, 202).get('[data-testid="weekly-usage"]').text()).toContain('userSubscriptions.weekly')
    expect(subscriptionCard(wrapper, 202).find('[data-testid="carpool-panel-stub"]').exists()).toBe(false)
    expect(subscriptionCard(wrapper, 303).find('[data-testid="unlimited-usage"]').exists()).toBe(false)
    expect(subscriptionCard(wrapper, 303).find('[data-testid="carpool-panel-stub"]').exists()).toBe(true)
  })

  it('does not request or render carpool detail without an active marked subscription', async () => {
    getMySubscriptions.mockResolvedValue([
      makeSubscription(101, { is_carpool: true, status: 'expired' }),
      makeSubscription(102, {
        is_carpool: true,
        status: 'revoked',
        group: { weekly_limit_usd: null },
      }),
      makeSubscription(202),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(getCarpoolUsage).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="carpool-panel-stub"]').exists()).toBe(false)
    expect(subscriptionCard(wrapper, 101).find('[data-testid="weekly-usage"]').exists()).toBe(true)
    expect(subscriptionCard(wrapper, 102).find('[data-testid="unlimited-usage"]').exists()).toBe(true)
  })

  it('keeps cards visible and reports detail failures only inside carpool panels', async () => {
    getMySubscriptions.mockResolvedValue([
      makeSubscription(101, { is_carpool: true }),
      makeSubscription(202),
    ])
    getCarpoolUsage.mockRejectedValue(new Error('detail unavailable'))

    const wrapper = mountView()
    await flushPromises()

    expect(subscriptionCard(wrapper, 101).exists()).toBe(true)
    expect(subscriptionCard(wrapper, 202).exists()).toBe(true)
    expect(subscriptionCard(wrapper, 101).get('[data-testid="carpool-panel-stub"]').attributes('data-error')).toBe('detail unavailable')
    expect(showError).not.toHaveBeenCalled()
  })

  it('shows an inline error when a successful batch omits an active carpool snapshot', async () => {
    getMySubscriptions.mockResolvedValue([makeSubscription(101, { is_carpool: true })])
    getCarpoolUsage.mockResolvedValue([])

    const wrapper = mountView()
    await flushPromises()

    expect(subscriptionCard(wrapper, 101).get('[data-testid="carpool-panel-stub"]').attributes('data-error')).toBe('userSubscriptions.carpoolUsage.loadFailed')
  })

  it('clears the inline error while retrying and applies the retried snapshot', async () => {
    const retryRequest = deferred<CarpoolUsageSnapshot[]>()
    getMySubscriptions.mockResolvedValue([makeSubscription(101, { is_carpool: true })])
    getCarpoolUsage
      .mockRejectedValueOnce(new Error('detail unavailable'))
      .mockReturnValueOnce(retryRequest.promise)

    const wrapper = mountView()
    await flushPromises()

    const card = subscriptionCard(wrapper, 101)
    expect(card.get('[data-testid="carpool-panel-stub"]').attributes('data-error')).toBe('detail unavailable')

    await card.get('[data-testid="retry-carpool"]').trigger('click')
    expect(card.get('[data-testid="carpool-panel-stub"]').attributes('data-loading')).toBe('true')
    expect(card.get('[data-testid="carpool-panel-stub"]').attributes('data-error')).toBe('')
    expect(getCarpoolUsage).toHaveBeenCalledTimes(2)

    retryRequest.resolve([makeSnapshot(101)])
    await flushPromises()
    expect(card.get('[data-testid="carpool-panel-stub"]').attributes('data-snapshot-id')).toBe('101')
  })

  it('ignores an older request failure while a newer retry is still pending', async () => {
    const initialRequest = deferred<CarpoolUsageSnapshot[]>()
    const retryRequest = deferred<CarpoolUsageSnapshot[]>()
    getMySubscriptions.mockResolvedValue([makeSubscription(101, { is_carpool: true })])
    getCarpoolUsage
      .mockReturnValueOnce(initialRequest.promise)
      .mockReturnValueOnce(retryRequest.promise)

    const wrapper = mountView()
    await flushPromises()

    const panel = subscriptionCard(wrapper, 101).get('[data-testid="carpool-panel-stub"]')
    await panel.get('[data-testid="retry-carpool"]').trigger('click')

    initialRequest.reject(new Error('stale failure'))
    await flushPromises()
    expect(panel.attributes('data-loading')).toBe('true')
    expect(panel.attributes('data-error')).toBe('')

    retryRequest.resolve([{ ...makeSnapshot(101), totalUsageUsd: 222 }])
    await flushPromises()
    expect(panel.attributes('data-loading')).toBe('false')
    expect(panel.attributes('data-error')).toBe('')
    expect(panel.attributes('data-total-usage')).toBe('222')
  })
})
