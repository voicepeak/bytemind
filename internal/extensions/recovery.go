package extensions

import "time"

func nextRecoveryCooldown(current, base time.Duration) time.Duration {
	if base <= 0 {
		base = 30 * time.Second
	}
	if current <= 0 {
		return base
	}
	next := current * 2
	max := base * 8
	if max <= 0 {
		return next
	}
	if next > max {
		return max
	}
	return next
}
