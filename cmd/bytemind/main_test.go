package main

import (
	"bytes"
	"strings"
	"testing"

	"bytemind/internal/session"
)

func TestCompleteSlashCommand(t *testing.T) {
	completed, suggestions := completeSlashCommand("/he")
	if len(suggestions) != 0 {
		t.Fatalf("expected unique completion, got suggestions %#v", suggestions)
	}
	if completed != "/help" {
		t.Fatalf("expected /help, got %q", completed)
	}
}

func TestCompleteSlashCommandReturnsSuggestionsForAmbiguousPrefix(t *testing.T) {
	completed, suggestions := completeSlashCommand("/sess")
	if completed != "/sess" {
		t.Fatalf("expected input to remain unchanged, got %q", completed)
	}
	if len(suggestions) != 2 || suggestions[0] != "/session" || suggestions[1] != "/sessions" {
		t.Fatalf("unexpected suggestions: %#v", suggestions)
	}
}

func TestResolveSessionIDSupportsUniquePrefix(t *testing.T) {
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	first := session.New(`E:\\repo`)
	first.ID = "20260324-120000-abcd"
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}

	second := session.New(`E:\\repo`)
	second.ID = "20260324-130000-efgh"
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveSessionID(store, "20260324-1300")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != second.ID {
		t.Fatalf("expected %q, got %q", second.ID, resolved)
	}
}

func TestHandleSlashCommandRejectsResumeAcrossWorkspaces(t *testing.T) {
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	current := session.New(`E:\\repo-a`)
	current.ID = "current"
	if err := store.Save(current); err != nil {
		t.Fatal(err)
	}

	other := session.New(`E:\\repo-b`)
	other.ID = "other"
	if err := store.Save(other); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	next, shouldExit, handled, err := handleSlashCommand(&out, store, current, "/resume other")
	if err == nil {
		t.Fatal("expected cross-workspace resume to fail")
	}
	if !strings.Contains(err.Error(), "belongs to workspace") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled || shouldExit {
		t.Fatalf("expected handled command without exit, got handled=%v shouldExit=%v", handled, shouldExit)
	}
	if next != current {
		t.Fatal("expected current session to remain active")
	}
}

func TestSameWorkspaceNormalizesPaths(t *testing.T) {
	if !sameWorkspace(`E:\\Repo`, `E:\\Repo\\.`) {
		t.Fatal("expected normalized paths to match")
	}
}

func TestRunChatAcceptsWorkspaceFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runChat([]string{"-workspace", `E:\\repo`}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected chat to fail later because config is incomplete")
	}
	if strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("unexpected error: %v", err)
	}
}
