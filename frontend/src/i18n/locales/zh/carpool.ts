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
      consumeOrder: '用量优先消耗你的锁定额度，用完后才使用公共池；公共池全员共享、先到先得，不保证可用。'
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
      plusEquivalents: 'Plus 等价',
      avgPrice: '均价',
      avgPriceUnit: '¥ / Plus等价 / 月',
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
      copyLink: '复制链接',
      copied: '邀请链接已复制'
    },
    wechat: {
      adminLabel: '管理员微信',
      scanToJoin: '扫码加入群聊',
      copied: '管理员微信号已复制'
    },
    createDialog: {
      title: '发起新拼车',
      submit: '创建并生成邀请链接',
      success: '拼车已创建',
      ownerQuota: '我的申报额度（可选，USD/周）',
      ownerQuotaHint: '留空表示仅发起拼车、不占用额度；填写则按 1 人记账预付。',
      contactTitle: '联系方式与群二维码（必填）',
      contactHint: '发车由管理员人工执行，发起前请先添加管理员微信，并上传微信群二维码供成员扫码入群。',
      addedAdmin: '我已添加管理员微信 {wechat}',
      qrLabel: '群聊二维码',
      qrHint: '支持 png / jpeg / webp，大小不超过 2MB',
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
      floorNotice: '即使未用满，也至少按申报的 80% 计费。',
      exceedsRemaining: '申报超过该车剩余可预约额度（{amount} USD），请调低或等待下一辆车',
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
      loadFailed: '加载结算单失败'
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
