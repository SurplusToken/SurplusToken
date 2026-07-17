package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/kimi"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/zhipu"
)

const (
	defaultOpenAIMessagesDispatchOpusMappedModel   = "gpt-5.4"
	defaultOpenAIMessagesDispatchSonnetMappedModel = "gpt-5.3-codex"
	defaultOpenAIMessagesDispatchHaikuMappedModel  = "gpt-5.4-mini"
)

func normalizeOpenAIMessagesDispatchMappedModel(model string) string {
	model = NormalizeOpenAICompatRequestedModel(strings.TrimSpace(model))
	return strings.TrimSpace(model)
}

func normalizeOpenAIMessagesDispatchModelConfig(cfg OpenAIMessagesDispatchModelConfig) OpenAIMessagesDispatchModelConfig {
	out := OpenAIMessagesDispatchModelConfig{
		OpusMappedModel:   normalizeOpenAIMessagesDispatchMappedModel(cfg.OpusMappedModel),
		SonnetMappedModel: normalizeOpenAIMessagesDispatchMappedModel(cfg.SonnetMappedModel),
		HaikuMappedModel:  normalizeOpenAIMessagesDispatchMappedModel(cfg.HaikuMappedModel),
	}

	if len(cfg.ExactModelMappings) > 0 {
		out.ExactModelMappings = make(map[string]string, len(cfg.ExactModelMappings))
		for requestedModel, mappedModel := range cfg.ExactModelMappings {
			requestedModel = strings.TrimSpace(requestedModel)
			mappedModel = normalizeOpenAIMessagesDispatchMappedModel(mappedModel)
			if requestedModel == "" || mappedModel == "" {
				continue
			}
			out.ExactModelMappings[requestedModel] = mappedModel
		}
		if len(out.ExactModelMappings) == 0 {
			out.ExactModelMappings = nil
		}
	}

	return out
}

func claudeMessagesDispatchFamily(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(normalized, "claude") {
		return ""
	}
	switch {
	case strings.Contains(normalized, "opus"):
		return "opus"
	case strings.Contains(normalized, "sonnet"):
		return "sonnet"
	case strings.Contains(normalized, "haiku"):
		return "haiku"
	default:
		return ""
	}
}

func (g *Group) ResolveMessagesDispatchModel(requestedModel string) string {
	if g == nil {
		return ""
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return ""
	}

	if g.Platform == PlatformGrok {
		if claudeMessagesDispatchFamily(requestedModel) != "" {
			return xai.DefaultModelMapping()["grok"]
		}
		return ""
	}
	if g.Platform == PlatformKimi {
		cfg := normalizeOpenAIMessagesDispatchModelConfig(g.MessagesDispatchModelConfig)
		if mappedModel := strings.TrimSpace(cfg.ExactModelMappings[requestedModel]); mappedModel != "" {
			return mappedModel
		}
		if claudeMessagesDispatchFamily(requestedModel) != "" {
			switch claudeMessagesDispatchFamily(requestedModel) {
			case "opus":
				if cfg.OpusMappedModel != "" && !strings.HasPrefix(cfg.OpusMappedModel, "gpt-") {
					return cfg.OpusMappedModel
				}
			case "sonnet":
				if cfg.SonnetMappedModel != "" && !strings.HasPrefix(cfg.SonnetMappedModel, "gpt-") {
					return cfg.SonnetMappedModel
				}
			case "haiku":
				if cfg.HaikuMappedModel != "" && !strings.HasPrefix(cfg.HaikuMappedModel, "gpt-") {
					return cfg.HaikuMappedModel
				}
			}
			return kimi.CodeModel
		}
		return ""
	}
	if g.Platform == PlatformZhipu {
		cfg := normalizeOpenAIMessagesDispatchModelConfig(g.MessagesDispatchModelConfig)
		if mappedModel := strings.TrimSpace(cfg.ExactModelMappings[requestedModel]); mappedModel != "" {
			return mappedModel
		}
		switch claudeMessagesDispatchFamily(requestedModel) {
		case "opus", "sonnet":
			if mapped := strings.TrimSpace(map[string]string{
				"opus": cfg.OpusMappedModel, "sonnet": cfg.SonnetMappedModel,
			}[claudeMessagesDispatchFamily(requestedModel)]); mapped != "" && !strings.HasPrefix(mapped, "gpt-") {
				return mapped
			}
			return zhipu.DefaultTestModel
		case "haiku":
			if mapped := strings.TrimSpace(cfg.HaikuMappedModel); mapped != "" && !strings.HasPrefix(mapped, "gpt-") {
				return mapped
			}
			return "glm-4.7"
		}
		return ""
	}

	cfg := normalizeOpenAIMessagesDispatchModelConfig(g.MessagesDispatchModelConfig)
	if mappedModel := strings.TrimSpace(cfg.ExactModelMappings[requestedModel]); mappedModel != "" {
		return mappedModel
	}

	switch claudeMessagesDispatchFamily(requestedModel) {
	case "opus":
		if mappedModel := strings.TrimSpace(cfg.OpusMappedModel); mappedModel != "" {
			return mappedModel
		}
		return defaultOpenAIMessagesDispatchOpusMappedModel
	case "sonnet":
		if mappedModel := strings.TrimSpace(cfg.SonnetMappedModel); mappedModel != "" {
			return mappedModel
		}
		return defaultOpenAIMessagesDispatchSonnetMappedModel
	case "haiku":
		if mappedModel := strings.TrimSpace(cfg.HaikuMappedModel); mappedModel != "" {
			return mappedModel
		}
		return defaultOpenAIMessagesDispatchHaikuMappedModel
	default:
		return ""
	}
}

func sanitizeGroupMessagesDispatchFields(g *Group) {
	if g == nil || g.Platform == PlatformOpenAI || g.Platform == PlatformKimi || g.Platform == PlatformZhipu {
		return
	}
	g.AllowMessagesDispatch = false
	g.DefaultMappedModel = ""
	g.MessagesDispatchModelConfig = OpenAIMessagesDispatchModelConfig{}
}
