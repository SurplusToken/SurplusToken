import { ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CarpoolView from '../CarpoolView.vue'

const {
  replace,
  showSuccess,
  showWarning,
  listCarpools,
  createCarpoolMock,
  joinMock,
  joinByInviteMock,
  launchMock,
  leaveMock,
  confirmMock,
  groupQrCodeMock,
  replaceGroupQrCodeMock,
  rosterMock,
  recommendationMock,
  notifyCustomRuleInterestMock,
  unconfirmMock,
  pendingLaunchMock,
  createInviteMock,
  settlementMock,
  settleMock,
  unsettleMock,
  updateMemberQuotaMock,
  authState,
} = vi.hoisted(() => ({
  replace: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
  listCarpools: vi.fn(),
  createCarpoolMock: vi.fn(),
  joinMock: vi.fn(),
  joinByInviteMock: vi.fn(),
  launchMock: vi.fn(),
  leaveMock: vi.fn(),
  confirmMock: vi.fn(),
  groupQrCodeMock: vi.fn(),
  replaceGroupQrCodeMock: vi.fn(),
  rosterMock: vi.fn(),
  recommendationMock: vi.fn(),
  notifyCustomRuleInterestMock: vi.fn(),
  unconfirmMock: vi.fn(),
  pendingLaunchMock: vi.fn(),
  createInviteMock: vi.fn(),
  settlementMock: vi.fn(),
  settleMock: vi.fn(),
  unsettleMock: vi.fn(),
  updateMemberQuotaMock: vi.fn(),
  authState: { isAdmin: false },
}))

vi.mock('@/api/carpools', () => ({
  default: {
    list: listCarpools,
    create: createCarpoolMock,
    resolveInvite: vi.fn(),
    createInvite: createInviteMock,
    join: joinMock,
    joinByInvite: joinByInviteMock,
    leave: leaveMock,
    confirm: confirmMock,
    unconfirm: unconfirmMock,
    pendingLaunch: pendingLaunchMock,
    groupQrCode: groupQrCodeMock,
    replaceGroupQrCode: replaceGroupQrCodeMock,
    roster: rosterMock,
    launch: launchMock,
    declarationRecommendation: recommendationMock,
    notifyCustomRuleInterest: notifyCustomRuleInterestMock,
    settlement: settlementMock,
    settle: settleMock,
    unsettle: unsettleMock,
    updateMemberQuota: updateMemberQuotaMock,
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
    showSuccess,
    showError: vi.fn(),
    showWarning,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isAdmin: authState.isAdmin,
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
    carType: 2,
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
    adminWechat: 'Charlemartingale',
    hasGroupQrCode: false,
    launchNotifiedAt: undefined,
    confirmedAt: undefined,
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
    pricingModel: 'quota',
    ruleNote: '',
    ...overrides,
  }
}

function mountView() {
  return shallowMount(CarpoolView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        // shallowMount 会把 Teleport 桩成空壳，放大层需要它渲染出插槽内容
        Teleport: { template: '<div class="teleport-stub"><slot /></div>' },
        BaseDialog: {
          name: 'BaseDialog',
          props: ['show', 'title', 'width'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        ConfirmDialog: {
          name: 'ConfirmDialog',
          props: ['show', 'title', 'message', 'confirmText', 'cancelText', 'danger'],
          emits: ['confirm', 'cancel'],
          template:
            '<div v-if="show" data-testid="confirm-dialog">' +
            '<p data-testid="confirm-dialog-message">{{ message }}</p>' +
            '<button data-testid="confirm-dialog-ok" @click="$emit(\'confirm\')">ok</button>' +
            '</div>',
        },
        Icon: true,
      },
    },
  })
}

function findButton(wrapper: ReturnType<typeof mountView>, text: string) {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) throw new Error(`button not found: ${text}`)
  return button
}

describe('CarpoolView', () => {
  beforeEach(() => {
    localStorage.clear()
    replace.mockReset()
    showSuccess.mockReset()
    showWarning.mockReset()
    listCarpools.mockReset()
    listCarpools.mockResolvedValue([])
    createCarpoolMock.mockReset()
    joinMock.mockReset()
    joinByInviteMock.mockReset()
    launchMock.mockReset()
    leaveMock.mockReset()
    confirmMock.mockReset()
    groupQrCodeMock.mockReset()
    rosterMock.mockReset()
    rosterMock.mockResolvedValue([])
    groupQrCodeMock.mockResolvedValue(new Blob(['qr'], { type: 'image/png' }))
    recommendationMock.mockReset()
    recommendationMock.mockResolvedValue({
      recommendedWeeklyQuotaUsd: 0,
      rawWeeklyUsageUsd: 0,
      bufferRatio: 0,
      daysWithRecords: 0,
      basis: 'usage_history',
      message: '',
    })
    notifyCustomRuleInterestMock.mockReset()
    notifyCustomRuleInterestMock.mockResolvedValue(undefined)
    unconfirmMock.mockReset()
    pendingLaunchMock.mockReset()
    pendingLaunchMock.mockResolvedValue([])
    createInviteMock.mockReset()
    settlementMock.mockReset()
    settleMock.mockReset()
    unsettleMock.mockReset()
    updateMemberQuotaMock.mockReset()
    authState.isAdmin = false
    URL.createObjectURL = vi.fn(() => 'blob:qr') as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn() as typeof URL.revokeObjectURL
  })

  it('shows the quota reservation rules and does not seed any car', async () => {
    const wrapper = mountView()
    await flushPromises()
    const rules = wrapper.get('[data-testid="gpt-carpool-rules"]')

    expect(rules.text()).toContain('carpool.rules.declare.text')
    expect(rules.text()).toContain('carpool.rules.reserve.text')
    expect(rules.text()).toContain('carpool.rules.pricing.text')
    // 风险说明已从规则条目下移到 notices 区
    expect(rules.text()).not.toContain('carpool.rules.risk')
    expect(rules.text()).toContain('carpool.notices.weeklyRefresh')
    expect(rules.text()).toContain('carpool.notices.consumeOrder')
    expect(rules.text()).toContain('carpool.notices.risk')
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
    expect(card.text()).toContain('carpool.fields.carMonthlyFee')
    expect(card.text()).not.toContain('carpool.fields.seatsRemaining')
    // 等效倍率与关联分组已从用户侧卡片移除
    expect(card.text()).not.toContain('carpool.fields.effectiveRate')
    expect(card.text()).not.toContain('carpool.detailDialog.linkedGroup')
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

  it('shows the owner an enabled confirm button inside the band and a disabled one below the line', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 20, name: 'ready-car', memberRole: 'owner', declaredTotalUsd: 2350, remainingJoinableUsd: 170 }),
      makeCarpool({ id: 21, name: 'early-car', memberRole: 'owner', declaredTotalUsd: 960, remainingJoinableUsd: 1560 }),
    ])

    const wrapper = mountView()
    await flushPromises()

    const readyButton = wrapper.get('[data-testid="confirm-20"]')
    expect(readyButton.attributes('disabled')).toBeUndefined()

    const earlyButton = wrapper.get('[data-testid="confirm-21"]')
    expect(earlyButton.attributes('disabled')).toBeDefined()

    // owner 不再有直接发车入口
    expect(wrapper.find('[data-testid="launch-20"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="force-launch-20"]').exists()).toBe(false)
  })

  it('shows a leave button to joined members instead of the confirm button', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 22, memberRole: 'member', declaredTotalUsd: 2350 }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="confirm-22"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="leave-22"]').exists()).toBe(true)
  })

  it('lets a member leave after a confirmation that warns about quota release', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 22, memberRole: 'member' })])
    leaveMock.mockResolvedValue(makeCarpool({ id: 22 }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="leave-22"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm-dialog-message"]').text()).toContain('carpool.leaveDialog.message')

    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()
    expect(leaveMock).toHaveBeenCalledWith(22)
  })

  it('lets the owner confirm the launch inside the band', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 20, memberRole: 'owner', declaredTotalUsd: 2350, remainingJoinableUsd: 170 }),
    ])
    confirmMock.mockResolvedValue(makeCarpool({ id: 20, status: 'confirmed', joinLocked: true }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="confirm-20"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm-dialog-message"]').text()).toContain('carpool.confirmDialog.message')

    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()
    expect(confirmMock).toHaveBeenCalledWith(20)
  })

  it('shows admins a launch button on confirmed carpools with the pending badge', async () => {
    authState.isAdmin = true
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 30, status: 'confirmed', joinLocked: true, confirmedAt: '2026-07-20T00:00:00Z' }),
    ])
    launchMock.mockResolvedValue(makeCarpool({ id: 30, status: 'active' }))

    const wrapper = mountView()
    await flushPromises()
    const card = wrapper.get('article')

    expect(card.text()).toContain('carpool.status.confirmed')
    await wrapper.get('[data-testid="launch-30"]').trigger('click')
    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()
    expect(launchMock).toHaveBeenCalledWith(30, false)
  })

  it('keeps the force launch entry for admins above 80% on recruiting carpools', async () => {
    authState.isAdmin = true
    listCarpools.mockResolvedValue([makeCarpool({ id: 31, declaredTotalUsd: 2000, remainingJoinableUsd: 520 })])
    launchMock.mockResolvedValue(makeCarpool({ id: 31, status: 'active' }))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="force-launch-31"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm-dialog-message"]').text()).toContain('carpool.launchDialog.forceMessage')

    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()
    expect(launchMock).toHaveBeenCalledWith(31, true)
  })

  it('requires the admin wechat confirmation and a group qr code before creating', async () => {
    createCarpoolMock.mockResolvedValue({ carpool: makeCarpool({ id: 40 }), inviteToken: 'token-40' })

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.create').trigger('click')
    const submit = () => wrapper.get('button[form="carpool-create-form"]')

    // 名称/日期填好，但两项确认未完成 → 禁提交
    await wrapper.get('#carpool-name').setValue('weekend-car')
    expect(submit().attributes('disabled')).toBeDefined()

    // 只勾选已添加管理员微信，缺二维码 → 仍禁提交
    await wrapper.get('#carpool-added-admin').setValue(true)
    expect(submit().attributes('disabled')).toBeDefined()

    const file = new File([new Uint8Array([137, 80, 78, 71])], 'qr.png', { type: 'image/png' })
    const qrInput = wrapper.get('#carpool-group-qr')
    Object.defineProperty(qrInput.element, 'files', { value: [file], configurable: true })
    await qrInput.trigger('change')
    await flushPromises()
    // FileReader.onload 异步完成后再等一拍
    await new Promise((resolve) => setTimeout(resolve, 0))
    // 缺风险确认 → 仍禁提交
    expect(submit().attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-create-risk').setValue(true)
    // 缺"已建群并拉管理员入群" → 仍禁提交
    expect(submit().attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-created-group').setValue(true)
    expect(submit().attributes('disabled')).toBeUndefined()

    await wrapper.get('#carpool-create-form').trigger('submit')
    await flushPromises()

    expect(createCarpoolMock).toHaveBeenCalledOnce()
    const payload = createCarpoolMock.mock.calls[0][0] as Record<string, unknown>
    expect(payload.added_admin_wechat).toBe(true)
    expect(payload.acknowledged_risk).toBe(true)
    expect(String(payload.group_qr_code)).toMatch(/^data:image\/png/)
    // 高级设置已移除：池参数不再由前端提交
    expect(payload.weekly_limit_usd).toBeUndefined()
    expect(payload.seat_fee_cny).toBeUndefined()
  })

  // 创建对话框的保底/预付预览（仅申报 > 0 时显示）：按发起人自己的申报份额预估
  // （整车打满口径），新建车恒为 type 3——保底 = 80%×申报；预付 = ¥50 席位 + 0.8×¥1200×份额。
  it('previews floor and prepaid for the owner declaration in the create dialog', async () => {
    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.create').trigger('click')

    const preview = () => wrapper.find('[data-testid="create-preview"]')
    // 未申报时不显示预览与保底提示
    expect(preview().exists()).toBe(false)
    expect(wrapper.text()).not.toContain('carpool.joinDialog.floorNotice')

    await wrapper.get('#carpool-owner-quota').setValue(50)
    // 50% × $2,400 = $1,200：保底 = 80%×1200 = $960；预付 = ¥50 + 0.8×¥1200×50% = ¥530
    expect(preview().exists()).toBe(true)
    expect(preview().text()).toContain('$960')
    expect(preview().text()).toContain('¥530')
    expect(preview().text()).toContain('¥50')
    expect(preview().text()).toContain('¥480')
    expect(wrapper.text()).toContain('carpool.createDialog.previewOnePersonNote')
    expect(wrapper.text()).toContain('carpool.joinDialog.floorNotice')
  })

  // 创建即上车：风险确认是硬门禁，勾选后提交带 acknowledged_risk 与换算后的美元申报。
  it('requires the risk acknowledgment and submits it with the converted declaration', async () => {
    createCarpoolMock.mockResolvedValue({ carpool: makeCarpool({ id: 40 }), inviteToken: 'token-40' })

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.create').trigger('click')
    const submit = () => wrapper.get('button[form="carpool-create-form"]')

    await wrapper.get('#carpool-name').setValue('weekend-car')
    await wrapper.get('#carpool-added-admin').setValue(true)
    await wrapper.get('#carpool-created-group').setValue(true)
    await wrapper.get('#carpool-owner-quota').setValue(50)
    const file = new File([new Uint8Array([137, 80, 78, 71])], 'qr.png', { type: 'image/png' })
    const qrInput = wrapper.get('#carpool-group-qr')
    Object.defineProperty(qrInput.element, 'files', { value: [file], configurable: true })
    await qrInput.trigger('change')
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 0))

    // 未勾选风险确认 → 禁提交
    expect(submit().attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-create-risk').setValue(true)
    expect(submit().attributes('disabled')).toBeUndefined()

    await wrapper.get('#carpool-create-form').trigger('submit')
    await flushPromises()

    expect(createCarpoolMock).toHaveBeenCalledOnce()
    const payload = createCarpoolMock.mock.calls[0][0] as Record<string, unknown>
    expect(payload.acknowledged_risk).toBe(true)
    // 50% × $2,400 = $1,200
    expect(payload.declared_weekly_quota_usd).toBe(1200)
  })


  it('rejects an oversized group qr code in the create dialog', async () => {
    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.create').trigger('click')
    const bigFile = new File([new Uint8Array(2 * 1024 * 1024 + 1)], 'big.png', { type: 'image/png' })
    const qrInput = wrapper.get('#carpool-group-qr')
    Object.defineProperty(qrInput.element, 'files', { value: [bigFile], configurable: true })
    await qrInput.trigger('change')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.createDialog.qrTooLarge')
    expect(wrapper.get('button[form="carpool-create-form"]').attributes('disabled')).toBeDefined()
  })

  it('requires the group join confirmation before boarding', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, hasGroupQrCode: true })])
    joinMock.mockResolvedValue({ carpool: makeCarpool({ id: 10, memberRole: 'member' }), prepaidAmountCny: 0 })

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    // 对话框展示群二维码与管理员微信
    expect(wrapper.text()).toContain('carpool.joinDialog.groupSection')
    expect(wrapper.text()).toContain('Charlemartingale')
    expect(groupQrCodeMock).toHaveBeenCalledWith(10)

    const confirmButton = () => findButton(wrapper, 'carpool.joinDialog.confirm')
    await wrapper.get('#carpool-join-quota').setValue(100)
    // 未勾选"我已加入群聊" → 禁提交
    expect(confirmButton().attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-join-group').setValue(true)
    expect(confirmButton().attributes('disabled')).toBeUndefined()

    await confirmButton().trigger('click')
    await flushPromises()
    // type 2 车不带风险确认（第三个参数为 false，不会写进请求体）
    expect(joinMock).toHaveBeenCalledWith(10, 100, false)
  })

  // 申报推荐是异步回填金额输入框的。迟到的响应绝不能覆盖用户已经改过的数字——
  // 否则用户会按一个自己没同意的额度上车，而这个额度直接决定保底和扣款。
  it('never overwrites a quota the user already typed with a late recommendation', async () => {
    let resolveRecommendation: (value: unknown) => void = () => {}
    recommendationMock.mockReturnValue(new Promise((resolve) => { resolveRecommendation = resolve }))
    listCarpools.mockResolvedValue([makeCarpool({ id: 10 })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    // 用户在推荐回来之前先自己填了金额
    await wrapper.get('#carpool-join-quota').setValue(250)

    resolveRecommendation({
      recommendedWeeklyQuotaUsd: 500,
      rawWeeklyUsageUsd: 450,
      bufferRatio: 1.1,
      daysWithRecords: 7,
      basis: 'usage_history',
      message: '',
    })
    await flushPromises()

    expect((wrapper.get('#carpool-join-quota').element as HTMLInputElement).value).toBe('250')
  })

  // 申报下限：低于下限时禁止提交并给出提示（后端也会拒，这里是即时反馈）。
  it('blocks joining with a declaration below the floor', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10 })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    await wrapper.get('#carpool-join-quota').setValue(5)
    await wrapper.get('#carpool-join-group').setValue(true)

    expect(wrapper.text()).toContain('carpool.joinDialog.belowFloor')
    expect(findButton(wrapper, 'carpool.joinDialog.confirm').attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-join-quota').setValue(20)
    expect(findButton(wrapper, 'carpool.joinDialog.confirm').attributes('disabled')).toBeUndefined()
  })

  // type 3（新 quota 车）：周限额 $2,400、席位费 ¥50/月、额度池 ¥1200/月，卡片按车型参数展示。
  it('shows the type-3 car parameters on the card', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10,
      carType: 3,
      weeklyLimitUsd: 2400,
      seatFeeCny: 50,
      usagePoolCny: 1200,
      declaredTotalUsd: 1200,
      remainingJoinableUsd: 1320,
    })])

    const wrapper = mountView()
    await flushPromises()
    const card = wrapper.get('article')

    expect(card.text()).toContain('$2,400')
    expect(card.text()).toContain('¥50')
    expect(card.text()).toContain('¥1,200')
    // 席位费是每人固定 ¥50：不能渲染出"席位+用量=¥1,250"这种整车误导合计
    expect(card.text()).not.toContain('¥1,250')
    // 风险说明在规则区的 notices 里
    expect(wrapper.text()).toContain('carpool.notices.risk')
  })

  // type 3 加入对话框：申报改为占全车额度的百分比，实时换算美元；
  // 必须勾选风险确认才能提交，提交带换算后的美元申报与 acknowledged_risk。
  it('joins a type-3 car with a percentage declaration and the risk acknowledgment', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10,
      carType: 3,
      weeklyLimitUsd: 2400,
      seatFeeCny: 50,
      usagePoolCny: 1200,
      memberCount: 2,
      declaredTotalUsd: 1200,
      remainingJoinableUsd: 1320,
    })])
    rosterMock.mockResolvedValue([
      { userId: 9, username: 'owner-a', role: 'owner', declaredWeeklyQuotaUsd: 1200, acknowledgedRisk: true },
    ])
    joinMock.mockResolvedValue({ carpool: makeCarpool({ id: 10, memberRole: 'member' }), prepaidAmountCny: 0 })

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    // 百分比口径的输入标签 + 风险确认勾选框
    expect(wrapper.text()).toContain('carpool.joinDialog.quotaLabelPercent')
    expect(wrapper.find('#carpool-join-risk').exists()).toBe(true)

    await wrapper.get('#carpool-join-quota').setValue(50)
    // 50% × $2,400 = $1,200：花名册"你"一行按美元显示；保底 80% = $960
    expect(wrapper.text()).toContain('carpool.joinDialog.quotaPercentEquals')
    expect(wrapper.text()).toContain('$1,200')
    expect(wrapper.text()).toContain('$960')
    // 席位费每人固定 ¥50（不均摊），提示走 per-person 口径
    expect(wrapper.text()).toContain('carpool.joinDialog.seatSharePerPerson')
    expect(wrapper.text()).not.toContain('carpool.joinDialog.seatShareHint')
    // 预付 = 席位 ¥50（固定） + 80% × ¥1200 × (1200/2400) = ¥530
    expect(wrapper.text()).toContain('¥530')

    const confirmButton = () => findButton(wrapper, 'carpool.joinDialog.confirm')
    await wrapper.get('#carpool-join-group').setValue(true)
    // 未勾选风险确认 → 禁提交
    expect(confirmButton().attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-join-risk').setValue(true)
    expect(confirmButton().attributes('disabled')).toBeUndefined()

    await confirmButton().trigger('click')
    await flushPromises()
    // 提交的是换算后的美元申报，且带风险确认
    expect(joinMock).toHaveBeenCalledWith(10, 1200, true)
  })

  // type 3 用量池预付按"占整车周限额的份额"计：车上申报很少时（车主 0 申报、Σ=0），
  // 不能把 80% 池子几乎全算到新上车的人头上（旧口径会显示 ¥960/¥1,010）。
  it('quotes the type-3 pool prepay against the car weekly limit, not the current declaration total', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10,
      carType: 3,
      weeklyLimitUsd: 2400,
      seatFeeCny: 50,
      usagePoolCny: 1200,
      memberCount: 1,
      declaredTotalUsd: 0,
      remainingJoinableUsd: 2520,
    })])
    rosterMock.mockResolvedValue([
      { userId: 9, username: 'owner-a', role: 'owner', declaredWeeklyQuotaUsd: 0, acknowledgedRisk: true },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    // 报 5%（$120）：用量部分 = 0.8×¥1200×(120/2400) = ¥48，预付 = ¥50 席位 + ¥48 = ¥98
    await wrapper.get('#carpool-join-quota').setValue(5)
    expect(wrapper.text()).toContain('¥98')
    expect(wrapper.text()).not.toContain('¥960')
    expect(wrapper.text()).not.toContain('¥1,010')
  })

  // type 3 的申报下限提示用百分比口径：$20 / $2400 ≈ 0.83%。
  it('blocks a type-3 declaration below the floor in percentage terms', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10,
      carType: 3,
      weeklyLimitUsd: 2400,
      seatFeeCny: 50,
      usagePoolCny: 1200,
      declaredTotalUsd: 1200,
      remainingJoinableUsd: 1320,
    })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    await wrapper.get('#carpool-join-quota').setValue(0.5) // 0.5% × 2400 = $12 < $20
    await wrapper.get('#carpool-join-group').setValue(true)
    await wrapper.get('#carpool-join-risk').setValue(true)

    expect(wrapper.text()).toContain('carpool.joinDialog.belowFloorPercent')
    expect(findButton(wrapper, 'carpool.joinDialog.confirm').attributes('disabled')).toBeDefined()

    await wrapper.get('#carpool-join-quota').setValue(1) // 1% = $24 ≥ $20
    expect(findButton(wrapper, 'carpool.joinDialog.confirm').attributes('disabled')).toBeUndefined()
  })

  // type 2（现行 quota 车）不渲染风险确认勾选，交互保持美元申报。
  it('keeps the type-2 join dialog free of the risk checkbox', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10 })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.joinDialog.quotaLabel')
    expect(wrapper.text()).not.toContain('carpool.joinDialog.quotaLabelPercent')
    expect(wrapper.find('#carpool-join-risk').exists()).toBe(false)
  })

  // 招募中的 type 3 车，成员可改自己的申报：百分比输入，预填当前值的百分比口径，
  // 提交时换算成美元调 updateMemberQuota。
  it('lets a member edit their declaration on a recruiting type-3 car', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10,
      carType: 3,
      weeklyLimitUsd: 2400,
      seatFeeCny: 50,
      usagePoolCny: 1200,
      memberRole: 'member',
      memberCount: 3,
      declaredTotalUsd: 1800,
      remainingJoinableUsd: 720,
    })])
    rosterMock.mockResolvedValue([
      { userId: 9, username: 'owner-a', role: 'owner', declaredWeeklyQuotaUsd: 1200, acknowledgedRisk: true },
      { userId: 1, username: 'preview-user', role: 'member', declaredWeeklyQuotaUsd: 600, acknowledgedRisk: true },
    ])
    updateMemberQuotaMock.mockResolvedValue({ carpool: makeCarpool({ id: 10 }), autoUnconfirmed: false })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="edit-quota-10"]').trigger('click')
    await flushPromises()

    // 预填当前申报的百分比口径：$600 / $2,400 = 25%，并展示当前申报
    expect((wrapper.get('#carpool-quota-input').element as HTMLInputElement).value).toBe('25')
    expect(wrapper.text()).toContain('carpool.quotaDialog.current')
    expect(wrapper.text()).toContain('carpool.joinDialog.quotaLabelPercent')

    await wrapper.get('#carpool-quota-input').setValue(50)
    // 50% × $2,400 = $1,200 的实时换算提示
    expect(wrapper.text()).toContain('carpool.joinDialog.quotaPercentEquals')

    await wrapper.get('[data-testid="quota-dialog-submit"]').trigger('click')
    await flushPromises()

    expect(updateMemberQuotaMock).toHaveBeenCalledWith(10, 1, 1200)
    expect(showSuccess).toHaveBeenCalledWith('carpool.quotaDialog.success')
  })

  // 把自己的申报改出 [95%,105%] 发车区间时，后端会把车自动退回招募中——必须明说。
  it('warns when the quota change pushes the car out of the launch band', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10, memberRole: 'member', declaredTotalUsd: 2350, remainingJoinableUsd: 170,
    })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'preview-user', role: 'member', declaredWeeklyQuotaUsd: 400, acknowledgedRisk: false },
    ])
    updateMemberQuotaMock.mockResolvedValue({ carpool: makeCarpool({ id: 10 }), autoUnconfirmed: true })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="edit-quota-10"]').trigger('click')
    await flushPromises()

    // 降申报：$100 ≥ $20 下限，且不超过 当前 $400 + 剩余 $170
    await wrapper.get('#carpool-quota-input').setValue(100)
    await wrapper.get('[data-testid="quota-dialog-submit"]').trigger('click')
    await flushPromises()

    expect(updateMemberQuotaMock).toHaveBeenCalledWith(10, 1, 100)
    expect(showWarning).toHaveBeenCalledWith('carpool.quotaDialog.autoUnconfirmed')
    expect(showSuccess).not.toHaveBeenCalled()
  })

  // 入口只对"招募中且我在车上"显示：非成员、已发车、已封车都不显示。
  it('shows the edit-quota entry only to members of a recruiting car', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'member-car', memberRole: 'member' }),
      makeCarpool({ id: 11, name: 'stranger-car', memberRole: null }),
      makeCarpool({ id: 12, name: 'active-car', memberRole: 'member', status: 'active' }),
      makeCarpool({ id: 13, name: 'locked-car', memberRole: 'member', joinLocked: true }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="edit-quota-10"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="edit-quota-11"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="edit-quota-12"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="edit-quota-13"]').exists()).toBe(false)
  })

  // 车主「仅发起」（申报 0）不显示入口：列表响应不带 viewer 的申报，
  // 前端对"我是车主且招募中"的车补拉花名册确认。
  it('hides the edit-quota entry from an owner who declared nothing', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10, memberRole: 'owner', declaredTotalUsd: 0, remainingJoinableUsd: 2520,
    })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'preview-user', role: 'owner', declaredWeeklyQuotaUsd: 0, acknowledgedRisk: false },
    ])

    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(rosterMock).toHaveBeenCalledWith(10)
    expect(wrapper.find('[data-testid="edit-quota-10"]').exists()).toBe(false)
  })

  // 车主有申报则能改：花名册确认申报 > 0 后显示入口。
  it('shows the edit-quota entry to an owner with a declaration', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10, memberRole: 'owner', declaredTotalUsd: 300, remainingJoinableUsd: 2220,
    })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'preview-user', role: 'owner', declaredWeeklyQuotaUsd: 300, acknowledgedRisk: false },
    ])

    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(wrapper.find('[data-testid="edit-quota-10"]').exists()).toBe(true)
  })

  // type 2 车维持美元输入：标签、预填、提交都不做百分比换算。
  it('edits the declaration in USD on a type-2 car', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, memberRole: 'member' })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'preview-user', role: 'member', declaredWeeklyQuotaUsd: 300, acknowledgedRisk: false },
    ])
    updateMemberQuotaMock.mockResolvedValue({ carpool: makeCarpool({ id: 10 }), autoUnconfirmed: false })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="edit-quota-10"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.joinDialog.quotaLabel')
    expect(wrapper.text()).not.toContain('carpool.joinDialog.quotaLabelPercent')
    // 预填美元原值
    expect((wrapper.get('#carpool-quota-input').element as HTMLInputElement).value).toBe('300')

    await wrapper.get('#carpool-quota-input').setValue(450)
    await wrapper.get('[data-testid="quota-dialog-submit"]').trigger('click')
    await flushPromises()

    expect(updateMemberQuotaMock).toHaveBeenCalledWith(10, 1, 450)
  })

  // 加入对话框的预付预览：合计 + 席位/用量拆分（等效倍率已移除，均价同样不出现）。
  it('previews the join prepaid with its seat and pool breakdown', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, memberCount: 3, avgPriceCny: 90 })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.joinDialog.previewPrepaid')
    expect(wrapper.text()).not.toContain('carpool.joinDialog.previewAvgPrice')

    // 申报 $60：预付 = 400/4 + 80%×1000×60/(1200+60) ≈ ¥138.1。
    await wrapper.get('#carpool-join-quota').setValue(60)
    expect(wrapper.text()).toContain('¥138.1')
    expect(wrapper.text()).toContain('carpool.joinDialog.prepaidBreakdown')
  })

  // 上车前要看得见"车上有几个人、席位费怎么分、别人各报了多少"——
  // 这三件事直接决定自己该报多少，只给一个合计预付是看不出来的。
  it('shows the roster and how the seat fee is split before joining', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, memberCount: 2 })])
    rosterMock.mockResolvedValue([
      { userId: 1, username: 'owner-zhao', role: 'owner', declaredWeeklyQuotaUsd: 300 },
      { userId: 2, username: '', role: 'member', declaredWeeklyQuotaUsd: 900 },
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    expect(rosterMock).toHaveBeenCalledWith(10, undefined)
    const roster = wrapper.get('[data-testid="carpool-join-roster"]')
    expect(roster.text()).toContain('owner-zhao')
    expect(roster.text()).toContain('$300')
    expect(roster.text()).toContain('$900')
    // 生产上确实有用户名为空的成员，不能渲染成一行空白
    expect(roster.text()).toContain('carpool.joinDialog.rosterAnonymous')
    // 席位费按"现有 2 人 + 我"= 3 人分摊，预付也拆成席位 + 用量两部分
    expect(wrapper.text()).toContain('carpool.joinDialog.seatShareHint')
    expect(wrapper.text()).toContain('carpool.joinDialog.prepaidBreakdown')
  })

  // 花名册只是参考信息，接口挂了不该把人挡在车外。
  it('degrades gracefully when the roster fails to load', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10 })])
    rosterMock.mockRejectedValue(new Error('boom'))

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.join').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('carpool.joinDialog.rosterFailed')
    expect(wrapper.find('[data-testid="carpool-join-roster"]').exists()).toBe(false)
    // 申报输入和预付试算照常工作——花名册失败不影响主流程
    await wrapper.get('#carpool-join-quota').setValue(100)
    expect(wrapper.text()).toContain('carpool.joinDialog.previewPrepaid')
  })

  // confirmed 的车原来在前端是死胡同：车主没有任何入口，只能等 admin。
  it('lets the owner withdraw a confirmation on a confirmed carpool', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, status: 'confirmed', memberRole: 'owner', joinLocked: true }),
    ])
    unconfirmMock.mockResolvedValue(makeCarpool({ id: 10, status: 'recruiting', memberRole: 'owner' }))

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.actions.unconfirm').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="confirm-dialog-ok"]').trigger('click')
    await flushPromises()

    expect(unconfirmMock).toHaveBeenCalledWith(10)
    expect(showSuccess).toHaveBeenCalledWith('carpool.unconfirmDialog.success')
  })

  // admin 待启动列表：确认通知只有一封邮件，邮件丢了车就无限挂起。
  it('shows the admin pending-launch banner with overdue marking', async () => {
    authState.isAdmin = true
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, status: 'confirmed', joinLocked: true })])
    pendingLaunchMock.mockResolvedValue([{
      carpoolId: 10,
      name: 'stuck-car',
      ownerUserId: 3,
      ownerEmail: 'owner@example.com',
      memberCount: 8,
      declaredTotalUsd: 2400,
      weeklyLimitUsd: 2400,
      confirmedAt: '2026-07-20T00:00:00Z',
      pendingHours: 51.5,
      overdue: true,
    }])

    const wrapper = mountView()
    await flushPromises()

    const banner = wrapper.get('[data-testid="carpool-pending-launch"]')
    expect(banner.text()).toContain('stuck-car')
    expect(banner.text()).toContain('owner@example.com')
    expect(banner.text()).toContain('carpool.pendingLaunch.overdue')
  })

  it('hides the pending-launch banner from non-admins', async () => {
    authState.isAdmin = false
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, status: 'confirmed' })])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="carpool-pending-launch"]').exists()).toBe(false)
    expect(pendingLaunchMock).not.toHaveBeenCalled()
  })

  // 结算单未冻结时是实时预览（金额还会随用量走），车主能看到"确认结算"；
  // 冻结后换成"已结算"提示，结算按钮消失。
  it('offers settling on a live preview and shows the frozen banner afterwards', async () => {
    const car = makeCarpool({ id: 10, status: 'active', memberRole: 'owner' })
    listCarpools.mockResolvedValue([car])

    const live = {
      carpoolId: 10, status: 'active', weeklyLimitUsd: 2400, seatFeeCny: 400, usagePoolCny: 1000,
      reserveRatio: 0.8, memberCount: 2, fullView: true, members: [],
      settled: false, canSettle: true, settleBlockedFor: '',
    }
    settlementMock.mockResolvedValue(live)
    settleMock.mockResolvedValue({ ...live, settled: true, canSettle: false, settleBlockedFor: 'already_settled', settledAt: '2026-08-01T00:00:00Z' })

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'carpool.actions.settlement').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="carpool-settlement-live"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('carpool.settlement.livePreview')

    await findButton(wrapper, 'carpool.settlement.settle').trigger('click')
    await flushPromises()

    expect(settleMock).toHaveBeenCalledWith(10)
    expect(wrapper.find('[data-testid="carpool-settlement-frozen"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="carpool-settlement-live"]').exists()).toBe(false)
  })

  // 撤销结算只对 admin 出现——普通车主结算错了要找管理员。
  it('shows the undo-settlement action only to admins', async () => {
    const car = makeCarpool({ id: 10, status: 'active', memberRole: 'owner' })
    listCarpools.mockResolvedValue([car])
    settlementMock.mockResolvedValue({
      carpoolId: 10, status: 'active', weeklyLimitUsd: 2400, seatFeeCny: 400, usagePoolCny: 1000,
      reserveRatio: 0.8, memberCount: 2, fullView: true, members: [],
      settled: true, canSettle: false, settleBlockedFor: 'already_settled', settledAt: '2026-08-01T00:00:00Z',
    })

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.settlement').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="carpool-settlement-frozen"]').exists()).toBe(true)
    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.settlement.unsettle'))).toBe(false)
  })

  // 车主取消之后，车不该继续挂在「我的拼车」里——那会让人以为取消没生效。
  // 广场页本来就排除了已取消，这里曾经漏掉，只是不一致。
  it('hides a cancelled carpool from my-carpools by default', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'live-car', status: 'recruiting', memberRole: 'owner' }),
      makeCarpool({ id: 11, name: 'cancelled-car', status: 'cancelled', memberRole: 'owner' }),
    ])

    const wrapper = mountView()
    await flushPromises()

    // 切到「我的拼车」
    await findButton(wrapper, 'carpool.mine').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('live-car')
    expect(wrapper.text()).not.toContain('cancelled-car')
  })

  // 但要回看历史时，显式选「已取消」筛选仍然看得到。
  it('shows cancelled carpools when the status filter asks for them', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'live-car', status: 'recruiting', memberRole: 'owner' }),
      makeCarpool({ id: 11, name: 'cancelled-car', status: 'cancelled', memberRole: 'owner' }),
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.mine').trigger('click')
    await wrapper.get('select').setValue('cancelled')
    await flushPromises()

    expect(wrapper.text()).toContain('cancelled-car')
    expect(wrapper.text()).not.toContain('live-car')
  })

  // 广场页同样不出现已取消的车。
  it('keeps cancelled carpools out of the plaza', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 11, name: 'cancelled-car', status: 'cancelled', memberRole: null }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain('cancelled-car')
  })

  // 下车之后必须能重新上车。此前 member_role 子查询不排除 status='left'，
  // 前端一直以为用户还在车上：「已上车」不消失、「上车」按钮不出现、
  // 车也退不出「我的拼车」——而后端其实一直允许重新上车。
  it('offers joining again after the member has left', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'left-car', status: 'recruiting', memberRole: null }),
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.actions.join'))).toBe(true)
    expect(wrapper.text()).not.toContain('carpool.actions.joined')
  })

  // 车主即使没有成员行也要能在「我的拼车」里看到自己的车：发车时申报为 0 的
  // 车主会被置成 'left'，memberRole 因此为 null，只按 memberRole 过滤会让他
  // 看不见自己发起的车、也就没法取消它。
  it('keeps an owned carpool in my-carpools even without a member row', async () => {
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'owned-car', status: 'recruiting', memberRole: null, ownerUserId: 1 }),
      makeCarpool({ id: 11, name: 'stranger-car', status: 'recruiting', memberRole: null, ownerUserId: 999 }),
    ])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.mine').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('owned-car')
    expect(wrapper.text()).not.toContain('stranger-car')
  })

  // 自定义规则车（含平台升级前建立的老车）：展示规则说明，不渲染额度进度与
  // 均价——它们的成员申报恒为 0，硬渲染出来就是 "0 / 2400"、"均价 ¥0"。
  it('renders the rule note instead of a quota bar for custom-rule carpools', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10,
      status: 'active',
      memberRole: 'member',
      pricingModel: 'custom',
      ruleNote: '旧版席位规则：共 5 席，基础费 ¥130/席。',
    })])

    const wrapper = mountView()
    await flushPromises()

    const box = wrapper.get('[data-testid="carpool-custom-rule"]')
    expect(box.text()).toContain('carpool.customRule.badge')
    expect(box.text()).toContain('基础费 ¥130/席')
    expect(wrapper.text()).not.toContain('carpool.fields.quotaProgress')
    expect(wrapper.text()).not.toContain('carpool.fields.carMonthlyFee')
    expect(wrapper.text()).not.toContain('carpool.fields.remainingJoinable')
  })

  // 自定义规则车不接新成员：升级前遗留的招募中老车永远达不到发车区间，
  // 再放人进去就是往开不走的车里交预付。
  it('never offers joining a custom-rule carpool', async () => {
    listCarpools.mockResolvedValue([makeCarpool({
      id: 10, status: 'recruiting', memberRole: null, pricingModel: 'custom',
    })])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.actions.join'))).toBe(false)
  })

  // 人工结算：只列实际用量，不出退补列。
  it('shows a usage-only settlement for custom-rule carpools', async () => {
    const car = makeCarpool({ id: 10, status: 'active', memberRole: 'owner', pricingModel: 'custom' })
    listCarpools.mockResolvedValue([car])
    settlementMock.mockResolvedValue({
      carpoolId: 10, status: 'active', weeklyLimitUsd: 2400, seatFeeCny: 400, usagePoolCny: 1000,
      reserveRatio: 0.8, memberCount: 1, fullView: true,
      members: [{
        userId: 12, role: 'member', declaredWeeklyQuotaUsd: 0, floorUsageUsd: 0,
        actualUsageUsd: 123.4, billableUsageUsd: 0, floorTriggered: false,
        prepaidAmountCny: 0, quotedPrepaidCny: 0, usagePrepaidCny: 0, usageFinalShareCny: 0,
        usageDeltaCny: 0, seatFeePrepaidCny: 0, seatFeeFinalCny: 0, seatFeeDeltaCny: 0, totalDeltaCny: 0,
      }],
      settled: false, canSettle: false, settleBlockedFor: 'manual_settlement',
      manualSettlement: true, pricingModel: 'custom', ruleNote: '旧版席位规则：共 5 席。',
    })

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.settlement').trigger('click')
    await flushPromises()

    const banner = wrapper.get('[data-testid="carpool-settlement-manual"]')
    expect(banner.text()).toContain('carpool.settlement.manualTitle')
    expect(banner.text()).toContain('旧版席位规则')
    // 退补相关的列与提示一律不出现
    expect(wrapper.text()).not.toContain('carpool.settlement.delta')
    expect(wrapper.text()).not.toContain('carpool.settlement.prepaid')
    expect(wrapper.text()).not.toContain('carpool.settlement.deltaNote')
    // 但用量要在
    expect(wrapper.text()).toContain('carpool.settlement.actual')
    // 也不该出现"确认结算"入口
    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.settlement.settle'))).toBe(false)
  })

  // 连点两辆车的"邀请"：先发的请求后回来时不能覆盖对话框，
  // 否则会显示 B 车、链接却是 A 车的邀请码，分享出去就是错的车。
  it('discards a stale invite response when another carpool is clicked', async () => {
    authState.isAdmin = true
    listCarpools.mockResolvedValue([
      makeCarpool({ id: 10, name: 'car-a' }),
      makeCarpool({ id: 11, name: 'car-b' }),
    ])

    let resolveA: (value: string) => void = () => {}
    let resolveB: (value: string) => void = () => {}
    createInviteMock
      .mockReturnValueOnce(new Promise<string>((resolve) => { resolveA = resolve }))
      .mockReturnValueOnce(new Promise<string>((resolve) => { resolveB = resolve }))

    const wrapper = mountView()
    await flushPromises()

    const inviteButtons = wrapper.findAll('button').filter((item) => item.text().includes('carpool.actions.invite'))
    expect(inviteButtons.length).toBe(2)
    await inviteButtons[0].trigger('click')
    await inviteButtons[1].trigger('click')

    // B 先回，A 后回（乱序）
    resolveB('token-b')
    await flushPromises()
    resolveA('token-a')
    await flushPromises()

    const link = (wrapper.get('#carpool-invite-link').element as HTMLInputElement).value
    expect(link).toContain('token-b')
    expect(link).not.toContain('token-a')
    expect(wrapper.text()).toContain('car-b')
  })
})

