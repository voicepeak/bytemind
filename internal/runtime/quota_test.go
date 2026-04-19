package runtime

import "testing"

func TestInMemoryQuotaManagerAcquireWithNilContextUsesBackground(t *testing.T) {
	quota := NewInMemoryQuotaManager(1, nil)

	if err := quota.Acquire(nil, "shared"); err != nil {
		t.Fatalf("Acquire with nil context failed: %v", err)
	}

	err := quota.Acquire(nil, "shared")
	if err == nil {
		t.Fatal("expected quota exceeded on second acquire")
	}
	if !hasErrorCode(err, ErrorCodeQuotaExceeded) {
		t.Fatalf("expected error code %q, got %q", ErrorCodeQuotaExceeded, errorCode(err))
	}
}
