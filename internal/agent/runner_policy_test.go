package agent

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"bytemind/internal/config"
	corepkg "bytemind/internal/core"
	"bytemind/internal/llm"
	"bytemind/internal/session"
	storagepkg "bytemind/internal/storage"
	"bytemind/internal/tools"
)

type recordingAuditStore struct {
	mu     sync.Mutex
	events []storagepkg.AuditEvent
}

func (s *recordingAuditStore) Append(_ context.Context, event storagepkg.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *recordingAuditStore) snapshot() []storagepkg.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]storagepkg.AuditEvent, len(s.events))
	copy(out, s.events)
	return out
}

type policyGatewayFunc func(context.Context, ToolDecisionInput) (ToolDecision, error)

func (f policyGatewayFunc) DecideTool(ctx context.Context, in ToolDecisionInput) (ToolDecision, error) {
	return f(ctx, in)
}

func TestRunPromptPolicyGatewayDeniesToolBeforeExecutor(t *testing.T) {
	workspace := t.TempDir()
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := session.New(workspace)

	executed := false
	blockedTool := &fakeTool{
		name: "blocked_tool",
		run: func(raw json.RawMessage, execCtx *tools.ExecutionContext) (string, error) {
			executed = true
			return `{"ok":true}`, nil
		},
	}
	registry := tools.DefaultRegistry()
	registry.Add(blockedTool)

	client := &recordingClient{replies: []llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{{
				ID:   "call-blocked",
				Type: "function",
				Function: llm.ToolFunctionCall{
					Name:      "blocked_tool",
					Arguments: `{}`,
				},
			}},
		},
		{
			Role:    "assistant",
			Content: "Policy handled.",
		},
	}}

	auditStore := &recordingAuditStore{}
	runner := NewRunner(Options{
		Workspace: workspace,
		Config: config.Config{
			Provider:       config.ProviderConfig{Type: "openai-compatible", Model: "test-model"},
			MaxIterations:  3,
			Stream:         false,
			TokenQuota:     generousTokenQuota,
			ApprovalPolicy: "on-request",
		},
		Client:   client,
		Store:    store,
		Registry: registry,
		PolicyGateway: policyGatewayFunc(func(_ context.Context, in ToolDecisionInput) (ToolDecision, error) {
			if in.ToolName != "blocked_tool" {
				t.Fatalf("unexpected tool name: %q", in.ToolName)
			}
			return ToolDecision{
				Decision:   corepkg.DecisionDeny,
				ReasonCode: policyReasonExplicitDeny,
				Reason:     "blocked by test policy",
				RiskLevel:  corepkg.RiskHigh,
			}, nil
		}),
		AuditStore: auditStore,
		Stdin:      strings.NewReader(""),
		Stdout:     io.Discard,
	})

	answer, err := runner.RunPrompt(context.Background(), sess, "run blocked tool", "build", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if answer != "Policy handled." {
		t.Fatalf("unexpected answer: %q", answer)
	}
	if executed {
		t.Fatal("expected tool execution to be blocked by policy")
	}

	foundPermissionDecision := false
	foundExecuteStart := false
	for _, event := range auditStore.snapshot() {
		if event.Action == "permission_decision" && event.Decision == corepkg.DecisionDeny && event.ReasonCode == policyReasonExplicitDeny {
			foundPermissionDecision = true
		}
		if event.Action == "tool_execute_start" && event.Metadata["tool_name"] == "blocked_tool" {
			foundExecuteStart = true
		}
	}
	if !foundPermissionDecision {
		t.Fatal("expected permission_decision audit event for denied tool")
	}
	if foundExecuteStart {
		t.Fatal("did not expect tool_execute_start audit event for denied tool")
	}
}
