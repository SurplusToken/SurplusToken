package handler

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AvailableChannelHandler 处理用户侧「可用渠道」查询。
//
// 用户侧接口委托 ChannelService.ListAvailable，并在返回前做三层过滤：
//  1. 行过滤：只保留状态为 Active 且与当前用户可访问分组有交集的渠道；
//  2. 分组过滤：渠道的 Groups 只保留用户可访问的那些；
//  3. 平台过滤：渠道的 SupportedModels 只保留平台在用户可见 Groups 中出现过的模型，
//     防止"渠道同时挂在 antigravity / anthropic 两个平台的分组上，用户只访问
//     antigravity，却看到 anthropic 模型"这类跨平台信息泄漏；
//  4. 字段白名单：仅返回用户需要的字段（省略 BillingModelSource / RestrictModels
//     / 内部 ID / Status 等管理字段）。
type AvailableChannelHandler struct {
	channelService *service.ChannelService
	gatewayService *service.GatewayService
	apiKeyService  *service.APIKeyService
	settingService *service.SettingService
	opsService     *service.OpsService
}

// NewAvailableChannelHandler 创建用户侧可用渠道 handler。
func NewAvailableChannelHandler(
	channelService *service.ChannelService,
	gatewayService *service.GatewayService,
	apiKeyService *service.APIKeyService,
	settingService *service.SettingService,
	opsService *service.OpsService,
) *AvailableChannelHandler {
	return &AvailableChannelHandler{
		channelService: channelService,
		gatewayService: gatewayService,
		apiKeyService:  apiKeyService,
		settingService: settingService,
		opsService:     opsService,
	}
}

// featureEnabled 返回 available-channels 开关是否启用。默认关闭（opt-in）。
func (h *AvailableChannelHandler) featureEnabled(c *gin.Context) bool {
	if h.settingService == nil {
		return false
	}
	return h.settingService.GetAvailableChannelsRuntime(c.Request.Context()).Enabled
}

// userAvailableGroup 用户可见的分组概要（白名单字段）。
//
// 前端据此区分专属 vs 公开分组（IsExclusive）、订阅 vs 标准分组（SubscriptionType，
// 订阅视觉加深），并展示默认倍率与高峰倍率规则；用户专属倍率前端走
// /groups/rates，和 API 密钥页面保持一致。
type userAvailableGroup struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Platform           string  `json:"platform"`
	SubscriptionType   string  `json:"subscription_type"`
	RateMultiplier     float64 `json:"rate_multiplier"`
	PeakRateEnabled    bool    `json:"peak_rate_enabled"`
	PeakStart          string  `json:"peak_start"`
	PeakEnd            string  `json:"peak_end"`
	PeakRateMultiplier float64 `json:"peak_rate_multiplier"`
	IsExclusive        bool    `json:"is_exclusive"`
}

// userSupportedModelPricing 用户可见的定价字段白名单。
type userSupportedModelPricing struct {
	BillingMode      string                   `json:"billing_mode"`
	InputPrice       *float64                 `json:"input_price"`
	OutputPrice      *float64                 `json:"output_price"`
	CacheWritePrice  *float64                 `json:"cache_write_price"`
	CacheReadPrice   *float64                 `json:"cache_read_price"`
	ImageInputPrice  *float64                 `json:"image_input_price"`
	ImageOutputPrice *float64                 `json:"image_output_price"`
	PerRequestPrice  *float64                 `json:"per_request_price"`
	Intervals        []userPricingIntervalDTO `json:"intervals"`
}

