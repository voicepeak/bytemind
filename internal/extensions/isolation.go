package extensions

import "time"

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type IsolationPolicy struct {
	FailureThreshold int
	RecoveryCooldown time.Duration
}

type IsolationSnapshot struct {
	FailureCount   int
	LastFailureUTC string
	CircuitState   CircuitState
	NextRetryAtUTC string
}

func normalizeIsolationPolicy(policy IsolationPolicy) IsolationPolicy {
	if policy.FailureThreshold < 1 {
		policy.FailureThreshold = 3
	}
	if policy.RecoveryCooldown <= 0 {
		policy.RecoveryCooldown = 30 * time.Second
	}
	return policy
}
