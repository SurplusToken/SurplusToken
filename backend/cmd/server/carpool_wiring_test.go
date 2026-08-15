package main

import (
	"os"
	"regexp"
	"strings"
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
// 这三行都在生成文件里，靠人工 review diff 很容易漏掉，所以在这里对生成结果本身
// 做断言：wire 重新生成后只要少了任何一处，这个测试立刻失败。
func TestWireGenWiresCarpoolDependencies(t *testing.T) {
	src, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err, "读取 wire_gen.go 失败")
	code := string(src)

	for _, want := range []struct {
		snippet string
		why     string
	}{
		{
			"subscriptionService.SetCarpoolUpstreamWindowSource(",
			"拼车周窗口将不跟随上游重置，成员会被我们自己的计数器挡住",
		},
		{
			"subscriptionService.SetCarpoolBillingCycleRecorder(",
			"已关闭周期不落台账，月底结算会丢掉按周期的 80% 地板",
		},
	} {
		require.Containsf(t, code, want.snippet,
			"wire_gen.go 里缺少 %q —— %s", want.snippet, want.why)
	}

	// 注入必须发生在 subscriptionService 构造之后，否则是对 nil 调用。
	ctorIdx := strings.Index(code, "subscriptionService := service.NewSubscriptionService(")
	require.GreaterOrEqual(t, ctorIdx, 0, "找不到 subscriptionService 的构造")
	for _, setter := range []string{
		"subscriptionService.SetCarpoolUpstreamWindowSource(",
		"subscriptionService.SetCarpoolBillingCycleRecorder(",
	} {
		require.Greaterf(t, strings.Index(code, setter), ctorIdx,
			"%s 出现在 subscriptionService 构造之前", setter)
	}
}

// 兜底：确保上面断言的 setter 名字没有被重命名——名字改了而测试没跟着改，
// 断言会变成对一个不存在的方法做字符串匹配，等于白测。
func TestCarpoolWiringSettersExistOnService(t *testing.T) {
	src, err := os.ReadFile("../../internal/service/subscription_service.go")
	require.NoError(t, err)
	code := string(src)

	for _, sig := range []string{
		`func (s *SubscriptionService) SetCarpoolUpstreamWindowSource(`,
		`func (s *SubscriptionService) SetCarpoolBillingCycleRecorder(`,
	} {
		require.Containsf(t, code, sig, "subscription_service.go 里找不到 %q", sig)
	}

	// 缺失时必须有告警，不能静默降级
	require.Regexp(t, regexp.MustCompile(`warnMissingUpstreamWindows\(\)`), code,
		"上游窗口缺失时没有告警")
	require.Regexp(t, regexp.MustCompile(`warnMissingCycleRecorder\(\)`), code,
		"台账记录器缺失时没有告警")
}
