package provider

import (
	"context"
	"errors"
	"testing"

	"bytemind/internal/config"
	"bytemind/internal/llm"
)

type stubHealthChecker struct {
	errors map[ProviderID]error
}

func (s stubHealthChecker) Check(_ context.Context, id ProviderID) error {
	if s.errors == nil {
		return nil
	}
	return s.errors[id]
}

type stubRouterClient struct {
	providerID ProviderID
	models     []ModelInfo
	streams    []stubRouterStreamResult
	streamReqs []llm.ChatRequest
}

type stubRouterStreamResult struct {
	message llm.Message
	err     error
	deltas  []string
}

func (s *stubRouterClient) ProviderID() ProviderID                          { return s.providerID }
func (s *stubRouterClient) ListModels(context.Context) ([]ModelInfo, error) { return s.models, nil }
func (s *stubRouterClient) Stream(_ context.Context, req Request) (<-chan Event, error) {
	idx := len(s.streamReqs)
	s.streamReqs = append(s.streamReqs, req.ChatRequest)
	result := stubRouterStreamResult{}
	if idx < len(s.streams) {
		result = s.streams[idx]
	}
	ch := make(chan Event, len(result.deltas)+3)
	go func() {
		defer close(ch)
		ch <- Event{Type: EventStart, ProviderID: s.providerID, ModelID: ModelID(req.Model), TraceID: req.TraceID}
		for _, delta := range result.deltas {
			ch <- Event{Type: EventDelta, ProviderID: s.providerID, ModelID: ModelID(req.Model), TraceID: req.TraceID, Delta: delta}
		}
		if result.err != nil {
			var providerErr *Error
			if errors.As(result.err, &providerErr) {
				ch <- Event{Type: EventError, ProviderID: s.providerID, ModelID: ModelID(req.Model), TraceID: req.TraceID, Error: providerErr}
				return
			}
			ch <- Event{Type: EventError, ProviderID: s.providerID, ModelID: ModelID(req.Model), TraceID: req.TraceID, Error: &Error{Code: ErrCodeUnavailable, Provider: s.providerID, Message: result.err.Error(), Retryable: false, Err: result.err}}
			return
		}
		message := result.message
		message.Normalize()
		ch <- Event{Type: EventResult, ProviderID: s.providerID, ModelID: ModelID(req.Model), TraceID: req.TraceID, Result: &message}
	}()
	return ch, nil
}

