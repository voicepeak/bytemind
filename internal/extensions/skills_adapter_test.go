package extensions

import (
	"testing"
	"time"

	skillspkg "bytemind/internal/skills"
)

func TestSkillAdapterSyncCachesSkillExtensions(t *testing.T) {
	adapter := newSkillAdapter()
	now := time.Now().UTC()
	items := adapter.Sync(skillspkg.Catalog{Skills: []skillspkg.Skill{{
		Name:         "review",
		Title:        "review",
		Description:  "desc",
		Scope:        skillspkg.ScopeProject,
		SourceDir:    `C:\\repo\\.bytemind\\skills\\review`,
		Instruction:  "body",
		DiscoveredAt: now,
	}}})
	if len(items) != 1 {
		t.Fatalf("expected 1 extension, got %d", len(items))
	}
	if items[0].ID != "skill.review" {
		t.Fatalf("unexpected id: %q", items[0].ID)
	}
	cached := adapter.Sync(skillspkg.Catalog{Skills: []skillspkg.Skill{{
		Name:         "review",
		Title:        "review",
		Description:  "desc",
		Scope:        skillspkg.ScopeProject,
		SourceDir:    `C:\\repo\\.bytemind\\skills\\review`,
		Instruction:  "body",
		DiscoveredAt: now,
	}}})
	if len(cached) != 1 {
		t.Fatalf("expected cached extension, got %d", len(cached))
	}
}

func TestSkillAdapterFromSkillMarksManifestOnlyAsDegraded(t *testing.T) {
	adapter := newSkillAdapter()
	item := adapter.FromSkill(skillspkg.Skill{
		Name:         "review",
		Title:        "review",
		Scope:        skillspkg.ScopeProject,
		SourceDir:    `C:\\repo\\.bytemind\\skills\\review`,
		DiscoveredAt: time.Now().UTC(),
	})
	if item.Status != ExtensionStatusDegraded {
		t.Fatalf("expected degraded status, got %q", item.Status)
	}
	if item.Health.Status != ExtensionStatusDegraded {
		t.Fatalf("expected degraded health, got %q", item.Health.Status)
	}
}
