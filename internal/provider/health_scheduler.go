package provider

import "context"

type ExternalHealthTicker struct {
	health HealthChecker
	ids    func(context.Context) ([]ProviderID, error)
}

func NewExternalHealthTicker(health HealthChecker, ids func(context.Context) ([]ProviderID, error)) *ExternalHealthTicker {
	return &ExternalHealthTicker{health: health, ids: ids}
}

func (t *ExternalHealthTicker) Tick(ctx context.Context) error {
	if t == nil || t.health == nil || t.ids == nil {
		return nil
	}
	ids, err := t.ids(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := t.health.Check(ctx, id); err != nil && ctx.Err() != nil {
			return err
		}
	}
	return nil
}
