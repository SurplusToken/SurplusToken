package service

import (
	"context"
	"log/slog"
	"math"
	"strconv"
	"time"
)

// clampSharingRateBounds clamps floor/cap into the hard [0,5] range and
// ensures floor <= cap (falling back to the full hard range on corrupt data,
// including NaN/Inf: NaN fails every ordinary comparison, so it must be
// checked explicitly or it would silently disable the floor/cap check).
func clampSharingRateBounds(floor, cap float64) (float64, float64) {
	if math.IsNaN(floor) || floor < SharingRateMultiplierHardMin || floor > SharingRateMultiplierHardMax {
		floor = SharingRateFloorDefault
	}
	if math.IsNaN(cap) || cap < SharingRateMultiplierHardMin || cap > SharingRateMultiplierHardMax {
		cap = SharingRateCapDefault
	}
	if floor > cap {
		floor, cap = SharingRateFloorDefault, SharingRateCapDefault
	}
	return floor, cap
}

func normalizeSharingRateCooldownMinutes(minutes int) int {
	if minutes < 0 {
		return SharingRateOwnerCooldownMinutesDefault
	}
	if minutes > SharingRateOwnerCooldownMinutesMax {
		return SharingRateOwnerCooldownMinutesMax
	}
	return minutes
}

// ValidateSharingRateSettings validates the administrator's policy exactly as
// submitted. Reads still normalize legacy/corrupt stored values defensively,
// but writes must never silently turn an invalid request into another policy.
func ValidateSharingRateSettings(floor, cap float64, cooldownMinutes int) error {
	if math.IsNaN(floor) || math.IsInf(floor, 0) ||
		math.IsNaN(cap) || math.IsInf(cap, 0) ||
		floor < SharingRateMultiplierHardMin || floor > SharingRateMultiplierHardMax ||
		cap < SharingRateMultiplierHardMin || cap > SharingRateMultiplierHardMax ||
		floor > cap || cooldownMinutes < 0 || cooldownMinutes > SharingRateOwnerCooldownMinutesMax {
		return ErrSharingRateSettingsInvalid
	}
	if !hasSharingRateStoragePrecision(floor) || !hasSharingRateStoragePrecision(cap) {
		return ErrSharingRatePrecisionInvalid
	}
	return nil
}

// ValidateSharingRateMultiplier enforces both the hard [0,5] range and the
// admin-configurable dynamic floor/cap. Returns a BadRequest-style error
// (via ErrSharingRateOutOfRange) describing the effective bound on failure.
// NaN is rejected explicitly: it fails every ordinary comparison, so without
// this guard an invalid value would silently pass both range checks below.
func ValidateSharingRateMultiplier(value, floor, cap float64) error {
	if math.IsNaN(value) {
		return ErrSharingRateOutOfRange
	}
	if value < SharingRateMultiplierHardMin || value > SharingRateMultiplierHardMax {
		return ErrSharingRateOutOfRange
	}
	if value < floor || value > cap {
		return ErrSharingRateOutOfRange
	}
	if !hasSharingRateStoragePrecision(value) {
		return ErrSharingRatePrecisionInvalid
	}
	return nil
}

// cachedSharingRateSettings caches every sharing-rate-marketplace setting in
// one process-local, TTL'd entry (60s, mirrors cachedBackendMode) so the
// per-request eligibility filter and per-billing-event multiplier lookup
// never block on the DB.
type cachedSharingRateSettings struct {
	displayEnabled     bool
	rangeFilterEnabled bool
	poolBillingEnabled bool
	floor              float64
	cap                float64
	cooldownMinutes    int
	expiresAt          int64 // unix nano
}

const sharingRateSettingsCacheTTL = 60 * time.Second
const sharingRateSettingsErrorTTL = 5 * time.Second
const sharingRateSettingsDBTimeout = 5 * time.Second

const sharingRateStorageScale = 10_000.0

func canonicalSharingRate(value float64) float64 {
	return math.Round(value*sharingRateStorageScale) / sharingRateStorageScale
}

func hasSharingRateStoragePrecision(value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	canonical := canonicalSharingRate(value)
	return math.Abs(value-canonical) <= 1e-9
}

func defaultSharingRateSettingsSnapshot() *cachedSharingRateSettings {
	return &cachedSharingRateSettings{
		displayEnabled:     false,
		rangeFilterEnabled: false,
		poolBillingEnabled: false,
		floor:              SharingRateFloorDefault,
		cap:                SharingRateCapDefault,
		cooldownMinutes:    SharingRateOwnerCooldownMinutesDefault,
	}
}

var sharingRateSettingKeys = []string{
	SettingKeySharingPoolDisplayEnabled,
	SettingKeySharingRangeFilterEnabled,
	SettingKeySharingPoolBillingEnabled,
	SettingKeySharingRateFloor,
	SettingKeySharingRateCap,
	SettingKeySharingRateCooldownMinutes,
}

