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
  createMock,
  addMemberMock,
  usersListMock,
  removeMemberMock,
  updateMemberQuotaMock,
  updateCarpoolMock,
  transferOwnerMock,
  setJoinLockedMock,
  unconfirmMock,
  launchMock,
  cancelMock,
  groupQrCodeMock,
  replaceGroupQrCodeMock,
} = vi.hoisted(() => ({
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
  showError: vi.fn(),
  adminOverviewMock: vi.fn(),
  rosterMock: vi.fn(),
  createMock: vi.fn(),
  addMemberMock: vi.fn(),
  usersListMock: vi.fn(),
  removeMemberMock: vi.fn(),
  updateMemberQuotaMock: vi.fn(),
  updateCarpoolMock: vi.fn(),
  transferOwnerMock: vi.fn(),
  setJoinLockedMock: vi.fn(),
  unconfirmMock: vi.fn(),
  launchMock: vi.fn(),
  cancelMock: vi.fn(),
  groupQrCodeMock: vi.fn(),
  replaceGroupQrCodeMock: vi.fn(),
}))

vi.mock('@/api/carpools', () => ({
  default: {
    adminOverview: adminOverviewMock,
    roster: rosterMock,
    create: createMock,
    addMember: addMemberMock,
    removeMember: removeMemberMock,
    updateMemberQuota: updateMemberQuotaMock,
    updateCarpool: updateCarpoolMock,
    transferOwner: transferOwnerMock,
    setJoinLocked: setJoinLockedMock,
    unconfirm: unconfirmMock,
    launch: launchMock,
    cancel: cancelMock,
    groupQrCode: groupQrCodeMock,
    replaceGroupQrCode: replaceGroupQrCodeMock,
  },
}))

