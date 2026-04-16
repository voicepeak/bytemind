package runtime

import (
	"context"
	"fmt"
	"sync"
)

type InMemoryQuotaManager struct {
	mu           sync.Mutex
	defaultLimit int
	limits       map[string]int
	inUse        map[string]int
}

func NewInMemoryQuotaManager(defaultLimit int, perKeyLimits map[string]int) *InMemoryQuotaManager {
	if defaultLimit <= 0 {
		defaultLimit = 1
	}
	limits := make(map[string]int, len(perKeyLimits))
	for key, value := range perKeyLimits {
		limits[key] = value
	}
	return &InMemoryQuotaManager{
		defaultLimit: defaultLimit,
		limits:       limits,
		inUse:        make(map[string]int),
	}
}

func (q *InMemoryQuotaManager) Acquire(ctx context.Context, key string) error {
	if q == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if key == "" {
		key = "global"
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	limit := q.limitForKeyLocked(key)
	if q.inUse[key] >= limit {
		return newRuntimeError(
			ErrorCodeQuotaExceeded,
			fmt.Sprintf("quota exceeded for key %q", key),
			true,
			nil,
		)
	}
	q.inUse[key]++
	return nil
}

func (q *InMemoryQuotaManager) Release(key string) {
	if q == nil {
		return
	}
	if key == "" {
		key = "global"
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	current := q.inUse[key]
	if current <= 1 {
		delete(q.inUse, key)
		return
	}
	q.inUse[key] = current - 1
}

func (q *InMemoryQuotaManager) limitForKeyLocked(key string) int {
	limit := q.defaultLimit
	if configured, ok := q.limits[key]; ok && configured > 0 {
		limit = configured
	}
	if limit <= 0 {
		limit = 1
	}
	return limit
}
