export default {
  carpool: {
    title: '拼车',
    description: '发现正在招募的拼车，或发起一辆新车',
    create: '发起拼车',
    rules: {
      title: 'GPT 拼车规则：额度预约制',
      weeklyBadge: '锁定额度按周刷新',
      declare: {
        label: '申报制',
        text: '上车前申报一周预期额度（USD），并按申报预付第一笔账；总申报进入整车周限额的 95%–105% 即可发车。'
      },
      reserve: {
        label: '80% 保底 + 20% 公共池',
        text: '申报的 80% 定向锁定给你，任何人抢不走；其余进入机动公共池，全员先到先得。'
      },
      pricing: {
        label: '¥400 + ¥1000 两部制',
        text: '每车每月 ¥400 席位费按人头均摊（人越多越便宜），¥1000 用量池按计费用量占比分摊。'
      },
      floor: {
        label: '80% 地板结算',
        text: '月末按 max（实际用量，80% × 申报）计费，多退少补——报多少保底多少，保底多少至少付多少。'
      }
    },
    notices: {
      weeklyRefresh: '锁定额度按周计算，每周自动刷新，未用完不结转。',
      consumeOrder: '用量优先消耗你的锁定额度，用完后才使用公共池；公共池全员共享、先到先得，不保证可用。',
      customRule: '支持自定义规则：如需不同的额度池、价格或保底比例，可联系管理员协商。'
    },
    plaza: '拼车广场',
    mine: '我的拼车',
    searchPlaceholder: '搜索拼车名称或发起人',
    allStatuses: '全部状态',
    stats: {
      recruiting: '正在招募',
      joinableQuota: '可预约额度',
      joined: '我已上车',
      launched: '已经开车'
    },
    status: {
      recruiting: '招募中',
      confirmed: '待管理员发车',
      starting: '准备开车',
      active: '已开车',
      cancelled: '已取消',
      ended: '已结束',
      locked: '已封车',
      full: '额度已满'
    },
    visibility: {
      public: '公开上车',
      inviteOnly: '仅限邀请'
    },
    fields: {
      name: '拼车名称',
      namePlaceholder: '例如：周末 Codex Pro 拼车',
      description: '备注',
      descriptionPlaceholder: '可选，补充开车时间或使用安排',
      visibility: '加入方式',
      scheduledStart: '预计开车',
      organizer: '发起人',
      weeklyLimitBadge: 'GPT · {limit} USD/周',
      quotaProgress: '额度池预约进度',
      declaredOf: '已预约 {declared} / {limit} USD',
      launchLine: '{ratio}% 发车线',
      remainingJoinable: '剩余可预约',
      effectiveRate: '等效倍率',
      effectiveRateHint: '¥1 ≈ 官方 ${usd}',
      carMonthlyFee: '整车月费',
      carMonthlyFeeUnit: '席位 {seat} + 用量 {pool}',
      members: '已上车 {count} 人'
    },
    roles: {
      owner: '我发起的',
      member: '我已上车'
    },
    actions: {
      join: '上车',
      joined: '已上车',
      invite: '邀请成员',
      details: '查看详情',
      confirm: '确认发车',
      launch: '启动',
      forceLaunch: '降档发车',
      leave: '下车',
      settlement: '结算单',
      lock: '停止上人',
      unlock: '重新开放',
      cancel: '取消拼车',
      unconfirm: '撤回确认',
      copyLink: '复制链接',
      copied: '邀请链接已复制'
    },
    // 后端错误码 → 中文提示。超额度、车满这类是核心拒绝路径，
    // 必须告诉用户下一步能做什么，而不是把英文原文抛出来。
    errors: {
      quotaExceeded: '申报额度超过这辆车的剩余可预约额度，请调低申报或等下一辆车',
      full: '这辆车人数已满，请等下一辆车',
      unavailable: '这辆车当前不能上车（可能已停止上人或已确认发车），请等下一辆车',
      alreadyJoined: '你已经在这辆车上了',
      forbidden: '你没有权限执行这个操作',
      notFound: '拼车不存在或已被取消',
      alreadySettled: '结算单已经冻结过了，需要重算请联系管理员撤销结算',
      notSettled: '这辆车还没有结算',
      notSettleable: '发车后才能结算',
      customRuleClosed: '这辆车按单独约定的规则运行，不再接受新成员，请上新车',
      inviteInvalid: '邀请链接无效或已过期，请向车主索取新链接',
      nameConflict: '已有同名的车在招募或运行中，请换一个名字',
      launchNotReady: '总申报额度不在发车区间内，暂时不能发车',
      notConfirmed: '车主尚未确认发车',
      declarationTooSmall: '申报额度不能低于 {min} USD/周',
      interestTooFrequent: '刚刚已经通知过管理员了，请稍后再试',
      customParamsForbidden: '自定义额度池 / 价格参数需要管理员协助，请在创建对话框里选择"自定义规则"联系管理员',
      groupJoinRequired: '请先确认已加入微信群',
      contactConfirmRequired: '请先确认已添加管理员微信',
      qrCodeRequired: '请上传微信群二维码',
      qrCodeInvalid: '群二维码需为 png / jpeg / webp 且不超过 2MB',
      ownerCannotLeave: '车主不能下车，只能取消整辆车',
      notMember: '你不在这辆车上'
    },
    // 自定义规则车：不走额度预约制，按 rule_note 写明的规则人工结算。
    // 平台升级前建立的老车全部属于此类。
    customRule: {
      badge: '自定义规则',
      noNote: '本车按单独约定的规则运行，不适用额度预约制的申报、保底与自动退补。'
    },
    wechat: {
      adminLabel: '管理员微信',
      scanToJoin: '扫码加入群聊',
      copied: '管理员微信号已复制',
      replaceQr: '更换二维码',
      qrReplaced: '群二维码已更换'
    },
    createDialog: {
      title: '发起新拼车',
      submit: '创建并生成邀请链接',
      success: '拼车已创建',
      ruleMode: '拼车规则',
      ruleModeDefault: '默认规则',
      ruleModeCustom: '自定义规则',
      customRule: {
        title: '自定义规则需联系管理员协商（额度池 / 价格 / 保底比例等）',
        description: '此模式下无需填写下方表单。点击“通知管理员”告知你的需求，管理员会通过邮件或微信与你联系；协商确认后由管理员为你创建并调整车辆。',
        notify: '通知管理员',
        notifySuccess: '已通知管理员，请添加微信 {wechat} 继续协商'
      },
      ownerQuota: '我的申报额度（可选，USD/周）',
      ownerQuotaHint: '留空表示仅发起拼车、不占用额度；填写则按 1 人记账预付。',
      contactTitle: '联系方式与群二维码（必填）',
      contactHint: '发车由管理员人工执行，发起前请先添加管理员微信，并上传微信群二维码供成员扫码入群。',
      addedAdmin: '我已添加管理员微信 {wechat}',
      qrLabel: '群聊二维码',
      qrHint: '支持 png / jpeg / webp，大小不超过 2MB。请先用微信「面对面建群」建好一人小群、拉管理员入群，再上传该群的二维码',
      qrInvalidType: '仅支持 png / jpeg / webp 格式图片',
      qrTooLarge: '图片大小不能超过 2MB'
    },
    joinDialog: {
      title: '确认上车',
      quotaLabel: '申报额度（USD/周）',
      recommendationLoading: '正在获取申报推荐…',
      recommendationFailed: '申报推荐获取失败，请按自身用量估计',
      previewFloor: '保底额度',
      floorUnit: 'USD/周',
      previewPrepaid: '预计预付',
      previewAvgPrice: '该车当前均价',
      previewEffectiveRate: '等效倍率',
      effectiveRateUnit: '¥1 ≈ 官方 ${usd}',
      effectiveRateBasis: '按 31 天折算',
      prepaidBreakdown: '席位 {seat} + 用量 {pool}',
      seatShareHint: '席位费 {total} ÷ {people} 人 = {each}/人',
      rosterTitle: '车上现有成员',
      rosterSummary: '{count} 人 · 共 {total}/周',
      rosterLoading: '正在加载成员…',
      rosterFailed: '成员列表加载失败，不影响上车',
      rosterEmpty: '还没有人上车，你是第一位',
      rosterOwner: '车主',
      rosterYou: '你（本次申报）',
      rosterAnonymous: '同学 #{id}',
      rateAboveAverage: '按你的申报折算，你的等效倍率是 {yours}，约为全车平均（{average}）的 {times} 倍——席位费按人头均摊，申报越少越不划算。',
      floorNotice: '即使未用满，也至少按申报的 80% 计费。',
      exceedsRemaining: '申报超过该车剩余可预约额度（{amount} USD），请调低或等待下一辆车',
      belowFloor: '申报额度不能低于 {min} USD/周',
      groupSection: '上车前先加入微信群',
      joinedGroup: '我已加入群聊',
      confirm: '确认上车',
      success: '已加入拼车，预付 ¥{amount}',
      successNoPrepaid: '已加入拼车'
    },
    confirmDialog: {
      title: '确认发车',
      message: '确认发车“{name}”？当前总申报 {total} USD（占周限额 {ratio}%）。确认后拼车将锁定，由管理员在 24 小时内启动。',
      confirm: '确认发车',
      notReady: '距 {ratio}% 发车线还差 {amount} USD',
      aboveMax: '已超出发车上限 {ratio}%，需有成员下车后才能确认',
      success: '已确认，等待管理员发车'
    },
    pendingLaunch: {
      title: '待启动（{count} 辆）',
      overdue: '已超 24 小时',
      overdueBadge: '{count} 辆超时',
      summary: '{members} 人 · 总申报 {total} · 已等待 {hours} 小时',
    },
    unconfirmDialog: {
      title: '撤回确认',
      message: '把"{name}"退回招募状态？成员和申报额度都会保留，重新开放上车；管理员启动前随时可以再次确认。',
      confirm: '确认撤回',
      success: '已撤回确认，拼车回到招募中'
    },
    leaveDialog: {
      title: '下车',
      message: '确定要从“{name}”下车吗？你的申报额度将立即释放。',
      confirm: '确认下车',
      success: '已下车，申报额度已释放'
    },
    launchDialog: {
      confirmTitle: '启动发车',
      confirmMessage: '确认启动“{name}”？车主已确认发车（总申报 {total} USD，占周限额 {ratio}%）。启动后按 80% 保底 + 公共池配置限额，本月锁定。',
      forceTitle: '降档发车',
      forceMessage: '当前总申报 {total} USD（占周限额 {ratio}%），未达 95% 标准线。确认降档发车“{name}”？公共池将变大，每位成员的保底不变。',
      confirm: '确认启动',
      forceReady: '已满 80%，可降档发车',
      success: '已发车'
    },
    cancelDialog: {
      title: '取消拼车',
      message: '取消“{name}”后，已有邀请链接将失效，已预约的额度也会被释放。',
      confirm: '确认取消',
      success: '拼车已取消'
    },
    inviteDialog: {
      title: '邀请成员',
      label: '邀请链接',
      uses: '当前已上车 {count} 人',
      unavailable: '当前拼车不能继续邀请成员'
    },
    detailDialog: {
      title: '拼车详情',
      runtime: '开车状态',
      linkedGroup: '关联分组',
      pendingGroup: '等待管理员开车时绑定'
    },
    settlement: {
      title: '月度结算单',
      period: '结算周期',
      member: '成员',
      declared: '申报 (USD/周)',
      actual: '实际用量 (USD)',
      billable: '计费用量 (USD)',
      floorBadge: '80% 地板',
      prepaid: '预付 (¥)',
      delta: '退/补 (¥)',
      deltaRefund: '退 ¥{amount}',
      deltaTopUp: '补 ¥{amount}',
      deltaEven: '¥0',
      deltaNote: '正数为退款，负数为补款',
      selfOnly: '仅展示你自己的结算行',
      fullView: '全车 {count} 名成员',
      usageDelta: '用量退/补',
      seatFeeDelta: '席位费退/补',
      loadFailed: '加载结算单失败',
      manualTitle: '本车按自定义规则人工结算',
      manualHint: '下方只列出各成员的实际用量，供车主按上述规则分账；平台不代为计算保底与退补。',
      livePreview: '实时预览：下方金额会随用量继续变化，确认结算后才会固定下来。',
      settle: '确认结算',
      settleSuccess: '结算单已冻结，金额不再变化',
      frozenAt: '已于 {time} 结算，金额已冻结',
      unsettle: '撤销结算',
      unsettleSuccess: '已撤销结算，回到实时预览',
      blockedNotLaunched: '发车后才能结算'
    },
    adminPage: {
      alertsTitle: '需要处理',
      alerts: {
        launchOverdue: '确认超 24 小时未启动',
        unsettled: '已结束未结算',
        overDeclared: '申报超上限',
        readyToConfirm: '已达发车线待确认'
      },
      allStatuses: '全部状态',
      searchPlaceholder: '搜索车名或发起人',
      total: '显示 {count} / 共 {all} 辆',
      empty: '没有符合条件的拼车',
      perWeek: '周',
      columns: {
        name: '拼车',
        status: '状态',
        owner: '发起人',
        members: '人数',
        declared: '申报 / 限额',
        settled: '结算时间',
        actions: '操作'
      },
      actions: {
        members: '成员',
        edit: '编辑',
        transfer: '转让车主',
        editQuota: '改额度',
        remove: '移出'
      },
      membersDialog: {
        title: '「{name}」的成员',
        hint: '发车前可以移出成员或代改申报额度，额度会立即释放并重算发车进度。',
        readOnly: '这辆车已发车或已结束，成员只能查看——改人涉及退补款，请走结算流程。',
        empty: '这辆车还没有成员',
        groupQr: '群二维码',
        qrOpen: '查看大图',
        qrLoading: '二维码加载中…',
        qrFailed: '二维码加载失败，点击重试',
        qrReplace: '更换',
        qrUpload: '上传二维码',
        qrReplaced: '群二维码已更换'
      },
      editDialog: {
        title: '编辑拼车'
      },
      transferDialog: {
        title: '转让车主',
        hint: '新车主必须是车上现有成员，转让后原车主降为普通成员。',
        pick: '选择新车主'
      },
      confirm: {
        unconfirm: {
          title: '撤回确认',
          message: '把「{name}」退回招募中？成员与申报额度都保留，重新开放上车。',
          success: '已撤回确认'
        },
        launch: {
          title: '启动发车',
          message: '确认启动「{name}」？启动后按 80% 保底 + 公共池配置限额，本月锁定。',
          success: '已发车'
        },
        cancel: {
          title: '取消拼车',
          message: '取消「{name}」后邀请链接失效，已预约的额度全部释放。',
          success: '拼车已取消'
        },
        cancelActive: {
          message: '「{name}」已发车：取消后全员订阅立即失效、不可恢复。已产生的用量与退补款请走结算流程线下处理。确定取消？'
        }
      },
      autoUnconfirmed: '操作已生效。总申报跌破发车线，这辆车已自动退回招募中，并已邮件通知车主。',
      memberRemoved: '成员已移出，申报额度已释放',
      quotaUpdated: '申报额度已更新',
      updated: '拼车信息已更新',
      ownerTransferred: '车主已转让'
    },
    admin: {
      locked: '已停止新成员上车',
      unlocked: '已重新开放上车'
    },
    empty: {
      title: '暂无符合条件的拼车',
      description: '调整筛选条件，或发起一辆新车'
    },
    unavailable: '这辆车当前不能上人',
    inviteNotFound: '邀请链接无效或已失效',
    loadFailed: '加载拼车失败',
    actionFailed: '拼车操作失败'
  }
}