// 成员编辑器的远程用户搜索（创建对话框与成员弹窗共用同一个 usersAPI.list）
vi.mock('@/api/admin/users', () => ({
  default: {
    list: usersListMock,
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
    carType: 2,
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

// Select 桩不会自己发 update:modelValue（真实件是自定义下拉）。按候选值定位到目标桩
// 直接 emit——v-model 的监听器挂在 vnode props 上，emit 即可回写父组件（同 KeysView.spec）。
async function selectOption(wrapper: ReturnType<typeof mountView>, optionValues: unknown[], value: unknown) {
  const select = wrapper.findAllComponents({ name: 'Select' }).find((s) => {
    const values = (s.props('options') as { value: unknown }[]).map((o) => o.value)
    return optionValues.every((v) => values.includes(v))
  })
  if (!select) throw new Error(`select not found for options: ${optionValues.join(', ')}`)
  await select.vm.$emit('update:modelValue', value)
  await flushPromises()
}

// 打开「手动创建车辆」对话框；openCreate 会立刻搜一次空串，flush 后用户候选就位。
async function openCreateDialog(wrapper: ReturnType<typeof mountView>) {
  await wrapper.get('[data-testid="open-create-carpool"]').trigger('click')
  await flushPromises()
}

// 创建对话框的群二维码走 FileReader（宏任务），flushPromises 一轮不一定等得到。
async function pickCreateQr(wrapper: ReturnType<typeof mountView>) {
  const input = wrapper.find('#admin-create-qr')
  const file = new File(['x'], 'qr.png', { type: 'image/png' })
  Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
  await input.trigger('change')
  await new Promise((r) => setTimeout(r, 20))
  await flushPromises()
}

// 创建对话框里把用户加进暂存列表：quota 车（2/3）必须带申报与代录风险勾选，否则按钮禁用。
async function stageCreateMember(wrapper: ReturnType<typeof mountView>, userId: number, declaredPercent?: number) {
  await selectOption(wrapper, [42, 43], userId)
  if (declaredPercent !== undefined) {
    await wrapper.find('#create-member-declared').setValue(declaredPercent)
    await wrapper.find('#create-member-risk').setValue(true)
  }
  await wrapper.get('[data-testid="create-member-add"]').trigger('click')
  await flushPromises()
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

  // 车型标签：type 1/2/3 的车在名称列有可读标签；type 0 已由「自定义规则」badge 覆盖，不重复标注。
  it('shows a readable car-type badge on each row', async () => {
    adminOverviewMock.mockResolvedValue([
      makeCarpool({ id: 1, name: 'car-quota', carType: 2 }),
      makeCarpool({ id: 2, name: 'car-new', carType: 3 }),
      makeCarpool({ id: 3, name: 'car-custom', carType: 0, pricingModel: 'custom' }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.carTypes.type2')
    expect(wrapper.text()).toContain('carpool.carTypes.type3')
    expect(wrapper.text()).not.toContain('carpool.carTypes.type0')
    expect(wrapper.text()).toContain('carpool.customRule.badge')
  })

  // 新 quota 车（type 3）的成员管理弹窗要能看到每个人的风险确认状态（只读）。
  it('shows each member risk acknowledgment state for type-3 cars', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, carType: 3, status: 'recruiting' })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'alice', role: 'owner', declaredWeeklyQuotaUsd: 1400, acknowledgedRisk: true },
      { userId: 2, username: 'bob', role: 'member', declaredWeeklyQuotaUsd: 700, acknowledgedRisk: false },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.adminPage.membersDialog.riskAcked')
    expect(wrapper.text()).toContain('carpool.adminPage.membersDialog.riskNotAcked')
  })

  // 其他车型的成员不渲染风险确认标记——该字段只对 type 3 有意义。
  it('hides the risk acknowledgment badge for non-type-3 cars', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, carType: 2, status: 'recruiting' })])
    rosterMock.mockResolvedValue([
      { userId: 2, username: 'bob', role: 'member', declaredWeeklyQuotaUsd: 400, acknowledgedRisk: false },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('carpool.adminPage.membersDialog.riskAcked')
    expect(wrapper.text()).not.toContain('carpool.adminPage.membersDialog.riskNotAcked')
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

describe('cancel active and QR replace', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    adminOverviewMock.mockResolvedValue([])
    rosterMock.mockResolvedValue([])
    URL.createObjectURL = vi.fn(() => 'blob:mock-qr')
    URL.revokeObjectURL = vi.fn()
  })

  // 已发车的车也要能取消（后端仅放行 admin，并会软删全员订阅）——
  // 但确认文案必须单独说透「订阅立即失效、不可恢复」。
  it('offers cancel for an active carpool with a stronger warning', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'active' })])
    cancelMock.mockResolvedValue(undefined)

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.cancel').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.adminPage.confirm.cancelActive.message')

    await findButton(wrapper, 'common.confirm').trigger('click')
    await flushPromises()
    expect(cancelMock).toHaveBeenCalledWith(1)
  })

  // 管理员在成员弹窗里换群二维码：换完要重取（旧的 object URL 吊销）。
  it('replaces the group QR code from the members dialog', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, status: 'recruiting', hasGroupQrCode: true })])
    rosterMock.mockResolvedValue([])
    groupQrCodeMock.mockResolvedValue(new Blob(['png'], { type: 'image/png' }))
    replaceGroupQrCodeMock.mockResolvedValue(undefined)

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    const input = wrapper.find('input[type="file"]')
    const file = new File(['x'], 'qr.png', { type: 'image/png' })
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    // FileReader 的 onload 是宏任务，flushPromises 一轮不一定等得到
    await new Promise((r) => setTimeout(r, 20))
    await flushPromises()

    expect(replaceGroupQrCodeMock).toHaveBeenCalledWith(1, expect.stringMatching(/^data:image\/png/))
    // 打开弹窗取了一次，换完又重取了一次
    expect(groupQrCodeMock).toHaveBeenCalledTimes(2)
  })
})

