package service

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// sharingRateTestSettingRepo is a minimal SettingRepository stub exercising only
// GetMultiple, which is all loadSharingRateSettings needs.
type sharingRateTestSettingRepo struct {
	values map[string]string
	err    error
	calls  int
}

func (r *sharingRateTestSettingRepo) Get(_ context.Context, _ string) (*Setting, error) {
	panic("sharingRateTestSettingRepo.Get not implemented")
}

func (r *sharingRateTestSettingRepo) GetValue(_ context.Context, _ string) (string, error) {
	panic("sharingRateTestSettingRepo.GetValue not implemented")
}

func (r *sharingRateTestSettingRepo) Set(_ context.Context, _, _ string) error {
	panic("sharingRateTestSettingRepo.Set not implemented")
}

func (r *sharingRateTestSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := r.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (r *sharingRateTestSettingRepo) SetMultiple(_ context.Context, _ map[string]string) error {
	panic("sharingRateTestSettingRepo.SetMultiple not implemented")
}

func (r *sharingRateTestSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	panic("sharingRateTestSettingRepo.GetAll not implemented")
}

func (r *sharingRateTestSettingRepo) Delete(_ context.Context, _ string) error {
	panic("sharingRateTestSettingRepo.Delete not implemented")
}

var _ SettingRepository = (*sharingRateTestSettingRepo)(nil)

// TestSharingRateSettingsCache_IsolatedAcrossServiceInstances proves the cache
// moved from a package global to a per-SettingService instance field: two
// SettingService instances backed by different repos must never observe each
// other's cached floor/cap, whether read sequentially or concurrently.
func TestSharingRateSettingsCache_IsolatedAcrossServiceInstances(t *testing.T) {
	repoA := &sharingRateTestSettingRepo{values: map[string]string{
		SettingKeySharingRateFloor: "0.5",
		SettingKeySharingRateCap:   "1.5",
	}}
	repoB := &sharingRateTestSettingRepo{values: map[string]string{
		SettingKeySharingRateFloor: "2.0",
		SettingKeySharingRateCap:   "3.0",
	}}
	svcA := &SettingService{settingRepo: repoA}
	svcB := &SettingService{settingRepo: repoB}

	floorA, capA := svcA.GetSharingRateBounds(t.Context())
	require.Equal(t, 0.5, floorA)
	require.Equal(t, 1.5, capA)

	floorB, capB := svcB.GetSharingRateBounds(t.Context())
	require.Equal(t, 2.0, floorB)
	require.Equal(t, 3.0, capB)

	// Re-read A after B populated its own cache: must still see A's values.
	floorA2, capA2 := svcA.GetSharingRateBounds(t.Context())
	require.Equal(t, 0.5, floorA2)
	require.Equal(t, 1.5, capA2)

	// Concurrent reads across both instances must not cross-contaminate.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			floor, cap := svcA.GetSharingRateBounds(t.Context())
			require.Equal(t, 0.5, floor)
			require.Equal(t, 1.5, cap)
		}()
		go func() {
			defer wg.Done()
			floor, cap := svcB.GetSharingRateBounds(t.Context())
			require.Equal(t, 2.0, floor)
			require.Equal(t, 3.0, cap)
		}()
	}
	wg.Wait()
}

