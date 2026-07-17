import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountPoolView from '../AccountPoolView.vue'

const { listPool, listProxies, getContributionSummary, getAvailableGroups, appState } = vi.hoisted(() => ({
  listPool: vi.fn(),
  listProxies: vi.fn(),
  getContributionSummary: vi.fn(),
  getAvailableGroups: vi.fn(),
  appState: {
    cachedPublicSettings: null as Record<string, unknown> | null,
    fetchPublicSettings: vi.fn(),
  },
}))

vi.mock('@/api/accounts', async () => {
  const actual = await vi.importActual<typeof import('@/api/accounts')>('@/api/accounts')
  return {
    ...actual,
    default: {
      listPool,
      listProxies,
      testProxy: vi.fn(),
      getContributionPool: vi.fn(),
    },
  }
})

vi.mock('@/api/user', () => ({
  default: { getContributionSummary, transferContributionQuota: vi.fn() },
}))

vi.mock('@/api/groups', () => ({
  userGroupsAPI: { getAvailable: getAvailableGroups },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    ...appState,
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const DataTableStub = defineComponent({
  props: { columns: { type: Array, default: () => [] } },
  setup(props) {
    return () => h('div', {
      'data-testid': 'account-pool-columns',
      'data-columns': (props.columns as Array<{ key: string }>).map((column) => column.key).join(','),
    })
  },
})

function mountView() {
  return shallowMount(AccountPoolView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="actions" /><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: DataTableStub,
        Pagination: true,
        Icon: true,
      },
    },
  })
}

describe('AccountPoolView sharing rate column', () => {
  beforeEach(() => {
    listPool.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 50 })
    listProxies.mockResolvedValue([])
    getContributionSummary.mockResolvedValue({})
    getAvailableGroups.mockResolvedValue([])
    appState.fetchPublicSettings.mockReset()
  })

  it('offers the rate as a toggleable table column when marketplace display is enabled', async () => {
    appState.cachedPublicSettings = {
      sharing_pool_display_enabled: true,
      sharing_rate_floor: 0,
      sharing_rate_cap: 5,
      sharing_rate_cooldown_minutes: 10,
    }
    appState.fetchPublicSettings.mockResolvedValue(appState.cachedPublicSettings)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="account-pool-columns"]').attributes('data-columns'))
      .toContain('sharing_rate')
    wrapper.unmount()
  })

  it('does not expose the rate column when marketplace display is disabled', async () => {
    appState.cachedPublicSettings = { sharing_pool_display_enabled: false }
    appState.fetchPublicSettings.mockResolvedValue(appState.cachedPublicSettings)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="account-pool-columns"]').attributes('data-columns'))
      .not.toContain('sharing_rate')
    wrapper.unmount()
  })
})
