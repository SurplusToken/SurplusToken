package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// 拼车依赖的三处注入曾经在一次 wire 重新生成中被整块抹掉，而且缺失是完全静默的：
//
//   - 上游周窗口缺失 → 拼车的"一周"不再跟随 OpenAI 的重置，上游重置后成员反而
//     被我们自己的计数器挡在门外（线上真实发生过，两辆车靠手工刷库才恢复）；
//   - 台账记录器缺失 → 每个已关闭周期的流水不落库，月底按 80% 地板结算直接失真
//     （线上有整整一辆车两天的台账为空）；
//   - 实测容量源缺失 → 公共池容量退回静态值。
//
// 注入现在固定在 service provider 源文件中，生成文件只负责调用 provider。测试同时
// 断言这两层，避免重新生成 wire_gen.go 时再次静默丢失。
func TestWireGenWiresCarpoolDependencies(t *testing.T) {
	src, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err, "读取 wire_gen.go 失败")
	code := string(src)

	require.Contains(t, code, "service.ProvideSubscriptionService(",
		"生成结果必须通过源级 provider 构造 SubscriptionService")
	require.Contains(t, code, "service.ProvideBillingCacheService(",
		"生成结果必须通过源级 provider 构造 BillingCacheService")
	require.NotContains(t, code, "service.NewSubscriptionService(",
		"生成结果绕过 provider 会再次丢失拼车依赖")
}

// 兜底：确保上面断言的 setter 名字没有被重命名——名字改了而测试没跟着改，
// 断言会变成对一个不存在的方法做字符串匹配，等于白测。
func TestCarpoolWiringSettersExistOnService(t *testing.T) {
	src, err := os.ReadFile("../../internal/service/wire.go")
	require.NoError(t, err)
	code := string(src)

	for _, snippet := range []string{
		`func ProvideSubscriptionService(`,
		`svc.SetCarpoolUpstreamWindowSource(carpoolUpstreamWindows)`,
		`svc.SetCarpoolBillingCycleRecorder(carpoolBillingCycles)`,
		`svc.SetCarpoolObservedCapacitySource(capacitySource)`,
	} {
		require.Containsf(t, code, snippet, "service/wire.go 里找不到 %q", snippet)
	}

	warningSrc, err := os.ReadFile("../../internal/service/subscription_service.go")
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`warnMissingUpstreamWindows\(\)`), string(warningSrc),
		"上游窗口缺失时没有告警")
	require.Regexp(t, regexp.MustCompile(`warnMissingCycleRecorder\(\)`), string(warningSrc),
		"台账记录器缺失时没有告警")
}
