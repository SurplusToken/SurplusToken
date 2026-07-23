package service

import (
	"context"
	"math"
	"strings"
)

// PricingSource 定价来源标识
const (
	PricingSourceChannel     = "channel"
	PricingSourceLiteLLM     = "litellm"
	PricingSourceUnavailable = "unavailable"
)

// ResolvedPricing 统一定价解析结果
type ResolvedPricing struct {
	// Mode 计费模式
	Mode BillingMode

	// Token 模式：基础定价（来自 LiteLLM 或 fallback）
	BasePricing *ModelPricing

	// Token 模式：区间定价列表（如有，覆盖 BasePricing 中的对应字段）
	Intervals []PricingInterval

	// 按次/图片模式：分层定价
	RequestTiers []PricingInterval

	// 按次/图片模式：默认价格（未命中层级时使用）
	DefaultPerRequestPrice float64

	// 来源标识
	Source string // "channel", "litellm", "fallback"

	// 是否支持缓存细分
	SupportsCacheBreakdown bool

	// 渠道定价原始配置（用于区间模式下获取 ImageOutputPrice）
	channelPricing *ChannelModelPricing
}

// ModelPricingResolver 统一模型定价解析器。
// 解析链：Channel → LiteLLM。两者都不可用时拒绝计费。
type ModelPricingResolver struct {
	channelService *ChannelService
	billingService *BillingService
}

// NewModelPricingResolver 创建定价解析器实例
func NewModelPricingResolver(channelService *ChannelService, billingService *BillingService) *ModelPricingResolver {
	return &ModelPricingResolver{
		channelService: channelService,
		billingService: billingService,
	}
}

// PricingInput 定价解析输入
type PricingInput struct {
	Model     string
	GroupID   *int64 // nil 表示不检查渠道
	AccountID *int64 // 实际调度账号；nil 表示请求前校验或无账号覆盖
}

// Resolve 解析模型定价。
// 渠道定价是原子配置：一旦存在就不会用 LiteLLM 补齐缺失字段。
func (r *ModelPricingResolver) Resolve(ctx context.Context, input PricingInput) *ResolvedPricing {
	if input.GroupID != nil && r.channelService != nil {
		if input.AccountID != nil {
			accountPricing := r.channelService.GetAccountModelPricing(ctx, *input.GroupID, *input.AccountID, input.Model)
			if accountPricing != nil {
				return r.resolveChannelPricing(accountPricing)
			}
		}
		chPricing := r.channelService.GetChannelModelPricing(ctx, *input.GroupID, input.Model)
		if chPricing != nil {
			return r.resolveChannelPricing(chPricing)
		}
	}

	return r.resolveLiteLLMPricing(input.Model)
}

func (r *ModelPricingResolver) resolveChannelPricing(chPricing *ChannelModelPricing) *ResolvedPricing {
	mode := chPricing.BillingMode
	if mode == "" {
		mode = BillingModeToken
	}
	resolved := &ResolvedPricing{
		Mode:           mode,
		Source:         PricingSourceChannel,
		channelPricing: chPricing,
	}
	switch mode {
	case BillingModePerRequest, BillingModeImage:
		r.applyRequestTierOverrides(chPricing, resolved)
	default:
		resolved.Mode = BillingModeToken
		r.applyTokenOverrides(chPricing, resolved)
	}
	return resolved
}

func (r *ModelPricingResolver) resolveLiteLLMPricing(model string) *ResolvedPricing {
	unavailable := &ResolvedPricing{Mode: BillingModeToken, Source: PricingSourceUnavailable}
	if r == nil || r.billingService == nil {
		return unavailable
	}

	if r.billingService.pricingService != nil {
		lp := r.billingService.pricingService.GetModelPricing(strings.ToLower(strings.TrimSpace(model)))
		if liteLLMImagePricingUsable(lp) {
			return &ResolvedPricing{
				Mode:                   BillingModeImage,
				DefaultPerRequestPrice: lp.OutputCostPerImage,
				Source:                 PricingSourceLiteLLM,
			}
		}
	}

	pricing, err := r.billingService.getLiteLLMModelPricing(model)
	if err != nil {
		return unavailable
	}
	return &ResolvedPricing{
		Mode:                   BillingModeToken,
		BasePricing:            pricing,
		Source:                 PricingSourceLiteLLM,
		SupportsCacheBreakdown: pricing.SupportsCacheBreakdown,
	}
}

