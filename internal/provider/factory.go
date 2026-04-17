package provider

import (
	"context"
	"fmt"
	"strings"

	"bytemind/internal/config"
	"bytemind/internal/llm"
)

func NewClient(cfg config.ProviderConfig) (llm.Client, error) {
	typ := strings.ToLower(strings.TrimSpace(cfg.Type))
	clientCfg := Config{
		Type:             typ,
		BaseURL:          cfg.BaseURL,
		APIPath:          cfg.APIPath,
		APIKey:           cfg.ResolveAPIKey(),
		AuthHeader:       cfg.AuthHeader,
		AuthScheme:       cfg.AuthScheme,
		ExtraHeaders:     cfg.ExtraHeaders,
		Model:            cfg.Model,
		AnthropicVersion: cfg.AnthropicVersion,
	}

	switch typ {
	case "openai-compatible", "openai":
		return NewOpenAICompatible(clientCfg), nil
	case "anthropic":
		return NewAnthropic(clientCfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider type %q", cfg.Type)
	}
}

func NewDomainClient(cfg config.ProviderConfig) (Client, error) {
	providerID := ProviderID(strings.ToLower(strings.TrimSpace(cfg.Type)))
	if providerID == "openai-compatible" || providerID == "openai" {
		providerID = ProviderOpenAI
	}
	if providerID == "anthropic" {
		providerID = ProviderAnthropic
	}
	if providerID == "" {
		providerID = ProviderID("unknown")
	}
	return NewDomainClientWithID(providerID, cfg)
}

func NewDomainClientWithID(providerID ProviderID, cfg config.ProviderConfig) (Client, error) {
	baseClient, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	id := ProviderID(strings.ToLower(strings.TrimSpace(string(providerID))))
	if id == "" {
		id = ProviderID("unknown")
	}
	return WrapClient(id, ModelID(strings.TrimSpace(cfg.Model)), baseClient), nil
}

func NewRouterClient(cfg config.ProviderRuntimeConfig, health HealthChecker) (llm.Client, error) {
	reg, err := NewRegistry(cfg)
	if err != nil {
		return nil, err
	}
	if health == nil {
		health = NewHealthChecker(HealthConfigFromRuntime(cfg.Health), newRuntimePreflightChecker(cfg))
	}
	return NewRoutedClientWithPolicy(NewRouter(reg, health, RouterConfig{
		DefaultProvider: ProviderID(cfg.DefaultProvider),
		DefaultModel:    ModelID(cfg.DefaultModel),
	}), health, cfg.AllowFallback), nil
}

func newRuntimePreflightChecker(cfg config.ProviderRuntimeConfig) func(context.Context, ProviderID) error {
	providers := make(map[ProviderID]config.ProviderConfig, len(cfg.Providers))
	for id, providerCfg := range cfg.Providers {
		providers[normalizeRegistryProviderID(id)] = providerCfg
	}
	return func(ctx context.Context, id ProviderID) error {
		normalizedID := normalizeRegistryProviderID(string(id))
		providerCfg, ok := providers[normalizedID]
		if !ok {
			return &Error{Code: ErrCodeProviderNotFound, Provider: normalizedID, Message: string(ErrCodeProviderNotFound), Retryable: false, Err: ErrProviderNotFound}
		}
		availability := CheckAvailability(ctx, providerCfg)
		if availability.Ready {
			return nil
		}
		return &Error{Code: ErrCodeUnavailable, Provider: normalizedID, Message: availability.Reason, Retryable: true, Detail: availability.Detail, Err: errorsUnavailable}
	}
}


	switch typ {
	case "openai-compatible", "openai":
		return NewOpenAICompatible(clientCfg), nil
	case "anthropic":
		return NewAnthropic(clientCfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider type %q", cfg.Type)
	}
}

func NewDomainClient(cfg config.ProviderConfig) (Client, error) {
	providerID := ProviderID(strings.ToLower(strings.TrimSpace(cfg.Type)))
	if providerID == "openai-compatible" || providerID == "openai" {
		providerID = ProviderOpenAI
	}
	if providerID == "anthropic" {
		providerID = ProviderAnthropic
	}
	if providerID == "" {
		providerID = ProviderID("unknown")
	}
	return NewDomainClientWithID(providerID, cfg)
}

func NewDomainClientWithID(providerID ProviderID, cfg config.ProviderConfig) (Client, error) {
	baseClient, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	id := ProviderID(strings.ToLower(strings.TrimSpace(string(providerID))))
	if id == "" {
		id = ProviderID("unknown")
	}
	return WrapClient(id, ModelID(strings.TrimSpace(cfg.Model)), baseClient), nil
}

func NewRouterClient(cfg config.ProviderRuntimeConfig, health HealthChecker) (llm.Client, error) {
	reg, err := NewRegistry(cfg)
	if err != nil {
		return nil, err
	}
<<<<<<< HEAD
=======
	if health == nil {
		health = NewHealthChecker(HealthConfigFromRuntime(cfg.Health), newRuntimePreflightChecker(cfg))
	}
>>>>>>> 4c0fb2b (fix: add provider preflight health checks)
	return NewRoutedClientWithPolicy(NewRouter(reg, health, RouterConfig{
		DefaultProvider: ProviderID(cfg.DefaultProvider),
		DefaultModel:    ModelID(cfg.DefaultModel),
	}), cfg.AllowFallback), nil
}

func newRuntimePreflightChecker(cfg config.ProviderRuntimeConfig) func(context.Context, ProviderID) error {
	providers := make(map[ProviderID]config.ProviderConfig, len(cfg.Providers))
	for id, providerCfg := range cfg.Providers {
		providers[normalizeRegistryProviderID(id)] = providerCfg
	}
	return func(ctx context.Context, id ProviderID) error {
		normalizedID := normalizeRegistryProviderID(string(id))
		providerCfg, ok := providers[normalizedID]
		if !ok {
			return &Error{Code: ErrCodeProviderNotFound, Provider: normalizedID, Message: string(ErrCodeProviderNotFound), Retryable: false, Err: ErrProviderNotFound}
		}
		availability := CheckAvailability(ctx, providerCfg)
		if availability.Ready {
			return nil
		}
		return &Error{Code: ErrCodeUnavailable, Provider: normalizedID, Message: availability.Reason, Retryable: true, Detail: availability.Detail, Err: errorsUnavailable}
	}
}
