package extensions

import (
	"errors"
	"testing"

	toolspkg "bytemind/internal/tools"
)

func TestContractLifecycleRejectsIllegalTransition(t *testing.T) {
	info := ExtensionInfo{
		ID:     "skill.review",
		Kind:   ExtensionSkill,
		Status: ExtensionStatusStopped,
	}
	_, _, err := degradeTransition(info, "unexpected failure", ErrCodeLoadFailed)
	if err == nil {
		t.Fatal("expected illegal stopped->degraded transition error")
	}
	var extErr *ExtensionError
	if !errors.As(err, &extErr) {
		t.Fatalf("expected ExtensionError, got %T", err)
	}
	if extErr.Code != ErrCodeInvalidTransition {
		t.Fatalf("expected invalid_transition error code, got %q", extErr.Code)
	}
}

func TestContractBridgeRejectsBuiltinConflict(t *testing.T) {
	registry := toolspkg.DefaultRegistry()
	_, err := RegisterBridgedTool(registry, ExtensionTool{
		Source:      ExtensionSkill,
		ExtensionID: "skill.review",
		Tool:        bridgeTestTool{name: "read_file"},
	})
	if err == nil {
		t.Fatal("expected builtin conflict rejection")
	}
	var regErr *toolspkg.RegistryError
	if !errors.As(err, &regErr) {
		t.Fatalf("expected RegistryError, got %T", err)
	}
	if regErr.Code != toolspkg.RegistryErrorDuplicateName {
		t.Fatalf("expected duplicate_name code, got %q", regErr.Code)
	}
	if regErr.ConflictWith.Source != toolspkg.RegistrationSourceBuiltin {
		t.Fatalf("expected builtin conflict source, got %#v", regErr.ConflictWith)
	}
}