func parseSharingRateSettingsMap(values map[string]string) *cachedSharingRateSettings {
	floor, cap := SharingRateFloorDefault, SharingRateCapDefault
	if v, err := strconv.ParseFloat(values[SettingKeySharingRateFloor], 64); err == nil {
		floor = v
	}
	if v, err := strconv.ParseFloat(values[SettingKeySharingRateCap], 64); err == nil {
		cap = v
	}
	floor, cap = clampSharingRateBounds(floor, cap)
	cooldown := SharingRateOwnerCooldownMinutesDefault
	if v, err := strconv.Atoi(values[SettingKeySharingRateCooldownMinutes]); err == nil {
		cooldown = normalizeSharingRateCooldownMinutes(v)
	}
	return &cachedSharingRateSettings{
		displayEnabled:     values[SettingKeySharingPoolDisplayEnabled] == "true",
		rangeFilterEnabled: values[SettingKeySharingRangeFilterEnabled] == "true",
		poolBillingEnabled: values[SettingKeySharingPoolBillingEnabled] == "true",
		floor:              floor,
		cap:                cap,
		cooldownMinutes:    cooldown,
	}
}

func (s *SettingService) loadSharingRateSettings(ctx context.Context) *cachedSharingRateSettings {
	if s == nil {
		return defaultSharingRateSettingsSnapshot()
	}
	if cached, ok := s.sharingRateSettingsCache.Load().(*cachedSharingRateSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached
		}
	}
	result, _, _ := s.sharingRateSettingsSF.Do("sharing_rate_settings", func() (any, error) {
		var stale *cachedSharingRateSettings
		if cached, ok := s.sharingRateSettingsCache.Load().(*cachedSharingRateSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached, nil
			}
			stale = cached
		}
		if s.settingRepo == nil {
			snap := sharingRateSettingsStaleOnError(stale)
			s.sharingRateSettingsCache.Store(snap)
			return snap, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharingRateSettingsDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, sharingRateSettingKeys)
		if err != nil {
			slog.Warn("failed to get sharing rate marketplace settings", "error", err)
			// Keep the last known policy during a transient DB failure. Falling
			// back to flags=false here would temporarily ignore consumers' accepted
			// ranges and could route them to prices they explicitly rejected.
			snap := sharingRateSettingsStaleOnError(stale)
			s.sharingRateSettingsCache.Store(snap)
			return snap, nil
		}
		snap := parseSharingRateSettingsMap(values)
		snap.expiresAt = time.Now().Add(sharingRateSettingsCacheTTL).UnixNano()
		s.sharingRateSettingsCache.Store(snap)
		return snap, nil
	})
	if val, ok := result.(*cachedSharingRateSettings); ok && val != nil {
		return val
	}
	return defaultSharingRateSettingsSnapshot()
}

func sharingRateSettingsStaleOnError(stale *cachedSharingRateSettings) *cachedSharingRateSettings {
	if stale == nil {
		stale = defaultSharingRateSettingsSnapshot()
	}
	snap := *stale
	snap.expiresAt = time.Now().Add(sharingRateSettingsErrorTTL).UnixNano()
	return &snap
}

// refreshSharingRateSettingsCache eagerly repopulates the cache right after an
// admin settings write, closing the up-to-60s staleness window instead of
// waiting for the TTL to lapse.
func (s *SettingService) refreshSharingRateSettingsCache(settings *SystemSettings) {
	if s == nil || settings == nil {
		return
	}
	s.sharingRateSettingsSF.Forget("sharing_rate_settings")
	floor, cap := clampSharingRateBounds(settings.SharingRateFloor, settings.SharingRateCap)
	s.sharingRateSettingsCache.Store(&cachedSharingRateSettings{
		displayEnabled:     settings.SharingPoolDisplayEnabled,
		rangeFilterEnabled: settings.SharingRangeFilterEnabled,
		poolBillingEnabled: settings.SharingPoolBillingEnabled,
		floor:              floor,
		cap:                cap,
		cooldownMinutes:    normalizeSharingRateCooldownMinutes(settings.SharingRateCooldownMinutes),
		expiresAt:          time.Now().Add(sharingRateSettingsCacheTTL).UnixNano(),
	})
}

// IsSharingPoolDisplayEnabled reports whether the frontend should render
// shared-account-marketplace pricing UI.
func (s *SettingService) IsSharingPoolDisplayEnabled(ctx context.Context) bool {
	return s.loadSharingRateSettings(ctx).displayEnabled
}

// IsSharingRangeFilterEnabled reports whether scheduling/model-list must
// filter contributed accounts by the consumer's accepted sharing-rate range.
func (s *SettingService) IsSharingRangeFilterEnabled(ctx context.Context) bool {
	return s.loadSharingRateSettings(ctx).rangeFilterEnabled
}

// IsSharingPoolBillingEnabled reports whether billing should apply the
// effective sharing-rate multiplier (and accrue contribution-pool rewards)
// for external consumers of a contributed account.
func (s *SettingService) IsSharingPoolBillingEnabled(ctx context.Context) bool {
	return s.loadSharingRateSettings(ctx).poolBillingEnabled
}

// GetSharingRateBounds returns the admin-configurable dynamic floor/cap that
// owner price updates must additionally satisfy (on top of the hard [0,5]
// range).
func (s *SettingService) GetSharingRateBounds(ctx context.Context) (floor, cap float64) {
	v := s.loadSharingRateSettings(ctx)
	return v.floor, v.cap
}

// GetSharingRateOwnerCooldownMinutes returns the minimum interval an owner
// must wait between two sharing-price changes on the same account.
func (s *SettingService) GetSharingRateOwnerCooldownMinutes(ctx context.Context) int {
	return s.loadSharingRateSettings(ctx).cooldownMinutes
}
