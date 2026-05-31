import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

import AccountPoolView from '../AccountPoolView.vue'

const {
  listPool,
  listProxies,
  getContributionSummary,
  getAvailableGroups,
} = vi.hoisted(() => ({
  listPool: vi.fn(),
  listProxies: vi.fn(),
  getContributionSummary: vi.fn(),
  getAvailableGroups: vi.fn(),
}))

vi.mock('@/api/accounts', async () => {
  const actual = await vi.importActual<typeof import('@/api/accounts')>('@/api/accounts')
  return {
    ...actual,
    default: {
      listPool,
      listProxies,
      testProxy: vi.fn(),
      generateOAuthAuthUrl: vi.fn(),
      exchangeOAuthCode: vi.fn(),
      createOAuth: vi.fn(),
      refreshOpenAIToken: vi.fn(),
      importCodexSession: vi.fn(),
      getStats: vi.fn(),
      getAvailableModels: vi.fn(),
      scheduledTests: {},
    },
  }
})

vi.mock('@/api/user', () => ({
  default: {
    getContributionSummary,
    transferContributionQuota: vi.fn(),
  },
}))

vi.mock('@/api/groups', () => ({
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
  }),
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

const OAuthAuthorizationFlowStub = {
  name: 'OAuthAuthorizationFlow',
  props: [
    'showRefreshTokenOption',
    'showMobileRefreshTokenOption',
    'showCodexSessionImportOption',
    'platform',
  ],
  template: '<div data-test="oauth-flow"></div>',
}

describe('AccountPoolView OAuth options', () => {
  beforeEach(() => {
    listPool.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 50 })
    listProxies.mockResolvedValue([])
    getContributionSummary.mockResolvedValue({})
    getAvailableGroups.mockResolvedValue([])
  })

  it('shows original sub2api OpenAI authorization input methods for user contributions', async () => {
    const wrapper = shallowMount(AccountPoolView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="actions" /><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
          DataTable: { template: '<div />' },
          Pagination: { template: '<div />' },
          OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
          ModelWhitelistSelector: { template: '<div />' },
          GroupSelector: { template: '<div />' },
          ProxySelector: { template: '<div />' },
          ConfirmDialog: { template: '<div />' },
          AccountStatsModal: { template: '<div />' },
          AccountTestModal: { template: '<div />' },
          ScheduledTestsPanel: { template: '<div />' },
          UserAccountActionMenu: { template: '<div />' },
          Icon: { template: '<span />' },
        },
      },
    })

    await flushPromises()
    await wrapper.find('button').trigger('click')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    const flow = wrapper.findComponent(OAuthAuthorizationFlowStub)
    expect(flow.props('showRefreshTokenOption')).toBe(true)
    expect(flow.props('showMobileRefreshTokenOption')).toBe(true)
    expect(flow.props('showCodexSessionImportOption')).toBe(true)
    expect(flow.props('platform')).toBe('openai')
  })
})
