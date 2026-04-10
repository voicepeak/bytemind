package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"bytemind/internal/llm"
)

type executorTestTool struct {
	name   string
	result string
	err    error
}

func (t executorTestTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name: t.name,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
			},
		},
	}
}

func (t executorTestTool) Run(_ context.Context, _ json.RawMessage, _ *ExecutionContext) (string, error) {
	return t.result, t.err
}

func TestExecutorRejectsUnknownArgumentsForStrictSpecs(t *testing.T) {
	registry := &Registry{}
	registry.Add(executorTestTool{name: "strict_tool", result: `{"ok":true}`})
	executor := NewExecutor(registry)

	_, err := executor.Execute(context.Background(), "strict_tool", `{"path":"a.txt","extra":true}`, &ExecutionContext{})
	if err == nil {
		t.Fatal("expected argument validation error")
	}
	execErr, ok := AsToolExecError(err)
	if !ok {
		t.Fatalf("expected ToolExecError, got %T", err)
	}
	if execErr.Code != ToolErrorInvalidArgs {
		t.Fatalf("unexpected code: %s", execErr.Code)
	}
}

func TestExecutorMapsPolicyFailuresToPermissionDenied(t *testing.T) {
	registry := &Registry{}
	registry.Add(executorTestTool{name: "strict_tool", result: `{"ok":true}`})
	executor := NewExecutor(registry)

	_, err := executor.Execute(context.Background(), "strict_tool", `{"path":"a.txt"}`, &ExecutionContext{
		AllowedTools: map[string]struct{}{"read_file": {}},
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
	execErr, ok := AsToolExecError(err)
	if !ok || execErr.Code != ToolErrorPermissionDenied {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestExecutorNormalizesToolFailure(t *testing.T) {
	registry := &Registry{}
	registry.Add(executorTestTool{name: "failing_tool", err: errors.New("command is required")})
	executor := NewExecutor(registry)

	_, err := executor.Execute(context.Background(), "failing_tool", `{"path":"a.txt"}`, &ExecutionContext{})
	if err == nil {
		t.Fatal("expected tool failure")
	}
	execErr, ok := AsToolExecError(err)
	if !ok {
		t.Fatalf("expected ToolExecError, got %T", err)
	}
	if execErr.Code != ToolErrorInvalidArgs {
		t.Fatalf("unexpected code: %s", execErr.Code)
	}
}

func TestExecutorTruncatesLongOutput(t *testing.T) {
	registry := &Registry{}
	registry.Add(executorTestTool{name: "strict_tool", result: strings.Repeat("a", 70000)})
	executor := NewExecutor(registry)

	got, err := executor.Execute(context.Background(), "strict_tool", `{"path":"a.txt"}`, &ExecutionContext{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "\n...[truncated]") {
		t.Fatalf("expected truncated output suffix, got %q", got[len(got)-16:])
	}
}
