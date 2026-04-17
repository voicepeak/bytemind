package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHealthCheckerStateMachineTransitions(t *testing.T) {
	checker := NewHealthChecker(HealthConfig{FailThreshold: 2, RecoverProbeSec: 10, RecoverSuccessThreshold: 2, WindowSize: 4}, nil).(*healthChecker)
	now := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	checker.clock = func() time.Time { return now }
	checker.RecordFailure(context.Background(), "openai", &Error{Code: ErrCodeTimeout, Retryable: true})
	snapshot := checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusDegraded || snapshot.FailureCount != 1 {
		t.Fatalf("unexpected degraded snapshot %#v", snapshot)
	}
	now = now.Add(time.Second)
	checker.RecordFailure(context.Background(), "openai", &Error{Code: ErrCodeUnavailable, Retryable: true})
	snapshot = checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusUnavailable || snapshot.FailureCount != 2 || snapshot.NextProbeAt.IsZero() {
		t.Fatalf("unexpected unavailable snapshot %#v", snapshot)
	}
	if err := checker.Check(context.Background(), "openai"); err == nil {
		t.Fatal("expected unavailable before probe window")
	}
	now = snapshot.NextProbeAt
	if err := checker.Check(context.Background(), "openai"); err != nil {
		t.Fatalf("expected half-open probe to pass without checker, got %v", err)
	}
	snapshot = checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusHalfOpen {
		t.Fatalf("expected half_open after probe, got %#v", snapshot)
	}
	checker.RecordSuccess(context.Background(), "openai")
	snapshot = checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusHalfOpen || snapshot.SuccessCount != 1 {
		t.Fatalf("unexpected half-open recovery snapshot %#v", snapshot)
	}
	checker.RecordSuccess(context.Background(), "openai")
	snapshot = checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusHealthy || snapshot.SuccessCount != 2 {
		t.Fatalf("unexpected healthy snapshot %#v", snapshot)
	}
}

func TestHealthCheckerIgnoresNonAvailabilityFailures(t *testing.T) {
	checker := NewHealthChecker(HealthConfig{FailThreshold: 1}, nil)
	checker.RecordFailure(context.Background(), "openai", &Error{Code: ErrCodeUnauthorized})
	snapshot := checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusHealthy || snapshot.FailureCount != 0 {
		t.Fatalf("unexpected snapshot %#v", snapshot)
	}
}

func TestHealthCheckerCheckUsesProbeAndRecordsResults(t *testing.T) {
	calls := 0
	checker := NewHealthChecker(HealthConfig{FailThreshold: 1, RecoverProbeSec: 5, RecoverSuccessThreshold: 1}, func(_ context.Context, id ProviderID) error {
		calls++
		if id != "openai" {
			t.Fatalf("unexpected provider %q", id)
		}
		return nil
	}).(*healthChecker)
	now := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	checker.clock = func() time.Time { return now }
	checker.RecordFailure(context.Background(), "openai", &Error{Code: ErrCodeTimeout, Retryable: true})
	snapshot := checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusUnavailable {
		t.Fatalf("unexpected snapshot %#v", snapshot)
	}
	if err := checker.Check(context.Background(), "openai"); err == nil {
		t.Fatal("expected probe window block")
	}
	now = snapshot.NextProbeAt
	if err := checker.Check(context.Background(), "openai"); err != nil {
		t.Fatalf("expected second probe success, got %v", err)
	}
	snapshot = checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusHealthy {
		t.Fatalf("unexpected recovered snapshot %#v", snapshot)
	}
	if calls != 1 {
		t.Fatalf("expected one active probe call, got %d", calls)
	}
}

func TestHealthSchedulerTick(t *testing.T) {
	var checked []ProviderID
	checker := NewHealthChecker(HealthConfig{FailThreshold: 1, RecoverProbeSec: 1, RecoverSuccessThreshold: 1}, func(_ context.Context, id ProviderID) error {
		checked = append(checked, id)
		return nil
	}).(*healthChecker)
	now := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	checker.clock = func() time.Time { return now }
	checker.RecordFailure(context.Background(), "openai", &Error{Code: ErrCodeUnavailable, Retryable: true})
	checker.RecordFailure(context.Background(), "anthropic", &Error{Code: ErrCodeUnavailable, Retryable: true})
	checker.providers["openai"].nextProbeAt = now
	checker.providers["anthropic"].nextProbeAt = now
	scheduler := NewHealthScheduler(checker, func(context.Context) ([]ProviderID, error) {
		return []ProviderID{"openai", "anthropic"}, nil
	})
	if err := scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(checked) != 2 {
		t.Fatalf("unexpected checked ids %#v", checked)
	}
}

func TestHealthCheckerOnlyProbesUnavailableOrHalfOpen(t *testing.T) {
	calls := 0
	checker := NewHealthChecker(HealthConfig{FailThreshold: 1, RecoverProbeSec: 5, RecoverSuccessThreshold: 1}, func(_ context.Context, _ ProviderID) error {
		calls++
		return nil
	}).(*healthChecker)
	now := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	checker.clock = func() time.Time { return now }
	if err := checker.Check(context.Background(), "openai"); err != nil {
		t.Fatalf("expected healthy check to be local-only, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected no active probe for healthy provider, got %d", calls)
	}
	checker.RecordFailure(context.Background(), "openai", &Error{Code: ErrCodeUnavailable, Retryable: true})
	snapshot := checker.Status(context.Background(), "openai")
	if snapshot.Status != HealthStatusUnavailable {
		t.Fatalf("unexpected snapshot %#v", snapshot)
	}
	now = snapshot.NextProbeAt
	if err := checker.Check(context.Background(), "openai"); err != nil {
		t.Fatalf("expected half-open probe success, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one active probe, got %d", calls)
	}
}

func TestCountsTowardAvailability(t *testing.T) {
	if countsTowardAvailability(nil) || countsTowardAvailability(context.Canceled) {
		t.Fatal("expected nil/canceled to be ignored")
	}
	if countsTowardAvailability(&Error{Code: ErrCodeBadRequest}) {
		t.Fatal("expected bad request to be ignored")
	}
	if !countsTowardAvailability(errors.New("boom")) || !countsTowardAvailability(&Error{Code: ErrCodeRateLimited, Retryable: true}) {
		t.Fatal("expected generic/retryable errors to count")
	}
}
