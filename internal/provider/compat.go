package provider

import (
	"context"
	"strings"

	"bytemind/internal/llm"
)

type clientAdapter struct {
	providerID   ProviderID
	defaultModel ModelID
	client       llm.Client
}

func WrapClient(providerID ProviderID, defaultModel ModelID, client llm.Client) Client {
	if client == nil {
		return nil
	}
	id := ProviderID(strings.TrimSpace(string(providerID)))
	if id == "" {
		id = ProviderID("unknown")
	}
	return &clientAdapter{
		providerID:   id,
		defaultModel: ModelID(strings.TrimSpace(string(defaultModel))),
		client:       client,
	}
}

func (a *clientAdapter) ProviderID() ProviderID {
	return a.providerID
}

func (a *clientAdapter) ListModels(context.Context) ([]ModelInfo, error) {
	if a.defaultModel == "" {
		return nil, nil
	}
	return []ModelInfo{{
		ProviderID: a.providerID,
		ModelID:    a.defaultModel,
	}}, nil
}

func (a *clientAdapter) Stream(ctx context.Context, req Request) (<-chan Event, error) {
	stream := make(chan Event, 4)
	go func() {
		defer close(stream)
		modelID := ModelID(strings.TrimSpace(req.Model))
		if modelID == "" {
			modelID = a.defaultModel
		}
		stream <- Event{
			Type:       EventStart,
			TraceID:    req.TraceID,
			ProviderID: a.providerID,
			ModelID:    modelID,
		}
		message, err := a.client.StreamMessage(ctx, req.ChatRequest, func(delta string) {
			if strings.TrimSpace(delta) == "" {
				return
			}
			stream <- Event{
				Type:       EventDelta,
				TraceID:    req.TraceID,
				ProviderID: a.providerID,
				ModelID:    modelID,
				Delta:      delta,
			}
		})
		if err != nil {
			stream <- Event{
				Type:       EventError,
				TraceID:    req.TraceID,
				ProviderID: a.providerID,
				ModelID:    modelID,
				Error: &Error{
					Code:      ErrCodeUnavailable,
					Provider:  a.providerID,
					Message:   err.Error(),
					Retryable: false,
					Err:       err,
				},
			}
			return
		}
		if message.Usage != nil {
			stream <- Event{
				Type:       EventUsage,
				TraceID:    req.TraceID,
				ProviderID: a.providerID,
				ModelID:    modelID,
				Usage: &Usage{
					InputTokens:  int64(message.Usage.InputTokens),
					OutputTokens: int64(message.Usage.OutputTokens),
					TotalTokens:  int64(message.Usage.TotalTokens),
				},
			}
		}
		message.Normalize()
		for _, toolCall := range message.ToolCalls {
			call := toolCall
			stream <- Event{
				Type:       EventToolCall,
				TraceID:    req.TraceID,
				ProviderID: a.providerID,
				ModelID:    modelID,
				ToolCall:   &call,
			}
		}
		stream <- Event{
			Type:       EventResult,
			TraceID:    req.TraceID,
			ProviderID: a.providerID,
			ModelID:    modelID,
			Result:     &message,
		}
	}()
	return stream, nil
}
