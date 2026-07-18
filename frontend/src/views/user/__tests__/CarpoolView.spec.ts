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

  it('shows the GPT tier rules and does not seed car1 or car2', async () => {
    const wrapper = mountView()
    await flushPromises()
    const rules = wrapper.get('[data-testid="gpt-carpool-rules"]')

    expect(rules.text()).toContain('carpool.rules.small.usageFee')
    expect(rules.text()).toContain('carpool.rules.large.usageFee')
    expect(rules.text()).toContain('carpool.rules.lockNotice')
    expect(wrapper.findAll('article')).toHaveLength(0)
    expect(listCarpools).toHaveBeenCalledOnce()
  })

  it('renders joined state only from the current user membership returned by the API', async () => {
    listCarpools.mockResolvedValue([
      {
        id: 10,
        name: 'open-car',
        description: '',
        organizer: 'owner-a',
        platform: 'openai',
        planType: 'openai_pro',
        carType: 'small',
        level: 1,
        capacity: 5,
        memberCount: 2,
        baseFeeCny: 130,
        usagePoolCnyPerAccount: 750,
        visibility: 'public',
        status: 'recruiting',
        joinLocked: false,
        scheduledStartAt: '2026-08-01',
        groupName: null,
        memberRole: null,
        createdAt: '2026-07-01T00:00:00Z',
      }, {
        id: 11,
        name: 'joined-car',
        description: '',
        organizer: 'owner-b',
        platform: 'openai',
        planType: 'openai_pro',
        carType: 'small',
        level: 1,
        capacity: 5,
        memberCount: 3,
        baseFeeCny: 130,
        usagePoolCnyPerAccount: 750,
        visibility: 'public',
        status: 'recruiting',
        joinLocked: false,
        scheduledStartAt: '2026-08-02',
        groupName: null,
        memberRole: 'member',
        createdAt: '2026-07-02T00:00:00Z',
      },
    ])

    const wrapper = mountView()
    await flushPromises()
    const cards = wrapper.findAll('article')

    expect(cards).toHaveLength(2)
    expect(cards[0].text()).toContain('carpool.actions.join')
    expect(cards[0].text()).not.toContain('carpool.actions.joined')
    expect(cards[1].text()).toContain('carpool.actions.joined')
  })
})
