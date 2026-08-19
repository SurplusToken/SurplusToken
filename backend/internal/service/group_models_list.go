package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

func normalizeGroupModelsListConfig(cfg GroupModelsListConfig) GroupModelsListConfig {
	out := GroupModelsListConfig{Enabled: cfg.Enabled}
	if len(cfg.Models) == 0 {
		return out
	}

	seen := make(map[string]struct{}, len(cfg.Models))
	out.Models = make([]string, 0, len(cfg.Models))
	for _, model := range cfg.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out.Models = append(out.Models, model)
	}
	if len(out.Models) == 0 {
		out.Models = nil
	}
	return out
}

func (g *Group) CustomModelsListEnabled() bool {
	return g != nil && g.ModelsListConfig.Enabled && len(g.ModelsListConfig.Models) > 0
}

// GetAdvertisedModelsForGroup resolves the concrete model IDs that /v1/models
// would advertise for a group: schedulable account mappings first, then the
// optional group allow-list, and finally the platform defaults.
func (s *GatewayService) GetAdvertisedModelsForGroup(ctx context.Context, group *Group) []string {
	if s == nil || group == nil {
		return nil
	}
	groupID := group.ID
	available := s.GetAvailableModels(ctx, &groupID, group.Platform)
	models := resolveAdvertisedModelsForGroup(group, available, SharingRateActiveFromContext(ctx))
	return s.FilterModelsWithUsablePricing(ctx, &groupID, models)
}

// FilterModelsWithUsablePricing removes models that would fail the strict
// channel -> LiteLLM pricing gate.
func (s *GatewayService) FilterModelsWithUsablePricing(ctx context.Context, groupID *int64, models []string) []string {
	if s == nil || s.resolver == nil {
		return cloneStringSlice(models)
	}
	filtered := make([]string, 0, len(models))
	for _, model := range models {
		if s.resolver.HasUsablePricing(ctx, PricingInput{Model: model, GroupID: groupID}) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func resolveAdvertisedModelsForGroup(group *Group, available []string, sharingFilterActive bool) []string {
	if group == nil {
		return nil
	}
	if sharingFilterActive && available != nil && len(available) == 0 {
		return []string{}
	}

	defaults := defaultAdvertisedModelIDsForPlatform(group.Platform)
	if group.CustomModelsListEnabled() {
		source := available
		customDefaults := defaults
		if group.Platform == PlatformAnthropic {
			customDefaults = mergeAdvertisedModelIDs(defaults, defaultAdvertisedModelIDsForPlatform(PlatformAntigravity))
			if len(source) > 0 {
				source = mergeAdvertisedModelIDs(source, customDefaults)
			}
		}
		return filterAdvertisedModels(source, customDefaults, group.ModelsListConfig.Models)
	}
	if len(available) > 0 {
		return cloneStringSlice(available)
	}
	return defaults
}

func defaultAdvertisedModelIDsForPlatform(platform string) []string {
	switch platform {
	case PlatformOpenAI:
		return openai.DefaultModelIDs()
	case PlatformGemini:
		ids := make([]string, 0, len(geminicli.DefaultModels))
		for _, model := range geminicli.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformAntigravity:
		models := antigravity.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformGrok:
		return xai.DefaultModelIDs()
	default:
		ids := make([]string, 0, len(claude.DefaultModels))
		for _, model := range claude.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	}
}

func filterAdvertisedModels(available, defaults, selected []string) []string {
	if len(selected) == 0 {
		return cloneStringSlice(available)
	}
	source := available
	if len(source) == 0 {
		source = defaults
	}
	if len(source) == 0 {
		return nil
	}

	allowed := make([]string, 0, len(source))
	for _, model := range source {
		if model = strings.TrimSpace(model); model != "" {
			allowed = append(allowed, model)
		}
	}

	seen := make(map[string]struct{}, len(selected))
	filtered := make([]string, 0, len(selected))
	for _, model := range selected {
		model = strings.TrimSpace(model)
		if model == "" || !advertisedModelsAllow(allowed, model) {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		filtered = append(filtered, model)
	}
	return filtered
}

func advertisedModelsAllow(patterns []string, model string) bool {
	for _, pattern := range patterns {
		if pattern == model || (strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*"))) {
			return true
		}
	}
	return false
}

func mergeAdvertisedModelIDs(primary, secondary []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	merged := make([]string, 0, len(primary)+len(secondary))
	for _, models := range [][]string{primary, secondary} {
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			if _, ok := seen[model]; ok {
				continue
			}
			seen[model] = struct{}{}
			merged = append(merged, model)
		}
	}
	return merged
}
