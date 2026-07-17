import { ref } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CarpoolView from '../CarpoolView.vue'

const { replace, showWarning } = vi.hoisted(() => ({
  replace: vi.fn(),
  showWarning: vi.fn(),
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
  })

  it('shows the GPT tier rules on the standalone carpool page', () => {
    const wrapper = mountView()
    const rules = wrapper.get('[data-testid="gpt-carpool-rules"]')

    expect(rules.text()).toContain('carpool.rules.small.usageFee')
    expect(rules.text()).toContain('carpool.rules.large.usageFee')
    expect(rules.text()).toContain('carpool.rules.lockNotice')
    expect(wrapper.findAll('article')).toHaveLength(2)
  })

  it('keeps car1 and car2 locked even when saved preview data says they are open', () => {
    localStorage.setItem('surplusai_carpool_preview_v2', JSON.stringify([
      {
        id: 8,
        name: 'car2',
        description: 'saved open car',
        organizer: 'SurplusToken',
        carType: 'small',
        level: 2,
        capacity: 10,
        memberCount: 7,
        visibility: 'public',
        status: 'recruiting',
        joinLocked: false,
        scheduledStartAt: '2026-08-01',
        groupName: 'car2',
        memberRole: null,
        inviteCode: 'CAR2DEMO',
        createdAt: '2026-07-01T00:00:00Z',
      },
    ]))

    const wrapper = mountView()

    expect(wrapper.get('article').text()).toContain('carpool.status.locked')
    expect(wrapper.get('article').text()).not.toContain('carpool.actions.join')
  })
})
