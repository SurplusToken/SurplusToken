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
	return models
}

func resolveAdvertisedModelsForGroup(group *Group, available []string, sharingFilterActive bool) []string {
	if group == nil {
		return nil
	}
	if sharingFilterActive && available != nil && len(available) == 0 {
		return []string{}
	}

	defaults := defaultAdvertisedModelIDsForPlatform(group.Platform)
	if group.ModelAllowlistEnabled() {
		source := available
		customDefaults := defaults
		if group.Platform == PlatformAnthropic {
			customDefaults = mergeAdvertisedModelIDs(defaults, defaultAdvertisedModelIDsForPlatform(PlatformAntigravity))
			if len(source) > 0 {
				source = mergeAdvertisedModelIDs(source, customDefaults)
			}
		}
		if len(source) == 0 {
			source = customDefaults
		}
		return group.ModelAllowlist.FilterForListing(source)
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