// userPricingIntervalDTO 定价区间白名单（去掉内部 ID、SortOrder 等前端不渲染的字段）。
type userPricingIntervalDTO struct {
	MinTokens       int      `json:"min_tokens"`
	MaxTokens       *int     `json:"max_tokens"`
	TierLabel       string   `json:"tier_label,omitempty"`
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheWritePrice *float64 `json:"cache_write_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
	PerRequestPrice *float64 `json:"per_request_price"`
}

// userSupportedModel 用户可见的支持模型条目。
type userSupportedModel struct {
	Name                    string                       `json:"name"`
	Platform                string                       `json:"platform"`
	Pricing                 *userSupportedModelPricing   `json:"pricing"`
	AccountPricingOverrides []userAccountPricingOverride `json:"account_pricing_overrides,omitempty"`
	Performance             *userModelPerformance        `json:"performance,omitempty"`
}

type userAccountPricingOverride struct {
	GroupID     int64                      `json:"group_id"`
	AccountName string                     `json:"account_name"`
	Pricing     *userSupportedModelPricing `json:"pricing"`
}

type userModelPerformance struct {
	WindowHours        int                         `json:"window_hours"`
	SuccessRate        float64                     `json:"success_rate"`
	SuccessCount       int64                       `json:"success_count"`
	FailureCount       int64                       `json:"failure_count"`
	SampleCount        int64                       `json:"sample_count"`
	LatencySampleCount int64                       `json:"latency_sample_count"`
	TTFTSampleCount    int64                       `json:"ttft_sample_count"`
	AvgLatencyMs       *int                        `json:"avg_latency_ms"`
	AvgTTFTMs          *int                        `json:"avg_ttft_ms"`
	GroupBreakdown     []userModelGroupPerformance `json:"groups"`
}

type userModelGroupPerformance struct {
	GroupID            int64   `json:"group_id"`
	SuccessRate        float64 `json:"success_rate"`
	SuccessCount       int64   `json:"success_count"`
	FailureCount       int64   `json:"failure_count"`
	SampleCount        int64   `json:"sample_count"`
	LatencySampleCount int64   `json:"latency_sample_count"`
	TTFTSampleCount    int64   `json:"ttft_sample_count"`
	AvgLatencyMs       *int    `json:"avg_latency_ms"`
	AvgTTFTMs          *int    `json:"avg_ttft_ms"`
}

// userChannelPlatformSection 单渠道内某个平台的子视图：用户可见的分组 + 该平台
// 支持的模型。按 platform 聚合后让前端可以把渠道名作为 row-group 一次渲染，
// 后面的平台行按 sections 顺序铺开。
type userChannelPlatformSection struct {
	Platform        string               `json:"platform"`
	Groups          []userAvailableGroup `json:"groups"`
	SupportedModels []userSupportedModel `json:"supported_models"`
}

// userAvailableChannel 用户可见的渠道条目（白名单字段）。
//
// 每个渠道聚合为一条记录，内嵌 platforms 子数组：每个 section 对应一个平台，
// 包含该平台的 groups 和 supported_models。
type userAvailableChannel struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Platforms   []userChannelPlatformSection `json:"platforms"`
}

// List 列出当前用户可见的「可用渠道」。
// GET /api/v1/channels/available
func (h *AvailableChannelHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Feature 未启用时返回空数组（不暴露渠道信息）。检查放在认证之后，
	// 保持与未开关前的 401 行为一致：未登录先 401，登录后再按开关决定。
	if !h.featureEnabled(c) {
		response.Success(c, []userAvailableChannel{})
		return
	}

	userGroups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	allowedGroupIDs := make(map[int64]struct{}, len(userGroups))
	for i := range userGroups {
		allowedGroupIDs[userGroups[i].ID] = struct{}{}
	}

	channels, err := h.channelService.ListAvailable(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]userAvailableChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Status != service.StatusActive {
			continue
		}
		visibleGroups := filterUserVisibleGroups(ch.Groups, allowedGroupIDs)
		if len(visibleGroups) == 0 {
			continue
		}
		sections := buildPlatformSections(ch, visibleGroups)
		if len(sections) == 0 {
			continue
		}
		out = append(out, userAvailableChannel{
			Name:        ch.Name,
			Description: ch.Description,
			Platforms:   sections,
		})
	}
	if len(out) == 0 {
		out = h.buildGroupModelFallback(c, userGroups)
	}
	h.attachModelPerformance(c, out, userGroups)

	response.Success(c, out)
}

func (h *AvailableChannelHandler) attachModelPerformance(
	c *gin.Context,
	channels []userAvailableChannel,
	groups []service.Group,
) {
	if h.opsService == nil || len(channels) == 0 || len(groups) == 0 {
		return
	}
	groupIDs := make([]int64, 0, len(groups))
	for i := range groups {
		if groups[i].ID > 0 {
			groupIDs = append(groupIDs, groups[i].ID)
		}
	}
	stats, err := h.opsService.GetModelPerformance(c.Request.Context(), groupIDs)
	if err != nil {
		slog.WarnContext(c.Request.Context(), "model square performance query failed", "error", err)
		return
	}
	attachModelPerformanceStats(channels, stats)
}

func attachModelPerformanceStats(channels []userAvailableChannel, stats []service.ModelPerformanceStat) {
	byModelGroup := make(map[string]service.ModelPerformanceStat, len(stats))
	for _, stat := range stats {
		key := modelPerformanceKey(stat.Model, stat.GroupID)
		if key != "" {
			byModelGroup[key] = stat
		}
	}

	for channelIndex := range channels {
		for sectionIndex := range channels[channelIndex].Platforms {
			section := &channels[channelIndex].Platforms[sectionIndex]
			for modelIndex := range section.SupportedModels {
				model := &section.SupportedModels[modelIndex]
				matched := make([]service.ModelPerformanceStat, 0, len(section.Groups))
				for _, group := range section.Groups {
					if stat, ok := byModelGroup[modelPerformanceKey(model.Name, group.ID)]; ok {
						matched = append(matched, stat)
					}
				}
				model.Performance = buildUserModelPerformance(matched)
			}
		}
	}
}

func modelPerformanceKey(model string, groupID int64) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" || groupID <= 0 {
		return ""
	}
	return model + "\x00" + fmt.Sprint(groupID)
}

func buildUserModelPerformance(stats []service.ModelPerformanceStat) *userModelPerformance {
	if len(stats) == 0 {
		return nil
	}
	result := &userModelPerformance{
		WindowHours:    service.ModelPerformanceWindowHours,
		GroupBreakdown: make([]userModelGroupPerformance, 0, len(stats)),
	}
	var latencyWeightedTotal int64
	var latencySamples int64
	var ttftWeightedTotal int64
	var ttftSamples int64
	for _, stat := range stats {
		sampleCount := stat.SuccessCount + stat.FailureCount
		if sampleCount <= 0 {
			continue
		}
		result.SuccessCount += stat.SuccessCount
		result.FailureCount += stat.FailureCount
		result.GroupBreakdown = append(result.GroupBreakdown, userModelGroupPerformance{
			GroupID:            stat.GroupID,
			SuccessRate:        modelSuccessRate(stat.SuccessCount, sampleCount),
			SuccessCount:       stat.SuccessCount,
			FailureCount:       stat.FailureCount,
			SampleCount:        sampleCount,
			LatencySampleCount: stat.LatencySampleCount,
			TTFTSampleCount:    stat.TTFTSampleCount,
			AvgLatencyMs:       stat.AvgLatencyMs,
			AvgTTFTMs:          stat.AvgTTFTMs,
		})
		if stat.AvgLatencyMs != nil && stat.LatencySampleCount > 0 {
			latencyWeightedTotal += int64(*stat.AvgLatencyMs) * stat.LatencySampleCount
			latencySamples += stat.LatencySampleCount
		}
		if stat.AvgTTFTMs != nil && stat.TTFTSampleCount > 0 {
			ttftWeightedTotal += int64(*stat.AvgTTFTMs) * stat.TTFTSampleCount
			ttftSamples += stat.TTFTSampleCount
		}
	}
	result.SampleCount = result.SuccessCount + result.FailureCount
	result.LatencySampleCount = latencySamples
	result.TTFTSampleCount = ttftSamples
	if result.SampleCount <= 0 {
		return nil
	}
	result.SuccessRate = modelSuccessRate(result.SuccessCount, result.SampleCount)
	if latencySamples > 0 {
		value := int(math.Round(float64(latencyWeightedTotal) / float64(latencySamples)))
		result.AvgLatencyMs = &value
	}
	if ttftSamples > 0 {
		value := int(math.Round(float64(ttftWeightedTotal) / float64(ttftSamples)))
		result.AvgTTFTMs = &value
	}
	sort.Slice(result.GroupBreakdown, func(i, j int) bool {
		return result.GroupBreakdown[i].GroupID < result.GroupBreakdown[j].GroupID
	})
	return result
}

func modelSuccessRate(successCount, sampleCount int64) float64 {
	if sampleCount <= 0 {
		return 0
	}
	return math.Round(float64(successCount)/float64(sampleCount)*10000) / 100
}

// buildGroupModelFallback keeps the model catalog useful on installations that
// have schedulable account groups but have not configured billing channels yet.
// Each synthetic row represents exactly one group so model visibility is not
// accidentally broadened across groups on the same platform.
func (h *AvailableChannelHandler) buildGroupModelFallback(c *gin.Context, groups []service.Group) []userAvailableChannel {
	out := make([]userAvailableChannel, 0, len(groups))
	for i := range groups {
		group := &groups[i]
		if group.ActiveAccountCount <= 0 || strings.TrimSpace(group.Platform) == "" {
			continue
		}
		modelIDs := h.gatewayService.GetAdvertisedModelsForGroup(c.Request.Context(), group)
		models := h.channelService.BuildSupportedModelsForDisplay(group.Platform, modelIDs)
		if len(models) == 0 {
			continue
		}
		allowedGroupIDs := map[int64]struct{}{group.ID: {}}
		overrides := h.channelService.GetAccountModelPricingOverridesForGroup(c.Request.Context(), group.ID)
		out = append(out, userAvailableChannel{
			Name:        group.Name,
			Description: group.Description,
			Platforms: []userChannelPlatformSection{{
				Platform:        group.Platform,
				Groups:          []userAvailableGroup{toUserAvailableGroup(group)},
				SupportedModels: toUserSupportedModels(models, nil, allowedGroupIDs, overrides),
			}},
		})
	}
	return out
}

func toUserAvailableGroup(group *service.Group) userAvailableGroup {
	return userAvailableGroup{
		ID:                 group.ID,
		Name:               group.Name,
		Platform:           group.Platform,
		SubscriptionType:   group.SubscriptionType,
		RateMultiplier:     group.RateMultiplier,
		PeakRateEnabled:    group.PeakRateEnabled,
		PeakStart:          group.PeakStart,
		PeakEnd:            group.PeakEnd,
		PeakRateMultiplier: group.PeakRateMultiplier,
		IsExclusive:        group.IsExclusive,
	}
}

// buildPlatformSections 把一个渠道按 visibleGroups 的平台集合拆成有序的 section 列表：
// 每个 section 对应一个平台，只包含该平台的 groups 和 supported_models。
// 输出按 platform 字母序稳定排序，便于前端等效比较与回归测试。
func buildPlatformSections(
	ch service.AvailableChannel,
	visibleGroups []userAvailableGroup,
) []userChannelPlatformSection {
	groupsByPlatform := make(map[string][]userAvailableGroup, 4)
	for _, g := range visibleGroups {
		if g.Platform == "" {
			continue
		}
		groupsByPlatform[g.Platform] = append(groupsByPlatform[g.Platform], g)
	}
	if len(groupsByPlatform) == 0 {
		return nil
	}

	platforms := make([]string, 0, len(groupsByPlatform))
	for p := range groupsByPlatform {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

	sections := make([]userChannelPlatformSection, 0, len(platforms))
	for _, platform := range platforms {
		platformSet := map[string]struct{}{platform: {}}
		visibleGroupIDs := make(map[int64]struct{}, len(groupsByPlatform[platform]))
		for _, group := range groupsByPlatform[platform] {
			visibleGroupIDs[group.ID] = struct{}{}
		}
		sections = append(sections, userChannelPlatformSection{
			Platform:        platform,
			Groups:          groupsByPlatform[platform],
			SupportedModels: toUserSupportedModels(ch.SupportedModels, platformSet, visibleGroupIDs, ch.AccountPricingOverrides),
		})
	}
	return sections
}

// filterUserVisibleGroups 仅保留用户可访问的分组。
func filterUserVisibleGroups(
	groups []service.AvailableGroupRef,
	allowed map[int64]struct{},
) []userAvailableGroup {
	visible := make([]userAvailableGroup, 0, len(groups))
	for _, g := range groups {
		if _, ok := allowed[g.ID]; !ok {
			continue
		}
		visible = append(visible, userAvailableGroup{
			ID:                 g.ID,
			Name:               g.Name,
			Platform:           g.Platform,
			SubscriptionType:   g.SubscriptionType,
			RateMultiplier:     g.RateMultiplier,
			PeakRateEnabled:    g.PeakRateEnabled,
			PeakStart:          g.PeakStart,
			PeakEnd:            g.PeakEnd,
			PeakRateMultiplier: g.PeakRateMultiplier,
			IsExclusive:        g.IsExclusive,
		})
	}
	return visible
}

// toUserSupportedModels 将 service 层支持模型转换为用户 DTO（字段白名单）。
// 仅保留平台在 allowedPlatforms 中的条目，防止跨平台模型信息泄漏。
// allowedPlatforms 为 nil 时不做平台过滤（保留全部，供测试或明确无过滤场景使用）。
func toUserSupportedModels(
	src []service.SupportedModel,
	allowedPlatforms map[string]struct{},
	allowedGroupIDs map[int64]struct{},
	overrides []service.AccountModelPricingOverride,
) []userSupportedModel {
	out := make([]userSupportedModel, 0, len(src))
	for i := range src {
		m := src[i]
		if allowedPlatforms != nil {
			if _, ok := allowedPlatforms[m.Platform]; !ok {
				continue
			}
		}
		out = append(out, userSupportedModel{
			Name:                    m.Name,
			Platform:                m.Platform,
			Pricing:                 toUserPricing(m.Pricing),
			AccountPricingOverrides: toUserAccountPricingOverrides(m, allowedGroupIDs, overrides),
		})
	}
	return out
}

func toUserAccountPricingOverrides(
	model service.SupportedModel,
	allowedGroupIDs map[int64]struct{},
	overrides []service.AccountModelPricingOverride,
) []userAccountPricingOverride {
	result := make([]userAccountPricingOverride, 0)
	for _, override := range overrides {
		if allowedGroupIDs != nil {
			if _, ok := allowedGroupIDs[override.GroupID]; !ok {
				continue
			}
		}
		if override.Platform != model.Platform || !pricingIncludesExactModel(override.Pricing, model.Name) {
			continue
		}
		result = append(result, userAccountPricingOverride{
			GroupID:     override.GroupID,
			AccountName: override.AccountName,
			Pricing:     toUserPricing(&override.Pricing),
		})
	}
	return result
}

func pricingIncludesExactModel(pricing service.ChannelModelPricing, model string) bool {
	for _, candidate := range pricing.Models {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(model)) {
			return true
		}
	}
	return false
}

// toUserPricing 将 service 层定价转换为用户 DTO；入参为 nil 时返回 nil。
func toUserPricing(p *service.ChannelModelPricing) *userSupportedModelPricing {
	if p == nil {
		return nil
	}
	intervals := make([]userPricingIntervalDTO, 0, len(p.Intervals))
	for _, iv := range p.Intervals {
		intervals = append(intervals, userPricingIntervalDTO{
			MinTokens:       iv.MinTokens,
			MaxTokens:       iv.MaxTokens,
			TierLabel:       iv.TierLabel,
			InputPrice:      iv.InputPrice,
			OutputPrice:     iv.OutputPrice,
			CacheWritePrice: iv.CacheWritePrice,
			CacheReadPrice:  iv.CacheReadPrice,
			PerRequestPrice: iv.PerRequestPrice,
		})
	}
	billingMode := string(p.BillingMode)
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	return &userSupportedModelPricing{
		BillingMode:      billingMode,
		InputPrice:       p.InputPrice,
		OutputPrice:      p.OutputPrice,
		CacheWritePrice:  p.CacheWritePrice,
		CacheReadPrice:   p.CacheReadPrice,
		ImageInputPrice:  p.ImageInputPrice,
		ImageOutputPrice: p.ImageOutputPrice,
		PerRequestPrice:  p.PerRequestPrice,
		Intervals:        intervals,
	}
}
