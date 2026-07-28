import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { ref } from 'vue'
import CarpoolsView from '../CarpoolsView.vue'

const {
  showSuccess,
  showWarning,
  showError,
  adminOverviewMock,
  rosterMock,
  removeMemberMock,
  updateMemberQuotaMock,
  updateCarpoolMock,
  transferOwnerMock,
  setJoinLockedMock,
  unconfirmMock,
  launchMock,
  cancelMock,
  groupQrCodeMock,
} = vi.hoisted(() => ({
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
  showError: vi.fn(),
  adminOverviewMock: vi.fn(),
  rosterMock: vi.fn(),
  removeMemberMock: vi.fn(),
  updateMemberQuotaMock: vi.fn(),
  updateCarpoolMock: vi.fn(),
  transferOwnerMock: vi.fn(),
  setJoinLockedMock: vi.fn(),
  unconfirmMock: vi.fn(),
  launchMock: vi.fn(),
  cancelMock: vi.fn(),
  groupQrCodeMock: vi.fn(),
}))

vi.mock('@/api/carpools', () => ({
  default: {
    adminOverview: adminOverviewMock,
    roster: rosterMock,
    removeMember: removeMemberMock,
    updateMemberQuota: updateMemberQuotaMock,
    updateCarpool: updateCarpoolMock,
    transferOwner: transferOwnerMock,
    setJoinLocked: setJoinLockedMock,
    unconfirm: unconfirmMock,
    launch: launchMock,
    cancel: cancelMock,
    groupQrCode: groupQrCodeMock,
  },
}))

// jsdom 没有 createObjectURL/revokeObjectURL，补两个能断言的
URL.createObjectURL = vi.fn(() => 'blob:mock-qr')
URL.revokeObjectURL = vi.fn()

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ locale: ref('zh'), t: (key: string) => key }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError, showWarning }),
}))

const DAY_MS = 24 * 60 * 60 * 1000

function makeCarpool(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: 'car-a',
    description: '',
    organizer: 'alice',
    ownerUserId: 1,
    platform: 'openai',
    planType: 'openai_pro',
    carType: 'large',
    level: 1,
    capacity: 30,
    memberCount: 3,
    baseFeeCny: 0,
    usagePoolCnyPerAccount: 0,
    visibility: 'public',
    status: 'recruiting',
    joinLocked: false,
    scheduledStartAt: '2026-08-01',
    groupName: null,
    memberRole: null,
    createdAt: '2026-07-01T00:00:00Z',
    adminWechat: '',
    hasGroupQrCode: false,
    pricingModel: 'quota',
    ruleNote: '',
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
  return shallowMount(CarpoolsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        // 真实的 TablePageLayout 只有 filters/table 等命名插槽、没有默认插槽——
        // 这页曾经把内容全塞默认插槽，线上整页空白而测试照绿，桩子必须与真实件一致。
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
        DataTable: {
          name: 'DataTable',
          props: ['columns', 'data', 'loading'],
          // 把每一行的 actions 插槽渲染出来，用例才点得到按钮
          template:
            '<div data-testid="table">' +
            '<div v-for="row in data" :key="row.id" data-testid="row">' +
            '<slot name="cell-name" :row="row" /><slot name="cell-actions" :row="row" />' +
            '</div></div>',
        },
        BaseDialog: {
          name: 'BaseDialog',
          props: ['show', 'title', 'width'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        // 把自定义下拉渲染成原生 option，用例才能直接断言候选列表
        Select: {
          name: 'Select',
          props: ['options', 'modelValue', 'placeholder'],
          template: '<select><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>',
        },
      },
    },
  })
}

function findButton(wrapper: ReturnType<typeof mountView>, text: string) {
  const btn = wrapper.findAll('button').find((b) => b.text().includes(text))
  if (!btn) throw new Error(`button not found: ${text}`)
  return btn
}

