package config

import "strings"

type ProviderRuntimeConfig struct {
	DefaultProvider string                    `json:"default_provider"`
	DefaultModel    string                    `json:"default_model"`
	AllowFallback   bool                      `json:"allow_fallback"`
	Providers       map[string]ProviderConfig `json:"providers"`
}

func LegacyProviderRuntimeConfig(cfg ProviderConfig) ProviderRuntimeConfig {
	providerID := strings.ToLower(strings.TrimSpace(cfg.Type))
	switch providerID {
	case "", "openai", "openai-compatible":
		providerID = "openai"
	case "anthropic":
		providerID = "anthropic"
	}
	cfg.Type = providerID
	return ProviderRuntimeConfig{
		DefaultProvider: providerID,
		DefaultModel:    cfg.Model,
		AllowFallback:   false,
		Providers: map[string]ProviderConfig{
			providerID: cfg,
		},
	}
}
