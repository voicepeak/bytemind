package extensions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toolspkg "bytemind/internal/tools"
)

func TestExtensionsE2EDiscoverLoadBridgeExecuteDegradeRecover(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	projectSkills := filepath.Join(workspace, ".bytemind", "skills")
	reviewDir := filepath.Join(projectSkills, "review")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review skill dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "skill.json"), []byte(`{"name":"review"}`), 0o644); err != nil {
		t.Fatalf("write skill.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "SKILL.md"), []byte("# /review\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}

	manager := NewManagerWithDirs(workspace, filepath.Join(root, "builtin"), filepath.Join(root, "user"), projectSkills)
	items, err := manager.List(context.Background())
	if err != nil {
		t.Fatalf("discover list failed: %v", err)
	}
	review, ok := findExtensionInfoByID(items, "skill.review")
	if !ok {
		t.Fatalf("expected discovered skill.review extension, got %#v", items)
	}
	if review.Status != ExtensionStatusActive {
		t.Fatalf("expected discovered review extension active, got %#v", review)
	}

	loaded, err := manager.Load(context.Background(), reviewDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.ID != "skill.review" || loaded.Status != ExtensionStatusActive {
		t.Fatalf("expected loaded active skill.review, got %#v", loaded)
	}

	registry := &toolspkg.Registry{}
	binding, err := RegisterBridgedTool(registry, ExtensionTool{
		Source:      ExtensionSkill,
		ExtensionID: loaded.ID,
		Tool:        bridgeTestTool{name: "open_doc"},
	})
	if err != nil {
		t.Fatalf("register bridged tool failed: %v", err)
	}
	executor := toolspkg.NewExecutor(registry)
	output, err := executor.Execute(context.Background(), binding.StableKey, `{}`, &toolspkg.ExecutionContext{})
	if err != nil {
		t.Fatalf("execute bridged tool failed: %v", err)
	}
	if strings.TrimSpace(output) != "ok" {
		t.Fatalf("expected bridged tool output ok, got %q", output)
	}

	if err := os.Remove(filepath.Join(reviewDir, "SKILL.md")); err != nil {
		t.Fatalf("remove SKILL.md failed: %v", err)
	}
	items, _ = manager.List(context.Background())
	review, ok = findExtensionInfoByID(items, "skill.review")
	if !ok {
		t.Fatalf("expected degraded review extension to remain visible, got %#v", items)
	}
	if review.Status != ExtensionStatusDegraded {
		t.Fatalf("expected degraded status after removing SKILL.md, got %#v", review)
	}

	if err := os.WriteFile(filepath.Join(reviewDir, "SKILL.md"), []byte("# /review restored\n"), 0o644); err != nil {
		t.Fatalf("restore SKILL.md failed: %v", err)
	}
	items, err = manager.List(context.Background())
	if err != nil {
		t.Fatalf("list after SKILL.md restore failed: %v", err)
	}
	review, ok = findExtensionInfoByID(items, "skill.review")
	if !ok {
		t.Fatalf("expected recovered review extension in list, got %#v", items)
	}
	if review.Status != ExtensionStatusActive {
		t.Fatalf("expected active status after recovery, got %#v", review)
	}
}

func findExtensionInfoByID(items []ExtensionInfo, id string) (ExtensionInfo, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return ExtensionInfo{}, false
}
