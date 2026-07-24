import { ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CarpoolView from '../CarpoolView.vue'

const { replace, showWarning, listCarpools } = vi.hoisted(() => ({
  replace: vi.fn(),
  showWarning: vi.fn(),
  listCarpools: vi.fn(),
}))

vi.mock('@/api/carpools', () => ({
  default: {
    list: listCarpools,
    create: vi.fn(),
    resolveInvite: vi.fn(),
    createInvite: vi.fn(),
    join: vi.fn(),
    joinByInvite: vi.fn(),
    launch: vi.fn(),
    declarationRecommendation: vi.fn(),
    settlement: vi.fn(),
    cancel: vi.fn(),
    setJoinLocked: vi.fn(),
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: ref('zh'),
      t: (key: string) => key,
    }),
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: {} }),
  useRouter: () => ({ replace }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: vi.fn(),
    showWarning,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isAdmin: false,
    user: { id: 1, username: 'preview-user' },
  }),
}))

function makeCarpool(overrides: Record<string, unknown> = {}) {
  return {
    id: 10,
    name: 'open-car',
    description: '',
    organizer: 'owner-a',
    platform: 'openai',
    planType: 'openai_pro',
    carType: 'small',
    level: 1,
    capacity: 0,
    memberCount: 2,
    baseFeeCny: 0,
    usagePoolCnyPerAccount: 0,
    visibility: 'public',
    status: 'recruiting',
    joinLocked: false,
    scheduledStartAt: '2026-08-01',
    groupName: null,
    memberRole: null,
    createdAt: '2026-07-01T00:00:00Z',
    weeklyLimitUsd: 2400,
    seatFeeCny: 400,
    usagePoolCny: 1000,
    reserveRatio: 0.8,
    launchMinRatio: 0.95,
    launchMaxRatio: 1.05,
    declaredTotalUsd: 1200,
    remainingJoinableUsd: 1320,
    plusEquivalents: 10,
    avgPriceCny: 90,
    ...overrides,
  }
}

function mountView() {
  return shallowMount(CarpoolView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: true,
        ConfirmDialog: true,
        Icon: true,
      },
    },
  })
}

describe('CarpoolView', () => {
  beforeEach(() => {
    localStorage.clear()
    replace.mockReset()
    showWarning.mockReset()
    listCarpools.mockReset()
    listCarpools.mockResolvedValue([])
  })

  it('shows the quota reservation rules and does not seed any car', async () => {
    const wrapper = mountView()
    await flushPromises()
    const rules = wrapper.get('[data-testid="gpt-carpool-rules"]')

    expect(rules.text()).toContain('carpool.rules.declare.text')
    expect(rules.text()).toContain('carpool.rules.reserve.text')
    expect(rules.text()).toContain('carpool.rules.pricing.text')
    expect(rules.text()).toContain('carpool.rules.floor.text')
    expect(rules.text()).toContain('carpool.notices.weeklyRefresh')
    expect(rules.text()).toContain('carpool.notices.consumeOrder')
    expect(wrapper.findAll('article')).toHaveLength(0)
    expect(listCarpools).toHaveBeenCalledOnce()
  })

  it('renders joined state only from the current user membership returned by the API', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool(),
      makeCarpool({
        id: 11,
        name: 'joined-car',
        organizer: 'owner-b',
        memberCount: 3,
        memberRole: 'member',
      }),
    ])

    const wrapper = mountView()
    await flushPromises()
    const cards = wrapper.findAll('article')

    expect(cards).toHaveLength(2)
    expect(cards[0].text()).toContain('carpool.actions.join')
    expect(cards[0].text()).not.toContain('carpool.actions.joined')
    expect(cards[1].text()).toContain('carpool.actions.joined')
  })

  it('shows quota reservation metrics instead of seat counts on the card', async () => {
    listCarpools.mockResolvedValue([makeCarpool()])

    const wrapper = mountView()
    await flushPromises()
    const card = wrapper.get('article')

    expect(card.text()).toContain('carpool.fields.quotaProgress')
    expect(card.text()).toContain('carpool.fields.remainingJoinable')
    expect(card.text()).toContain('carpool.fields.plusEquivalents')
    expect(card.text()).toContain('carpool.fields.avgPrice')
    expect(card.text()).not.toContain('carpool.fields.seatsRemaining')
  })

  it('hides the join action when no joinable quota remains', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ remainingJoinableUsd: 0, declaredTotalUsd: 2520 }),
    ])

    const wrapper = mountView()
    await flushPromises()
    const card = wrapper.get('article')

    expect(card.text()).not.toContain('carpool.actions.join')
    expect(card.text()).toContain('carpool.status.full')
  })

  it('enables launch for the owner inside the band and disables it below the force floor', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 20, name: 'ready-car', memberRole: 'owner', declaredTotalUsd: 2350, remainingJoinableUsd: 170 }),
      makeCarpool({ id: 21, name: 'early-car', memberRole: 'owner', declaredTotalUsd: 960, remainingJoinableUsd: 1560 }),
    ])

    const wrapper = mountView()
    await flushPromises()

    const readyButton = wrapper.get('[data-testid="launch-20"]')
    expect(readyButton.attributes('disabled')).toBeUndefined()

    const earlyButton = wrapper.get('[data-testid="launch-21"]')
    expect(earlyButton.attributes('disabled')).toBeDefined()
  })

  it('does not show the launch button to non-owner members', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 22, memberRole: 'member', declaredTotalUsd: 2350 }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="launch-22"]').exists()).toBe(false)
  })
})
