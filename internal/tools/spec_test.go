package tools

import (
	"testing"

	"bytemind/internal/llm"
	planpkg "bytemind/internal/plan"
)

func TestDefaultToolSpecDerivesReadOnlyAndPlanModes(t *testing.T) {
	spec := DefaultToolSpec(llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name: "read_file",
		},
	})

	if !spec.ReadOnly {
		t.Fatal("expected read_file to be read-only")
	}
	if !spec.StrictArgs {
		t.Fatal("expected strict args to default true")
	}
	if !modeAllowed(spec, planpkg.ModePlan) {
		t.Fatal("expected read_file to be available in plan mode")
	}
}

func TestNormalizeToolSpecFillsDefaults(t *testing.T) {
	spec := NormalizeToolSpec(ToolSpec{
		Name:         "custom_tool",
		AllowedModes: []planpkg.AgentMode{planpkg.ModeBuild, planpkg.ModeBuild, planpkg.ModePlan},
	})

	if spec.DefaultTimeoutS != 30 {
		t.Fatalf("unexpected default timeout: %d", spec.DefaultTimeoutS)
	}
	if spec.MaxTimeoutS != 300 {
		t.Fatalf("unexpected max timeout: %d", spec.MaxTimeoutS)
	}
	if len(spec.AllowedModes) != 2 {
		t.Fatalf("expected deduplicated modes, got %#v", spec.AllowedModes)
	}
}

func TestValidateToolSpecRejectsConflictingFlags(t *testing.T) {
	err := ValidateToolSpec(ToolSpec{
		Name:            "bad_tool",
		ReadOnly:        true,
		Destructive:     true,
		SafetyClass:     SafetyClassSafe,
		AllowedModes:    []planpkg.AgentMode{planpkg.ModeBuild},
		DefaultTimeoutS: 1,
		MaxTimeoutS:     1,
		MaxResultChars:  1,
	})
	if err == nil {
		t.Fatal("expected spec validation error")
	}
}