describe('manual create and add member', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    adminOverviewMock.mockResolvedValue([])
    rosterMock.mockResolvedValue([])
    usersListMock.mockResolvedValue({
      items: [
        { id: 42, username: 'bob', email: 'bob@test.local' },
        { id: 43, username: 'carol', email: 'carol@test.local' },
      ],
      total: 2,
      page: 1,
      page_size: 10,
      pages: 1,
    })
    createMock.mockResolvedValue({ carpool: makeCarpool({ id: 99 }), inviteToken: '' })
    addMemberMock.mockResolvedValue({ carpool: makeCarpool(), autoUnconfirmed: false })
  })

  // 车型决定创建表单：type 3（quota 车）要发车日/可见性/二维码，成员编辑器带申报+风险；
  // type 1 只剩周限额，type 0 只剩规则说明，申报栏同样只属 quota 车。
  it('switches the create dialog fields with the selected car type', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openCreateDialog(wrapper)

    // 默认 type 3
    expect(wrapper.find('#admin-create-start').exists()).toBe(true)
    expect(wrapper.find('#admin-create-visibility').exists()).toBe(true)
    expect(wrapper.find('#admin-create-qr').exists()).toBe(true)
    expect(wrapper.find('#create-member-declared').exists()).toBe(true)
    expect(wrapper.find('#create-member-risk').exists()).toBe(true)
    expect(wrapper.find('#admin-create-weekly-limit').exists()).toBe(false)
    expect(wrapper.find('#admin-create-rule-note').exists()).toBe(false)

    await selectOption(wrapper, [3, 2, 1, 0], 1)
    expect(wrapper.find('#admin-create-weekly-limit').exists()).toBe(true)
    expect(wrapper.find('#admin-create-start').exists()).toBe(false)
    expect(wrapper.find('#admin-create-qr').exists()).toBe(false)
    expect(wrapper.find('#create-member-declared').exists()).toBe(false)
    expect(wrapper.find('#create-member-risk').exists()).toBe(false)
    expect(wrapper.find('#admin-create-rule-note').exists()).toBe(false)

    await selectOption(wrapper, [3, 2, 1, 0], 0)
    expect(wrapper.find('#admin-create-rule-note').exists()).toBe(true)
    expect(wrapper.find('#admin-create-weekly-limit').exists()).toBe(false)
  })

  // type 3 提交：先按 car_type=3 创建车，再把暂存成员逐个代加；申报口径是百分比，
  // 按默认周限额 2400 换算成美元（50% → 1200）。
  it('creates a type-3 carpool, then adds staged members with percent converted to USD', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openCreateDialog(wrapper)

    await wrapper.find('#admin-create-name').setValue('manual-quota')
    await wrapper.find('#admin-create-start').setValue('2026-09-01')
    await pickCreateQr(wrapper)
    await stageCreateMember(wrapper, 42, 50)

    const staged = wrapper.get('[data-testid="create-staged-members"]').text()
    expect(staged).toContain('bob（bob@test.local）')
    expect(staged).toContain('50%')

    await wrapper.get('[data-testid="create-submit"]').trigger('click')
    await flushPromises()

    expect(createMock).toHaveBeenCalledTimes(1)
    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'manual-quota',
      car_type: 3,
      scheduled_start_at: '2026-09-01',
      added_admin_wechat: true,
      acknowledged_risk: true,
      group_qr_code: expect.stringMatching(/^data:image\/png/),
    }))
    expect(addMemberMock).toHaveBeenCalledTimes(1)
    expect(addMemberMock).toHaveBeenCalledWith(99, {
      user_id: 42,
      declared_weekly_quota_usd: 1200,
      acknowledged_risk: true,
    })
    expect(showSuccess).toHaveBeenCalledWith('carpool.adminPage.createDialog.success')
    // 提交成功后对话框关闭、总览重新加载
    expect(wrapper.find('[data-testid="create-submit"]').exists()).toBe(false)
    expect(adminOverviewMock).toHaveBeenCalledTimes(2)
  })

  // type 1（无保底老车）：payload 带 weekly_limit_usd；成员只选人，
  // addMember 请求体里不能出现申报/风险字段（后端按车型分别校验）。
  it('creates a type-1 carpool whose staged members carry no declaration', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openCreateDialog(wrapper)

    await selectOption(wrapper, [3, 2, 1, 0], 1)
    await wrapper.find('#admin-create-name').setValue('legacy-car')
    await wrapper.find('#admin-create-weekly-limit').setValue(1200)
    await stageCreateMember(wrapper, 43)

    const staged = wrapper.get('[data-testid="create-staged-members"]').text()
    expect(staged).toContain('carol（carol@test.local）')
    expect(staged).not.toContain('$')

    await wrapper.get('[data-testid="create-submit"]').trigger('click')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'legacy-car',
      car_type: 1,
      weekly_limit_usd: 1200,
      group_qr_code: '',
      // type 1 创建即 active，发车日无意义，前端补一个合法日期
      scheduled_start_at: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
    }))
    expect(addMemberMock).toHaveBeenCalledWith(99, { user_id: 43 })
    expect(showSuccess).toHaveBeenCalledWith('carpool.adminPage.createDialog.success')
  })

  // 初始成员逐个代加：某个失败不能中断后面的；结束时点名警告而不是报成功。
  it('keeps adding the remaining staged members when one fails and warns instead of success', async () => {
    addMemberMock.mockRejectedValueOnce(new Error('boom'))

    const wrapper = mountView()
    await flushPromises()
    await openCreateDialog(wrapper)

    await wrapper.find('#admin-create-name').setValue('manual-quota')
    await wrapper.find('#admin-create-start').setValue('2026-09-01')
    await pickCreateQr(wrapper)
    await stageCreateMember(wrapper, 42, 50)
    await stageCreateMember(wrapper, 43, 25)

    await wrapper.get('[data-testid="create-submit"]').trigger('click')
    await flushPromises()

    // 第一个失败后第二个照样调
    expect(addMemberMock).toHaveBeenCalledTimes(2)
    expect(addMemberMock).toHaveBeenNthCalledWith(1, 99, {
      user_id: 42,
      declared_weekly_quota_usd: 1200,
      acknowledged_risk: true,
    })
    expect(addMemberMock).toHaveBeenNthCalledWith(2, 99, {
      user_id: 43,
      declared_weekly_quota_usd: 600,
      acknowledged_risk: true,
    })
    expect(showWarning).toHaveBeenCalledWith('carpool.adminPage.createDialog.membersFailed')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="create-submit"]').exists()).toBe(false)
  })

  // 成员弹窗添加区（type 3）：缺申报或缺风险勾选时提交禁用；补齐后申报百分比
  // 按车的周限额换算（25% of 2400 = 600），成功后刷新总览与花名册。
  it('requires declaration and risk acknowledgment when adding a member to a type-3 carpool', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, carType: 3, status: 'recruiting' })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'alice', role: 'owner', declaredWeeklyQuotaUsd: 1200, acknowledgedRisk: true },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    const submit = () => wrapper.get('[data-testid="add-member-submit"]')
    await selectOption(wrapper, [42, 43], 42)
    expect(submit().attributes('disabled')).toBeDefined()

    await wrapper.find('#add-member-declared').setValue(25)
    expect(submit().attributes('disabled')).toBeDefined()

    await wrapper.find('#add-member-risk').setValue(true)
    expect(submit().attributes('disabled')).toBeUndefined()

    await submit().trigger('click')
    await flushPromises()

    expect(addMemberMock).toHaveBeenCalledWith(1, {
      user_id: 42,
      declared_weekly_quota_usd: 600,
      acknowledged_risk: true,
    })
    expect(showSuccess).toHaveBeenCalledWith('carpool.adminPage.addMember.success')
    expect(rosterMock).toHaveBeenCalledTimes(2)
    expect(adminOverviewMock).toHaveBeenCalledTimes(2)
  })

  // type 1（active 老车）的添加区只有选人：不渲染申报/风险，添加即生效并刷新花名册。
  it('adds a member to a type-1 carpool with only a user pick', async () => {
    adminOverviewMock.mockResolvedValue([makeCarpool({ id: 1, carType: 1, status: 'active' })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'alice', role: 'owner', declaredWeeklyQuotaUsd: 0 },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.adminPage.actions.members').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="add-member-section"]').exists()).toBe(true)
    expect(wrapper.find('#add-member-declared').exists()).toBe(false)
    expect(wrapper.find('#add-member-risk').exists()).toBe(false)

    await selectOption(wrapper, [42, 43], 42)
    expect(wrapper.get('[data-testid="add-member-submit"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-testid="add-member-submit"]').trigger('click')
    await flushPromises()

    expect(addMemberMock).toHaveBeenCalledWith(1, { user_id: 42 })
    expect(showSuccess).toHaveBeenCalledWith('carpool.adminPage.addMember.success')
    expect(rosterMock).toHaveBeenCalledTimes(2)
  })
})