func TestRouterRoutesRequestedModelWithFallbacks(t *testing.T) {
	reg, _ := NewRegistryFromProviderConfig(config.ProviderConfig{Type: "openai-compatible", BaseURL: "https://api.openai.com/v1", APIKey: "key", Model: "gpt-5.4"})
	_ = reg.Register(context.Background(), &stubRouterClient{providerID: "backup", models: []ModelInfo{{ProviderID: "backup", ModelID: "gpt-5.4"}}})
	router := NewRouter(reg, nil, RouterConfig{DefaultProvider: ProviderOpenAI, DefaultModel: "gpt-5.4"})
	result, err := router.Route(context.Background(), "gpt-5.4", RouteContext{AllowFallback: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Primary.ProviderID != ProviderOpenAI || result.Primary.ModelID != "gpt-5.4" {
		t.Fatalf("unexpected primary %#v", result.Primary)
	}
	if len(result.Fallbacks) != 1 || result.Fallbacks[0].ProviderID != "backup" {
		t.Fatalf("unexpected fallbacks %#v", result.Fallbacks)
	}
}

func TestRouterFiltersUnhealthyProviders(t *testing.T) {
	reg, _ := NewRegistry(config.ProviderRuntimeConfig{})
	_ = reg.Register(context.Background(), &stubRouterClient{providerID: "openai", models: []ModelInfo{{ProviderID: "openai", ModelID: "gpt-5.4"}}})
	_ = reg.Register(context.Background(), &stubRouterClient{providerID: "backup", models: []ModelInfo{{ProviderID: "backup", ModelID: "gpt-5.4"}}})
	router := NewRouter(reg, stubHealthChecker{errors: map[ProviderID]error{"openai": errors.New("down")}}, RouterConfig{})
	result, err := router.Route(context.Background(), "gpt-5.4", RouteContext{AllowFallback: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Primary.ProviderID != "backup" {
		t.Fatalf("unexpected primary %#v", result.Primary)
	}
}

func TestRouterReturnsUnavailableWithoutCandidates(t *testing.T) {
	reg, _ := NewRegistry(config.ProviderRuntimeConfig{})
	router := NewRouter(reg, nil, RouterConfig{})
	_, err := router.Route(context.Background(), "missing", RouteContext{AllowFallback: true})
	var providerErr *Error
	if !errors.As(err, &providerErr) || providerErr.Code != ErrCodeUnavailable {
		t.Fatalf("unexpected error %#v", err)
	}
}

func TestRoutedClientFallsBackOnRetryableProviderError(t *testing.T) {
	primary := &stubRouterClient{providerID: "openai", models: []ModelInfo{{ProviderID: "openai", ModelID: "gpt-5.4"}}, streams: []stubRouterStreamResult{{err: &Error{Code: ErrCodeRateLimited, Provider: "openai", Message: "rate limited", Retryable: true}}}}
	fallback := &stubRouterClient{providerID: "backup", models: []ModelInfo{{ProviderID: "backup", ModelID: "gpt-5.4"}}, streams: []stubRouterStreamResult{{message: llm.Message{Role: llm.RoleAssistant, Content: "ok"}, deltas: []string{"o", "k"}}}}
	reg, _ := NewRegistry(config.ProviderRuntimeConfig{})
	_ = reg.Register(context.Background(), primary)
	_ = reg.Register(context.Background(), fallback)
	client := NewRoutedClient(NewRouter(reg, nil, RouterConfig{DefaultProvider: "openai"}))
	msg, err := client.StreamMessage(context.Background(), llm.ChatRequest{Model: "gpt-5.4"}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if msg.Content != "ok" {
		t.Fatalf("unexpected message %#v", msg)
	}
	if len(primary.streamReqs) != 1 || len(fallback.streamReqs) != 1 {
		t.Fatalf("unexpected request counts primary=%d fallback=%d", len(primary.streamReqs), len(fallback.streamReqs))
	}
}

func TestRoutedClientStopsOnNonRetryableProviderError(t *testing.T) {
	primary := &stubRouterClient{providerID: "openai", models: []ModelInfo{{ProviderID: "openai", ModelID: "gpt-5.4"}}, streams: []stubRouterStreamResult{{err: &Error{Code: ErrCodeBadRequest, Provider: "openai", Message: "bad request", Retryable: false}}}}
	fallback := &stubRouterClient{providerID: "backup", models: []ModelInfo{{ProviderID: "backup", ModelID: "gpt-5.4"}}, streams: []stubRouterStreamResult{{message: llm.Message{Role: llm.RoleAssistant, Content: "ok"}}}}
	reg, _ := NewRegistry(config.ProviderRuntimeConfig{})
	_ = reg.Register(context.Background(), primary)
	_ = reg.Register(context.Background(), fallback)
	client := NewRoutedClient(NewRouter(reg, nil, RouterConfig{DefaultProvider: "openai"}))
	_, err := client.CreateMessage(context.Background(), llm.ChatRequest{Model: "gpt-5.4"})
	var providerErr *Error
	if !errors.As(err, &providerErr) || providerErr.Code != ErrCodeBadRequest {
		t.Fatalf("unexpected error %#v", err)
	}
	if len(fallback.streamReqs) != 0 {
		t.Fatalf("expected fallback to be skipped, got %d calls", len(fallback.streamReqs))
	}
}
