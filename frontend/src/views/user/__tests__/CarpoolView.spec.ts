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
})