describe('group QR zoom and owner replace', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listCarpools.mockResolvedValue([])
    groupQrCodeMock.mockResolvedValue(new Blob(['qr'], { type: 'image/png' }))
    URL.createObjectURL = vi.fn(() => 'blob:qr') as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn() as typeof URL.revokeObjectURL
  })

  // 二维码缩略图必须能点开大图：用户要拿手机扫，14px 的图扫不出来。
  it('zooms the group QR code on click and closes on backdrop click', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, hasGroupQrCode: true })])

    const wrapper = mountView()
    await flushPromises()

    const thumb = wrapper.find('img.cursor-zoom-in')
    expect(thumb.exists()).toBe(true)
    await thumb.trigger('click')
    await flushPromises()

    const zoomed = wrapper.find('.teleport-stub img')
    expect(zoomed.exists()).toBe(true)
    expect(zoomed.attributes('src')).toBe('blob:qr')

    await wrapper.find('.teleport-stub .fixed.inset-0').trigger('click')
    await flushPromises()
    expect(wrapper.find('.teleport-stub img').exists()).toBe(false)
  })

  // 车主在详情弹窗里能换群二维码；普通成员看不到这个入口（后端也只放行车主/admin）。
  it('lets the owner replace the group QR code from the detail dialog', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, hasGroupQrCode: true, memberRole: 'owner' })])
    replaceGroupQrCodeMock.mockResolvedValue(undefined)

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.details').trigger('click')
    await flushPromises()

    await findButton(wrapper, 'carpool.wechat.replaceQr').trigger('click')
    const input = wrapper.find('input[type="file"]')
    expect(input.exists()).toBe(true)

    const file = new File(['x'], 'qr.png', { type: 'image/png' })
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    // FileReader 的 onload 是宏任务，flushPromises 一轮不一定等得到
    await new Promise((r) => setTimeout(r, 20))
    await flushPromises()

    expect(replaceGroupQrCodeMock).toHaveBeenCalledWith(10, expect.stringMatching(/^data:image\/png/))
  })

  it('hides the replace entry from non-owner members', async () => {
    listCarpools.mockResolvedValue([makeCarpool({ id: 10, hasGroupQrCode: true, memberRole: 'member' })])

    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'carpool.actions.details').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().includes('carpool.wechat.replaceQr'))).toBe(false)
  })
})