// HasUsablePricing reports whether a request can be billed without built-in prices.
func (r *ModelPricingResolver) HasUsablePricing(ctx context.Context, input PricingInput) bool {
	return resolvedPricingUsable(r.Resolve(ctx, input))
}

func resolvedPricingUsable(resolved *ResolvedPricing) bool {
	if resolved == nil || resolved.Source == PricingSourceUnavailable {
		return false
	}
	if resolved.Source == PricingSourceChannel {
		return channelPricingUsable(resolved.channelPricing)
	}
	switch resolved.Mode {
	case BillingModeImage, BillingModePerRequest:
		return validPrice(resolved.DefaultPerRequestPrice)
	default:
		return resolved.BasePricing != nil
	}
}

func channelPricingUsable(p *ChannelModelPricing) bool {
	if p == nil {
		return false
	}
	mode := p.BillingMode
	if mode == "" {
		mode = BillingModeToken
	}
	switch mode {
	case BillingModePerRequest, BillingModeImage:
		if !validPricePointer(p.PerRequestPrice) {
			return false
		}
		for i := range p.Intervals {
			if !validPricePointer(p.Intervals[i].PerRequestPrice) {
				return false
			}
		}
		return true
	default:
		if !validPricePointer(p.InputPrice) || !validPricePointer(p.OutputPrice) {
			return false
		}
		for i := range p.Intervals {
			if !validPricePointer(p.Intervals[i].InputPrice) || !validPricePointer(p.Intervals[i].OutputPrice) {
				return false
			}
		}
		return true
	}
}

func liteLLMTokenPricingUsable(p *LiteLLMModelPricing) bool {
	return p != nil && !p.TokenPricingAbsent && validPrice(p.InputCostPerToken) && validPrice(p.OutputCostPerToken)
}

func liteLLMImagePricingUsable(p *LiteLLMModelPricing) bool {
	return p != nil && p.Mode == "image_generation" && p.OutputCostPerImage > 0 && validPrice(p.OutputCostPerImage)
}

func validPricePointer(price *float64) bool {
	return price != nil && validPrice(*price)
}

func validPrice(price float64) bool {
	return !math.IsNaN(price) && !math.IsInf(price, 0) && price >= 0
}

// applyTokenOverrides 应用 token 模式的渠道覆盖
func (r *ModelPricingResolver) applyTokenOverrides(chPricing *ChannelModelPricing, resolved *ResolvedPricing) {
	resolved.Intervals = filterValidIntervals(chPricing.Intervals)
	resolved.BasePricing = &ModelPricing{}

	if chPricing.InputPrice != nil {
		resolved.BasePricing.InputPricePerToken = *chPricing.InputPrice
		resolved.BasePricing.InputPricePerTokenPriority = *chPricing.InputPrice
	}
	if chPricing.OutputPrice != nil {
		resolved.BasePricing.OutputPricePerToken = *chPricing.OutputPrice
		resolved.BasePricing.OutputPricePerTokenPriority = *chPricing.OutputPrice
	}
	if chPricing.CacheWritePrice != nil {
		resolved.BasePricing.CacheCreationPricePerToken = *chPricing.CacheWritePrice
		resolved.BasePricing.CacheCreationPricePerTokenPriority = *chPricing.CacheWritePrice
		resolved.BasePricing.CacheCreationPriceExplicit = true
		resolved.BasePricing.CacheCreation5mPrice = *chPricing.CacheWritePrice
		resolved.BasePricing.CacheCreation1hPrice = *chPricing.CacheWritePrice
	}
	if chPricing.CacheReadPrice != nil {
		resolved.BasePricing.CacheReadPricePerToken = *chPricing.CacheReadPrice
		resolved.BasePricing.CacheReadPricePerTokenPriority = *chPricing.CacheReadPrice
	}
	// 渠道定价覆盖一切：显式配置则用配置值，未配置则归零（不回退到 LiteLLM）
	if chPricing.ImageOutputPrice != nil {
		resolved.BasePricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
	} else {
		resolved.BasePricing.ImageOutputPricePerToken = 0
	}
	resolved.BasePricing.ImageOutputPriceExplicit = true
	applyChannelImageInputPrice(chPricing, resolved.BasePricing)
}

// applyChannelImageInputPrice 应用渠道图片输入价：显式配置则用配置值；
// 未配置时归零，使 computeTokenBreakdown 回退到文本输入价（向后兼容，
// 避免 commit 引入的 LiteLLM 图片输入价泄漏进渠道自定义定价）。
// 与 image_output 不同，此处不设 Explicit 标志——图片输入未配置应回退文本价，
// 而非硬置 0。
func applyChannelImageInputPrice(chPricing *ChannelModelPricing, pricing *ModelPricing) {
	if chPricing != nil && chPricing.ImageInputPrice != nil {
		pricing.ImageInputPricePerToken = *chPricing.ImageInputPrice
	} else {
		pricing.ImageInputPricePerToken = 0
	}
}

