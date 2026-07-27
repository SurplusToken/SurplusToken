package service

import "time"

const subscriptionDayDuration = 24 * time.Hour

type UserSubscription struct {
	ID      int64
	UserID  int64
	GroupID int64

	StartsAt  time.Time
	ExpiresAt time.Time
	Status    string

	DailyWindowStart   *time.Time
	WeeklyWindowStart  *time.Time
	MonthlyWindowStart *time.Time

	DailyUsageUSD   float64
	WeeklyUsageUSD  float64
	MonthlyUsageUSD float64

	// WeeklyLimitUSD 是订阅级周限额覆盖（拼车额度预约制）。
	// nil 表示未设置，限额检查回退到分组级 group.WeeklyLimitUSD。
	WeeklyLimitUSD *float64

	// WeeklyReservedUSD 是拼车保底额度 r（公共池硬约束，设计文档 §4.2）。
	// nil 表示非拼车订阅，限额行为与既有语义完全一致。
	// 当周用量 < r 时无条件放行（保底硬保证）；≥ r 的部分计入组级公共池计数器，
	// 全车超额之和达到公共池容量 C 后拒绝新的公共池消耗。
	WeeklyReservedUSD *float64

	AssignedBy *int64
	AssignedAt time.Time
	Notes      string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	User           *User
	Group          *Group
	AssignedByUser *User
}

func (s *UserSubscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive && time.Now().Before(s.ExpiresAt)
}

func (s *UserSubscription) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *UserSubscription) DaysRemaining() int {
	return s.daysRemainingAt(time.Now())
}

func (s *UserSubscription) daysRemainingAt(now time.Time) int {
	remaining := s.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}

	days := int(remaining / subscriptionDayDuration)
	if remaining%subscriptionDayDuration != 0 {
		days++
	}
	return days
}

func (s *UserSubscription) IsWindowActivated() bool {
	return s.DailyWindowStart != nil || s.WeeklyWindowStart != nil || s.MonthlyWindowStart != nil
}

func (s *UserSubscription) HasOneTimeDailyQuota() bool {
	if s == nil || s.StartsAt.IsZero() || s.ExpiresAt.IsZero() {
		return false
	}
	return !s.ExpiresAt.After(s.StartsAt.AddDate(0, 0, 1))
}

func (s *UserSubscription) NeedsDailyReset() bool {
	return s.NeedsDailyResetAt(time.Now())
}

func (s *UserSubscription) NeedsDailyResetAt(now time.Time) bool {
	if s.DailyWindowStart == nil {
		return false
	}
	if s.HasOneTimeDailyQuota() {
		return false
	}
	return !now.Before(s.DailyWindowStart.Add(24 * time.Hour))
}

func (s *UserSubscription) NeedsWeeklyReset() bool {
	if s.WeeklyWindowStart == nil {
		return false
	}
	return time.Since(*s.WeeklyWindowStart) >= 7*24*time.Hour
}

func (s *UserSubscription) NeedsMonthlyReset() bool {
	if s.MonthlyWindowStart == nil {
		return false
	}
	return time.Since(*s.MonthlyWindowStart) >= 30*24*time.Hour
}

func (s *UserSubscription) DailyResetTime() *time.Time {
	if s.DailyWindowStart == nil {
		return nil
	}
	if s.HasOneTimeDailyQuota() {
		t := s.ExpiresAt
		return &t
	}
	t := s.DailyWindowStart.Add(24 * time.Hour)
	return &t
}

func (s *UserSubscription) WeeklyResetTime() *time.Time {
	if s.WeeklyWindowStart == nil {
		return nil
	}
	t := s.WeeklyWindowStart.Add(7 * 24 * time.Hour)
	return &t
}

func (s *UserSubscription) MonthlyResetTime() *time.Time {
	if s.MonthlyWindowStart == nil {
		return nil
	}
	t := s.MonthlyWindowStart.Add(30 * 24 * time.Hour)
	return &t
}

func (s *UserSubscription) CheckDailyLimit(group *Group, additionalCost float64) bool {
	if !group.HasDailyLimit() {
		return true
	}
	return s.DailyUsageUSD+additionalCost <= *group.DailyLimitUSD
}

func (s *UserSubscription) CheckWeeklyLimit(group *Group, additionalCost float64) bool {
	limit := s.EffectiveWeeklyLimit(group)
	if limit == nil {
		return true
	}
	return s.WeeklyUsageUSD+additionalCost <= *limit
}

// EffectiveWeeklyLimit 返回生效的周限额：订阅级覆盖优先，否则回退分组级限额。
// 两者都未设置时返回 nil（不限量）。
func (s *UserSubscription) EffectiveWeeklyLimit(group *Group) *float64 {
	if s.WeeklyLimitUSD != nil {
		return s.WeeklyLimitUSD
	}
	if group != nil && group.HasWeeklyLimit() {
		return group.WeeklyLimitUSD
	}
	return nil
}

// HasWeeklyReserve 报告本订阅是否为拼车额度预约订阅（写入了保底额度 r）。
// 仅这类订阅参与组级公共池计数；其余订阅限额行为与既有语义一致。
func (s *UserSubscription) HasWeeklyReserve() bool {
	return s != nil && s.WeeklyReservedUSD != nil
}

// CarpoolSharedPoolCapacityUSD 返回本订阅所在车的公共池容量 C。
// 发车时 weekly_limit_usd = r + C（个人绝对上限），故 C = 周限额 − 保底；
// 全车成员按同一公式写入，各自派生出相同的 C。字段缺失或差值非正时返回 0
// （调用方据此跳过公共池检查）。
func (s *UserSubscription) CarpoolSharedPoolCapacityUSD() float64 {
	if s == nil || s.WeeklyReservedUSD == nil || s.WeeklyLimitUSD == nil {
		return 0
	}
	if capacity := *s.WeeklyLimitUSD - *s.WeeklyReservedUSD; capacity > 0 {
		return capacity
	}
	return 0
}

// NeedsCarpoolCommonsCheck 报告本次预检查是否需要读组级公共池计数器：
// 仅当周用量已达到保底 r（后续请求将消耗公共池）时才需要。
func (s *UserSubscription) NeedsCarpoolCommonsCheck(weeklyUsage float64) bool {
	return s.HasWeeklyReserve() && weeklyUsage >= *s.WeeklyReservedUSD
}

func (s *UserSubscription) CheckMonthlyLimit(group *Group, additionalCost float64) bool {
	if !group.HasMonthlyLimit() {
		return true
	}
	return s.MonthlyUsageUSD+additionalCost <= *group.MonthlyLimitUSD
}

func (s *UserSubscription) CheckAllLimits(group *Group, additionalCost float64) (daily, weekly, monthly bool) {
	daily = s.CheckDailyLimit(group, additionalCost)
	weekly = s.CheckWeeklyLimit(group, additionalCost)
	monthly = s.CheckMonthlyLimit(group, additionalCost)
	return
}
