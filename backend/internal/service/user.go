package service

import (
	"math"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID             int64
	Email          string
	Username       string
	Notes          string
	AvatarURL      string
	AvatarSource   string
	AvatarMIME     string
	AvatarByteSize int
	AvatarSHA256   string
	PasswordHash   string
	Role           string
	Balance        float64
	FrozenBalance  float64
	Concurrency    int
	Status         string
	AllowedGroups  []int64
	TokenVersion   int64 // Incremented on password change to invalidate existing tokens
	// TokenVersionResolved indicates TokenVersion already contains the fingerprint-derived
	// value expected in JWT claims and refresh-token state.
	TokenVersionResolved bool
	SignupSource         string
	LastLoginAt          *time.Time
	LastActiveAt         *time.Time
	LastUsedAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            *time.Time // 非 nil 表示用户已软删除

	// GroupRates 用户专属分组倍率配置
	// map[groupID]rateMultiplier
	GroupRates map[int64]float64

	// TOTP 双因素认证字段
	TotpSecretEncrypted *string    // AES-256-GCM 加密的 TOTP 密钥
	TotpEnabled         bool       // 是否启用 TOTP
	TotpEnabledAt       *time.Time // TOTP 启用时间

	// 余额不足通知
	BalanceNotifyEnabled       bool
	BalanceNotifyThresholdType string // "fixed" (default) | "percentage"
	BalanceNotifyThreshold     *float64
	BalanceNotifyExtraEmails   []NotifyEmailEntry
	TotalRecharged             float64

	// RPMLimit 用户级每分钟请求数上限（0 = 不限制）。仅在所用分组未设置 rpm_limit
	// 且该 (用户, 分组) 无 rpm_override 时作为全局兜底生效，计数键 rpm:u:{userID}:{min}。
	RPMLimit int

	// UserGroupRPMOverride 来自 auth cache snapshot 的 (user, group) RPM 覆盖值。
	// nil = 该 API Key 对应的 (user, group) 无 override；非 nil 时 checkRPM 直接使用，
	// 避免每请求查 DB。字段不持久化到数据库。
	UserGroupRPMOverride *int

	APIKeys       []APIKey
	Subscriptions []UserSubscription

	// ContributionRewardRate 用户贡献账号收益比例覆盖。nil 表示跟随系统全局默认。
	ContributionRewardRate *float64

	// SharingRateMin/Max 用户愿意接受的贡献账号共享报价闭区间。两端可空；
	// 均空表示不过滤（接受任意报价）。
	SharingRateMin *float64
	SharingRateMax *float64
}

// AcceptsSharingRate reports whether rate falls within the user's accepted
// closed interval [SharingRateMin, SharingRateMax]. A nil bound is unbounded
// on that side; both nil accepts any rate. A NaN rate is never accepted:
// NaN comparisons are always false, so without this guard an invalid rate
// would silently pass every bound check.
func (u *User) AcceptsSharingRate(rate float64) bool {
	if math.IsNaN(rate) {
		return false
	}
	if u == nil {
		return true
	}
	if u.SharingRateMin != nil && rate < *u.SharingRateMin {
		return false
	}
	if u.SharingRateMax != nil && rate > *u.SharingRateMax {
		return false
	}
	return true
}

// ValidateSharingRateAcceptedRange validates a user's accepted sharing-price
// closed interval [min, max] before it is persisted. A nil bound is
// unbounded on that side and always valid on its own. Each non-nil bound
// must be a finite value within the hard [SharingRateMultiplierHardMin,
// SharingRateMultiplierHardMax] range, and min must not exceed max.
func ValidateSharingRateAcceptedRange(min, max *float64) error {
	if err := validateSharingRateBound(min); err != nil {
		return err
	}
	if err := validateSharingRateBound(max); err != nil {
		return err
	}
	if min != nil && max != nil && *min > *max {
		return ErrSharingRateRangeInvalid
	}
	return nil
}

func validateSharingRateBound(bound *float64) error {
	if bound == nil {
		return nil
	}
	v := *bound
	if math.IsNaN(v) || v < SharingRateMultiplierHardMin || v > SharingRateMultiplierHardMax {
		return ErrSharingRateOutOfRange
	}
	if !hasSharingRateStoragePrecision(v) {
		return ErrSharingRatePrecisionInvalid
	}
	return nil
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// CanBindGroup checks whether a user can bind to a given group.
// For standard groups:
// - Public groups (non-exclusive): all users can bind
// - Exclusive groups: only users with the group in AllowedGroups can bind
func (u *User) CanBindGroup(groupID int64, isExclusive bool) bool {
	// 公开分组（非专属）：所有用户都可以绑定
	if !isExclusive {
		return true
	}
	// 专属分组：需要在 AllowedGroups 中
	for _, id := range u.AllowedGroups {
		if id == groupID {
			return true
		}
	}
	return false
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}