// applyRequestTierOverrides 应用按次/图片模式的渠道覆盖
func (r *ModelPricingResolver) applyRequestTierOverrides(chPricing *ChannelModelPricing, resolved *ResolvedPricing) {
	resolved.RequestTiers = filterValidIntervals(chPricing.Intervals)
	if chPricing.PerRequestPrice != nil {
		resolved.DefaultPerRequestPrice = *chPricing.PerRequestPrice
	}
}

// filterValidIntervals 过滤掉所有价格字段都为空的无效 interval。
// 前端可能创建了只有 min/max 但无价格的空 interval。
func filterValidIntervals(intervals []PricingInterval) []PricingInterval {
	var valid []PricingInterval
	for _, iv := range intervals {
		if iv.InputPrice != nil || iv.OutputPrice != nil ||
			iv.CacheWritePrice != nil || iv.CacheReadPrice != nil ||
			iv.PerRequestPrice != nil {
			valid = append(valid, iv)
		}
	}
	return valid
}

// GetIntervalPricing 根据 context token 数获取区间定价。
// 如果有区间列表，找到匹配区间并构造 ModelPricing；否则直接返回 BasePricing。
func (r *ModelPricingResolver) GetIntervalPricing(resolved *ResolvedPricing, totalContextTokens int) *ModelPricing {
	if len(resolved.Intervals) == 0 {
		return resolved.BasePricing
	}

	iv := FindMatchingInterval(resolved.Intervals, totalContextTokens)
	if iv == nil {
		return resolved.BasePricing
	}

	return intervalToModelPricing(iv, resolved.SupportsCacheBreakdown, resolved.channelPricing)
}

// intervalToModelPricing 将区间定价转换为 ModelPricing
func intervalToModelPricing(iv *PricingInterval, supportsCacheBreakdown bool, chPricing *ChannelModelPricing) *ModelPricing {
	pricing := &ModelPricing{
		SupportsCacheBreakdown: supportsCacheBreakdown,
	}
	if iv.InputPrice != nil {
		pricing.InputPricePerToken = *iv.InputPrice
		pricing.InputPricePerTokenPriority = *iv.InputPrice
	}
	if iv.OutputPrice != nil {
		pricing.OutputPricePerToken = *iv.OutputPrice
		pricing.OutputPricePerTokenPriority = *iv.OutputPrice
	}
	if iv.CacheWritePrice != nil {
		pricing.CacheCreationPricePerToken = *iv.CacheWritePrice
		pricing.CacheCreationPricePerTokenPriority = *iv.CacheWritePrice
		pricing.CacheCreationPriceExplicit = true
		pricing.CacheCreation5mPrice = *iv.CacheWritePrice
		pricing.CacheCreation1hPrice = *iv.CacheWritePrice
	}
	if iv.CacheReadPrice != nil {
		pricing.CacheReadPricePerToken = *iv.CacheReadPrice
		pricing.CacheReadPricePerTokenPriority = *iv.CacheReadPrice
	}
	// 渠道定价存在时，ImageOutputPrice 显式覆盖；图片输入价用渠道级配置
	// （区间不携带图片输入价，与 image_output 一致）。
	if chPricing != nil {
		pricing.ImageOutputPriceExplicit = true
		if chPricing.ImageOutputPrice != nil {
			pricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
		}
		applyChannelImageInputPrice(chPricing, pricing)
	}
	return pricing
}

// GetRequestTierPrice 根据层级标签获取按次价格
func (r *ModelPricingResolver) GetRequestTierPrice(resolved *ResolvedPricing, tierLabel string) float64 {
	for _, tier := range resolved.RequestTiers {
		if tier.TierLabel == tierLabel && tier.PerRequestPrice != nil {
			return *tier.PerRequestPrice
		}
	}
	return 0
}

// GetRequestTierPriceByContext 根据 context token 数获取按次价格
func (r *ModelPricingResolver) GetRequestTierPriceByContext(resolved *ResolvedPricing, totalContextTokens int) float64 {
	iv := FindMatchingInterval(resolved.RequestTiers, totalContextTokens)
	if iv != nil && iv.PerRequestPrice != nil {
		return *iv.PerRequestPrice
	}
	return 0
}