describe('admin CarpoolsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    adminOverviewMock.mockResolvedValue([])
    rosterMock.mockResolvedValue([])
  })

  // 管理总览必须走 adminOverview 而不是用户侧的 list：后者的 SQL 会把
  // 别人的私密车和已取消的车全部过滤掉，管理员根本看不见。
  it('loads every carpool through the admin overview endpoint', async () => {
    adminOverviewMock.mockResolvedValue([
      makeCarpool({ id: 1, name: 'car-a' }),
      makeCarpool({ id: 2, name: 'car-b', status: 'cancelled', visibility: 'invite_only' }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(adminOverviewMock).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('[data-testid="row"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('car-b')
  })

  // 异常条是这页的主要价值：管理员要的是「哪几辆现在要我处理」。
  it('flags carpools that need attention', async () => {
    adminOverviewMock.mockResolvedValue([
      // 确认超 24 小时还没启动
      makeCarpool({ id: 1, status: 'confirmed', confirmedAt: new Date(Date.now() - 2 * DAY_MS).toISOString() }),
      // 已结束却没结算
      makeCarpool({ id: 2, status: 'ended', settledAt: undefined }),
      // 申报超过 105% 上限（不该同时被标成「待确认」——车主确认不了）
      makeCarpool({ id: 3, declaredTotalUsd: 3000 }),
      // 达到 95% 发车线，等车主确认
      makeCarpool({ id: 4, declaredTotalUsd: 2300 }),
    ])

    const wrapper = mountView()
    await flushPromises()

    const alerts = wrapper.get('[data-testid="carpool-admin-alerts"]').text()
    expect(alerts).toContain('carpool.adminPage.alerts.launchOverdue')
    expect(alerts).toContain('carpool.adminPage.alerts.unsettled')
    expect(alerts).toContain('carpool.adminPage.alerts.overDeclared')
    expect(alerts).toContain('carpool.adminPage.alerts.readyToConfirm')
  })

  // 超出上限的车不能同时提示「已达发车线待确认」：后端的 Confirm 要求
  // Σ申报 落在 [95%,105%] 区间内，超上限时车主点确认必定被拒。
  it('does not mark an over-declared carpool as ready to confirm', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, declaredTotalUsd: 3000 })])

    const wrapper = mountView()
    await flushPromises()

    const alerts = wrapper.get('[data-testid="carpool-admin-alerts"]').text()
    expect(alerts).toContain('carpool.adminPage.alerts.overDeclared')
    expect(alerts).not.toContain('carpool.adminPage.alerts.readyToConfirm')
  })

  // 自定义规则车不走申报制，额度类异常对它们不成立，不能误报「申报超上限」。
  it('does not raise quota alerts for custom-rule carpools', async () => {
    adminOverviewMock.mockResolvedValue([
      makeCarpool({ id: 1, pricingModel: 'custom', declaredTotalUsd: 9999 }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="carpool-admin-alerts"]').exists()).toBe(false)
  })

  // 发车前才能改人：已发车的车成员只读，不渲染移出按钮。
  it('keeps the roster read-only once the carpool is active', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'active' })])
    rosterMock.mockResolvedValue([
      { userId: 2, username: 'bob', role: 'member', declaredWeeklyQuotaUsd: 400 },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.adminPage.membersDialog.readOnly')
    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.adminPage.actions.remove'))).toBe(false)
  })

  // 移出成员后要刷新总览与花名册，否则页面上还留着已经不在车上的人。
  it('removes a member and refreshes both the table and the roster', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'recruiting' })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'alice', role: 'owner', declaredWeeklyQuotaUsd: 600 },
      { userId: 2, username: 'bob', role: 'member', declaredWeeklyQuotaUsd: 400 },
    ])
    removeMemberMock.mockResolvedValue({ carpool: makeCarpool(), autoUnconfirmed: false })

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    // 车主没有移出按钮，只有 bob 有
    const removeButtons = wrapper.findAll('button').filter((b) => b.text().includes('carpool.adminPage.actions.remove'))
    expect(removeButtons).toHaveLength(1)

    await removeButtons[0].trigger('click')
    await flushPromises()

    expect(removeMemberMock).toHaveBeenCalledWith(1, 2)
    expect(showSuccess).toHaveBeenCalledWith('carpool.adminPage.memberRemoved')
    expect(adminOverviewMock).toHaveBeenCalledTimes(2)
    expect(rosterMock).toHaveBeenCalledTimes(2)
  })

  // 踢人把车打出发车区间时，界面必须明说「已退回招募中」——
  // 悄悄改状态最容易让人以为系统坏了。
  it('warns instead of a plain success when the carpool auto-unconfirms', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'confirmed' })])
    rosterMock.mockResolvedValue([
      { userId: 2, username: 'bob', role: 'member', declaredWeeklyQuotaUsd: 400 },
    ])
    removeMemberMock.mockResolvedValue({ carpool: makeCarpool(), autoUnconfirmed: true })

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.remove').trigger('click')
    await flushPromises()

    expect(showWarning).toHaveBeenCalledWith('carpool.adminPage.autoUnconfirmed')
    expect(showSuccess).not.toHaveBeenCalled()
  })

  // 危险操作要过一道确认，不能点一下就把车取消了。
  it('asks for confirmation before cancelling a carpool', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'recruiting' })])
    cancelMock.mockResolvedValue(undefined)

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.cancel').trigger('click')
    await flushPromises()

    expect(cancelMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('carpool.adminPage.confirm.cancel.message')

    await findButton(wrapper, 'common.confirm').trigger('click')
    await flushPromises()
    expect(cancelMock).toHaveBeenCalledWith(1)
  })

  // 转让车主的候选里不能出现现任车主，否则会转给他自己（后端 400）。
  it('excludes the current owner from transfer candidates', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1 })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'alice', role: 'owner', declaredWeeklyQuotaUsd: 600 },
      { userId: 2, username: 'bob', role: 'member', declaredWeeklyQuotaUsd: 400 },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.transfer').trigger('click')
    await flushPromises()

    const options = wrapper.findAll('option').map((o) => o.text())
    expect(options).toContain('bob')
    expect(options).not.toContain('alice')
  })

  // 自定义规则车不走申报制：后端会拒代改申报，前端干脆不渲染这个按钮；
  // 但移出成员（清理历史名册）仍然可用。
  it('hides quota editing for custom-rule carpools but still allows removal', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'recruiting', pricingModel: 'custom' })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'alice', role: 'owner', declaredWeeklyQuotaUsd: 0 },
      { userId: 2, username: 'bob', role: 'member', declaredWeeklyQuotaUsd: 0 },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.adminPage.actions.editQuota'))).toBe(false)
    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.adminPage.actions.remove'))).toBe(true)
  })

  // 编辑保存必须把日期按 YYYY-MM-DD 传给后端：后端的 time.Parse 只认这个口径。
  it('saves edits with the scheduled start as a YYYY-MM-DD string', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'recruiting', scheduledStartAt: '2026-08-01' })])
    updateCarpoolMock.mockResolvedValue(makeCarpool())

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.edit').trigger('click')
    await flushPromises()
    await findButton(wrapper, 'common.save').trigger('click')
    await flushPromises()

    expect(updateCarpoolMock).toHaveBeenCalledWith(1, expect.objectContaining({ scheduled_start_at: '2026-08-01' }))
  })

  // 管理员要在成员管理里直接联系到人：后端只给 admin 返回 email，前端要显示出来。
  it('shows member emails in the roster', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'recruiting' })])
    rosterMock.mockResolvedValue([
      { userId: 2, username: 'bob', email: 'bob@test.local', role: 'member', declaredWeeklyQuotaUsd: 400 },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('bob@test.local')
  })

  // 群二维码：打开成员弹窗时现取现渲染，关掉弹窗要吊销 object URL
  // （它是私密车的入场券，不能一直挂在内存里）。
  it('loads the group QR code on open and revokes it on close', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'recruiting', hasGroupQrCode: true })])
    rosterMock.mockResolvedValue([])
    groupQrCodeMock.mockResolvedValue(new Blob(['png'], { type: 'image/png' }))

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    expect(groupQrCodeMock).toHaveBeenCalledWith(1)
    expect(wrapper.find('img').exists()).toBe(true)
    expect(wrapper.find('img').attributes('src')).toBe('blob:mock-qr')

    await findButton(wrapper, 'common.close').trigger('click')
    await flushPromises()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-qr')
  })
})
