package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// AvailableGroupRef 渠道视图中关联分组的简要信息。
//
// 用户侧「可用渠道」页面据此展示：专属分组 vs 公开分组（IsExclusive）、
// 订阅 vs 标准（SubscriptionType）、默认倍率（RateMultiplier）与高峰倍率规则。
// 用户专属倍率不在这里暴露，前端自己通过 /groups/rates 拉取，和 API 密钥页面保持一致。
type AvailableGroupRef struct {
	ID                 int64
	Name               string
	Platform           string
	SubscriptionType   string
	RateMultiplier     float64
	PeakRateEnabled    bool
	PeakStart          string
	PeakEnd            string
	PeakRateMultiplier float64
	IsExclusive        bool
}

// AvailableChannel 可用渠道视图：用于「可用渠道」页面展示渠道基础信息 +
// 关联的分组 + 推导出的支持模型列表（无通配符）。
type AvailableChannel struct {
	ID                      int64
	Name                    string
	Description             string
	Status                  string
	BillingModelSource      string
	RestrictModels          bool
	Groups                  []AvailableGroupRef
	SupportedModels         []SupportedModel
	AccountPricingOverrides []AccountModelPricingOverride
}

// ListAvailable 返回所有渠道的可用视图：每个渠道附带关联分组信息与支持模型列表。
//
// 支持模型通过 (*Channel).SupportedModels() 计算（mapping ∪ pricing 并联）。
// 对于渠道未配置定价的模型，进一步用 PricingService 的全局 LiteLLM 数据合成
// 一份展示用定价，让用户看到默认价格而非"未配置"。
//
// 关联分组信息通过 groupRepo.ListActive 查询后按 ID 映射；渠道 GroupIDs 中未在活跃列表中
// 的分组（已停用或删除）会被忽略。
//
// 前置条件：s.groupRepo 必须非 nil（由 wire DI 保证）。直接 nil-deref 用于 fail-fast，
// 避免静默掩盖注入缺失。
func (s *ChannelService) ListAvailable(ctx context.Context) ([]AvailableChannel, error) {
	channels, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}

	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}
	groupByID := make(map[int64]AvailableGroupRef, len(groups))
	for i := range groups {
		g := groups[i]
		groupByID[g.ID] = AvailableGroupRef{
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
		}
	}

	var accountOverrides []AccountModelPricingOverride
	if repo, ok := s.repo.(accountModelPricingOverrideRepository); ok {
		accountOverrides, err = repo.ListAccountModelPricingOverrides(ctx)
		if err != nil {
			return nil, fmt.Errorf("list account model pricing overrides: %w", err)
		}
	}

	out := make([]AvailableChannel, 0, len(channels))
	for i := range channels {
		ch := &channels[i]
		groups := make([]AvailableGroupRef, 0, len(ch.GroupIDs))
		for _, gid := range ch.GroupIDs {
			if ref, ok := groupByID[gid]; ok {
				groups = append(groups, ref)
			}
		}
		sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

		ch.normalizeBillingModelSource()

		supported := ch.SupportedModels()
		s.fillGlobalPricingFallback(supported)
		supported = filterUsableSupportedModels(supported)

		out = append(out, AvailableChannel{
			ID:                      ch.ID,
			Name:                    ch.Name,
			Description:             ch.Description,
			Status:                  ch.Status,
			BillingModelSource:      ch.BillingModelSource,
			RestrictModels:          ch.RestrictModels,
			Groups:                  groups,
			SupportedModels:         supported,
			AccountPricingOverrides: accountOverridesForGroups(accountOverrides, ch.GroupIDs),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func accountOverridesForGroups(overrides []AccountModelPricingOverride, groupIDs []int64) []AccountModelPricingOverride {
	allowed := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		allowed[groupID] = struct{}{}
	}
	result := make([]AccountModelPricingOverride, 0)
	for _, override := range overrides {
		if _, ok := allowed[override.GroupID]; ok {
			result = append(result, override)
		}
	}
	return result
}

// fillGlobalPricingFallback 对未命中渠道定价的支持模型，从全局 LiteLLM 数据合成一份
// 展示用定价。仅用于「可用渠道」展示，不影响真实计费链路。
//
// 仅当 Pricing == nil 时回落。渠道一旦声明定价，就作为原子配置处理，
// 不从 LiteLLM 补齐缺失字段。
func (s *ChannelService) fillGlobalPricingFallback(models []SupportedModel) {
	if s.pricingService == nil {
		return
	}
	for i := range models {
		if models[i].Pricing != nil {
			continue
		}
		lp := s.pricingService.GetModelPricing(models[i].Name)
		if !liteLLMTokenPricingUsable(lp) && !liteLLMImagePricingUsable(lp) {
			continue
		}
		models[i].Pricing = synthesizePricingFromLiteLLM(lp, models[i].Pricing)
	}
}

// BuildSupportedModelsForDisplay converts resolved model IDs into the same
// display shape used by configured channels and fills prices from LiteLLM.
// Trailing-wildcard mappings are expanded against the platform defaults so the
// user-facing catalog only contains model IDs that can be called directly.
func (s *ChannelService) BuildSupportedModelsForDisplay(platform string, modelIDs []string) []SupportedModel {
	expanded := make([]string, 0, len(modelIDs))
	defaults := defaultAdvertisedModelIDsForPlatform(platform)
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		if prefix, wildcard := splitWildcardSuffix(modelID); wildcard {
			for _, candidate := range defaults {
				if strings.HasPrefix(candidate, prefix) {
					expanded = append(expanded, candidate)
				}
			}
			continue
		}
		expanded = append(expanded, modelID)
	}

	seen := make(map[string]struct{}, len(expanded))
	models := make([]SupportedModel, 0, len(expanded))
	for _, modelID := range expanded {
		key := strings.ToLower(modelID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		models = append(models, SupportedModel{Name: modelID, Platform: platform})
	}
	sort.SliceStable(models, func(i, j int) bool {
		return strings.ToLower(models[i].Name) < strings.ToLower(models[j].Name)
	})
	s.fillGlobalPricingFallback(models)
	return filterUsableSupportedModels(models)
}

func filterUsableSupportedModels(models []SupportedModel) []SupportedModel {
	filtered := make([]SupportedModel, 0, len(models))
	for _, model := range models {
		if channelPricingUsable(model.Pricing) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func pricingNeedsFallback(p *ChannelModelPricing) bool {
	return p == nil
}

// synthesizePricingFromLiteLLM 把 LiteLLM 的定价数据转成 ChannelModelPricing 形态，
// 仅用于展示。
//
// 计费模式优先级：
//  1. 渠道已选 BillingMode（admin 在 UI 里选了 image / per_request 但没填价的场景，
//     按选定模式合成对应字段）
//  2. LiteLLM mode="image_generation" → image
//  3. 默认 token
//
// LiteLLM 中字段 0 视为未配置，不带入展示。
func synthesizePricingFromLiteLLM(lp *LiteLLMModelPricing, existing *ChannelModelPricing) *ChannelModelPricing {
	if lp == nil {
		return existing
	}

	mode := BillingModeToken
	switch {
	case existing != nil && existing.BillingMode != "":
		mode = existing.BillingMode
	case lp.Mode == "image_generation":
		mode = BillingModeImage
	}

	if mode == BillingModeImage || mode == BillingModePerRequest {
		return &ChannelModelPricing{
			BillingMode:      mode,
			PerRequestPrice:  nonZeroPtr(lp.OutputCostPerImage),
			ImageOutputPrice: nonZeroPtr(lp.OutputCostPerImageToken),
			InputPrice:       nonZeroPtr(lp.InputCostPerToken),
			OutputPrice:      nonZeroPtr(lp.OutputCostPerToken),
		}
	}
	return &ChannelModelPricing{
		BillingMode:      mode,
		InputPrice:       nonZeroPtr(lp.InputCostPerToken),
		OutputPrice:      nonZeroPtr(lp.OutputCostPerToken),
		CacheWritePrice:  nonZeroPtr(lp.CacheCreationInputTokenCost),
		CacheReadPrice:   nonZeroPtr(lp.CacheReadInputTokenCost),
		ImageOutputPrice: nonZeroPtr(lp.OutputCostPerImageToken),
	}
}

func nonZeroPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}