// TestSharingRateSettingsCache_RefreshAfterUpdateIsInstanceScoped proves that
// refreshSharingRateSettingsCache — the eager cache repopulation invoked from
// UpdateSettings/UpdateSettingsWithAuthSourceDefaults right after a successful
// write — only mutates the cache of the SettingService instance it is called
// on, and that the refreshed value is immediately visible without waiting for
// the TTL to lapse or re-hitting the DB.
func TestSharingRateSettingsCache_RefreshAfterUpdateIsInstanceScoped(t *testing.T) {
	repoA := &sharingRateTestSettingRepo{values: map[string]string{
		SettingKeySharingRateFloor: "0.5",
		SettingKeySharingRateCap:   "1.5",
	}}
	repoB := &sharingRateTestSettingRepo{values: map[string]string{
		SettingKeySharingRateFloor: "2.0",
		SettingKeySharingRateCap:   "3.0",
	}}
	svcA := &SettingService{settingRepo: repoA}
	svcB := &SettingService{settingRepo: repoB}

	// Warm both caches from their respective repos first.
	_, _ = svcA.GetSharingRateBounds(t.Context())
	_, _ = svcB.GetSharingRateBounds(t.Context())
	callsAAfterWarm := repoA.calls
	callsBAfterWarm := repoB.calls

	// Simulate what UpdateSettings does after a successful write to svcA only.
	svcA.refreshSharingRateSettingsCache(&SystemSettings{SharingRateFloor: 0.9, SharingRateCap: 1.1})

	floorA, capA := svcA.GetSharingRateBounds(t.Context())
	require.Equal(t, 0.9, floorA)
	require.Equal(t, 1.1, capA)
	require.Equal(t, callsAAfterWarm, repoA.calls, "refresh must serve from cache, no extra DB round trip")

	// svcB's cache must be completely untouched by svcA's refresh.
	floorB, capB := svcB.GetSharingRateBounds(t.Context())
	require.Equal(t, 2.0, floorB)
	require.Equal(t, 3.0, capB)
	require.Equal(t, callsBAfterWarm, repoB.calls)
}

func TestSharingRateSettingsCache_UsesStalePolicyOnRepositoryError(t *testing.T) {
	repo := &sharingRateTestSettingRepo{values: map[string]string{
		SettingKeySharingPoolDisplayEnabled:  "true",
		SettingKeySharingRangeFilterEnabled:  "true",
		SettingKeySharingPoolBillingEnabled:  "true",
		SettingKeySharingRateFloor:           "0.5",
		SettingKeySharingRateCap:             "2.5",
		SettingKeySharingRateCooldownMinutes: "20",
	}}
	svc := &SettingService{settingRepo: repo}

	warm := svc.loadSharingRateSettings(t.Context())
	require.True(t, warm.rangeFilterEnabled)
	require.Equal(t, 0.5, warm.floor)

	expired := *warm
	expired.expiresAt = time.Now().Add(-time.Second).UnixNano()
	svc.sharingRateSettingsCache.Store(&expired)
	repo.err = errors.New("temporary database failure")

	got := svc.loadSharingRateSettings(t.Context())
	require.True(t, got.displayEnabled)
	require.True(t, got.rangeFilterEnabled)
	require.True(t, got.poolBillingEnabled)
	require.Equal(t, 0.5, got.floor)
	require.Equal(t, 2.5, got.cap)
	require.Equal(t, 20, got.cooldownMinutes)
	require.Greater(t, got.expiresAt, time.Now().UnixNano())
	require.Equal(t, 2, repo.calls)
}

func TestSharingRateSettingsCache_ColdRepositoryErrorUsesSafeDefaults(t *testing.T) {
	repo := &sharingRateTestSettingRepo{err: errors.New("database unavailable")}
	svc := &SettingService{settingRepo: repo}

	got := svc.loadSharingRateSettings(t.Context())
	require.False(t, got.displayEnabled)
	require.False(t, got.rangeFilterEnabled)
	require.False(t, got.poolBillingEnabled)
	require.Equal(t, SharingRateFloorDefault, got.floor)
	require.Equal(t, SharingRateCapDefault, got.cap)
}

func TestValidateSharingRateSettingsRejectsInvalidAdminPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		floor    float64
		cap      float64
		cooldown int
	}{
		{name: "floor above cap", floor: 4, cap: 3, cooldown: 10},
		{name: "negative floor", floor: -1, cap: 3, cooldown: 10},
		{name: "cap over hard max", floor: 1, cap: 6, cooldown: 10},
		{name: "nan floor", floor: math.NaN(), cap: 3, cooldown: 10},
		{name: "infinite cap", floor: 1, cap: math.Inf(1), cooldown: 10},
		{name: "negative cooldown", floor: 1, cap: 3, cooldown: -1},
		{name: "cooldown over max", floor: 1, cap: 3, cooldown: SharingRateOwnerCooldownMinutesMax + 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateSharingRateSettings(test.floor, test.cap, test.cooldown), ErrSharingRateSettingsInvalid)
		})
	}
	require.NoError(t, ValidateSharingRateSettings(0, 5, 0))
	require.NoError(t, ValidateSharingRateSettings(1, 1, SharingRateOwnerCooldownMinutesMax))
}
