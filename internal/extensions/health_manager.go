package extensions

import (
	"strings"
	"sync"
	"time"
)

type HealthManagerOption func(*HealthManager)

func WithHealthManagerClock(clock func() time.Time) HealthManagerOption {
	return func(manager *HealthManager) {
		if manager == nil || clock == nil {
			return
		}
		manager.clock = clock
	}
}

type HealthManager struct {
	mu     sync.RWMutex
	policy IsolationPolicy
	clock  func() time.Time
	states map[string]healthState
}

type healthState struct {
	FailureCount int
	LastFailure  time.Time
	Circuit      CircuitState
	NextRetryAt  time.Time
	Cooldown     time.Duration
}

func NewHealthManager(policy IsolationPolicy, options ...HealthManagerOption) *HealthManager {
	manager := &HealthManager{
		policy: normalizeIsolationPolicy(policy),
		clock: func() time.Time {
			return time.Now().UTC()
		},
		states: map[string]healthState{},
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(manager)
	}
	return manager
}

func (m *HealthManager) AllowProbe(extensionID string) bool {
	id := strings.TrimSpace(extensionID)
	if id == "" {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.states[id]
	if !ok || state.Circuit != CircuitOpen {
		return true
	}

	now := m.clock().UTC()
	if state.NextRetryAt.IsZero() || now.Before(state.NextRetryAt) {
		return false
	}

	state.Circuit = CircuitHalfOpen
	state.NextRetryAt = time.Time{}
	m.states[id] = state
	return true
}

func (m *HealthManager) RecordFailure(extensionID string) IsolationSnapshot {
	id := strings.TrimSpace(extensionID)
	if id == "" {
		return IsolationSnapshot{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[id]
	if state.Cooldown <= 0 {
		state.Cooldown = m.policy.RecoveryCooldown
	}
	now := m.clock().UTC()
	state.FailureCount++
	state.LastFailure = now

	if state.Circuit == CircuitHalfOpen {
		state.Circuit = CircuitOpen
		state.Cooldown = nextRecoveryCooldown(state.Cooldown, m.policy.RecoveryCooldown)
		state.NextRetryAt = now.Add(state.Cooldown)
		m.states[id] = state
		return snapshotFromState(state)
	}

	if state.FailureCount >= m.policy.FailureThreshold {
		state.Circuit = CircuitOpen
		state.NextRetryAt = now.Add(state.Cooldown)
	} else if state.Circuit == "" {
		state.Circuit = CircuitClosed
	}

	m.states[id] = state
	return snapshotFromState(state)
}

func (m *HealthManager) RecordSuccess(extensionID string) IsolationSnapshot {
	id := strings.TrimSpace(extensionID)
	if id == "" {
		return IsolationSnapshot{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[id]
	state.FailureCount = 0
	state.LastFailure = time.Time{}
	state.Circuit = CircuitClosed
	state.NextRetryAt = time.Time{}
	state.Cooldown = m.policy.RecoveryCooldown
	m.states[id] = state
	return snapshotFromState(state)
}

func (m *HealthManager) Snapshot(extensionID string) IsolationSnapshot {
	id := strings.TrimSpace(extensionID)
	if id == "" {
		return IsolationSnapshot{}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.states[id]
	if !ok {
		return IsolationSnapshot{CircuitState: CircuitClosed}
	}
	return snapshotFromState(state)
}

func (m *HealthManager) Forget(extensionID string) {
	id := strings.TrimSpace(extensionID)
	if id == "" {
		return
	}
	m.mu.Lock()
	delete(m.states, id)
	m.mu.Unlock()
}

func (m *HealthManager) SetClockForTesting(clock func() time.Time) {
	if m == nil || clock == nil {
		return
	}
	m.mu.Lock()
	m.clock = clock
	m.mu.Unlock()
}

func (m *HealthManager) UpdatePolicy(policy IsolationPolicy) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	normalized := normalizeIsolationPolicy(policy)
	m.policy = normalized
	now := m.clock().UTC()
	for id, state := range m.states {
		state.Cooldown = normalized.RecoveryCooldown
		if state.Circuit == CircuitOpen {
			switch {
			case !state.LastFailure.IsZero():
				nextRetryAt := state.LastFailure.Add(state.Cooldown)
				if nextRetryAt.Before(now) {
					nextRetryAt = now
				}
				state.NextRetryAt = nextRetryAt
			case state.NextRetryAt.IsZero():
				state.NextRetryAt = now.Add(state.Cooldown)
			}
		}
		m.states[id] = state
	}
}

func snapshotFromState(state healthState) IsolationSnapshot {
	snapshot := IsolationSnapshot{
		FailureCount: state.FailureCount,
		CircuitState: state.Circuit,
	}
	if snapshot.CircuitState == "" {
		snapshot.CircuitState = CircuitClosed
	}
	if !state.LastFailure.IsZero() {
		snapshot.LastFailureUTC = state.LastFailure.UTC().Format(time.RFC3339)
	}
	if !state.NextRetryAt.IsZero() {
		snapshot.NextRetryAtUTC = state.NextRetryAt.UTC().Format(time.RFC3339)
	}
	return snapshot
}
