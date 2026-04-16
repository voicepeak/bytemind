package runtime

import "testing"

func TestInMemoryQuotaManagerAcquireWithNilContextUsesBackground(t *testing.T) {
	quota := NewInMemoryQuotaManager(1, nil)
	if err := quota.Acquire(nil, "shared"); err != nil {
		t.Fatalf("Acquire with nil context failed: %v", err)
	}
}
