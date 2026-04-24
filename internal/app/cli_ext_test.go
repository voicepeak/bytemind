package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	extensionspkg "bytemind/internal/extensions"
)

func TestRunExtHelpRendersUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := RunExt([]string{"help"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	if !strings.Contains(output, "bytemind ext list") || !strings.Contains(output, "bytemind ext status") {
		t.Fatalf("unexpected help output: %q", output)
	}
}

func TestRunExtListStatusAndUnload(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("BYTEMIND_HOME", t.TempDir())
	skillDir := filepath.Join(workspace, ".bytemind", "skills", "review")
	writeExtSkillFixture(t, skillDir, "review")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := RunExt([]string{"list", "--workspace", workspace}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("RunExt list failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "skill.review") {
		t.Fatalf("expected list output to include skill.review, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := RunExt([]string{"status", "skill.review", "--workspace", workspace}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("RunExt status failed: %v", err)
	}
	statusOutput := stdout.String()
	if !strings.Contains(statusOutput, "id: skill.review") || !strings.Contains(statusOutput, "status: active") {
		t.Fatalf("expected status output to include id and active status, got %q", statusOutput)
	}

	stdout.Reset()
	stderr.Reset()
	if err := RunExt([]string{"unload", "skill.review", "--workspace", workspace}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("RunExt unload failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "unloaded extension skill.review") {
		t.Fatalf("expected unload confirmation, got %q", stdout.String())
	}
}

func TestRunExtLoadExternalSource(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("BYTEMIND_HOME", t.TempDir())

	externalSource := filepath.Join(t.TempDir(), "docs")
	writeExtSkillFixture(t, externalSource, "docs")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := RunExt([]string{"load", externalSource, "--workspace", workspace}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("RunExt load failed: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "id: skill.docs") || !strings.Contains(output, "status: active") {
		t.Fatalf("expected load output to include skill.docs active, got %q", output)
	}
}

func TestRunExtHandlesUnknownAndNoArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := RunExt(nil, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("RunExt with no args should render usage, got %v", err)
	}
	if !strings.Contains(stdout.String(), "bytemind ext list") {
		t.Fatalf("expected usage output for no args, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	err := RunExt([]string{"unknown-subcommand"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown ext subcommand") {
		t.Fatalf("expected unknown subcommand error, got %v", err)
	}
}

func TestParseExtActionTargetAndCommonFlags(t *testing.T) {
	workspace := t.TempDir()

	gotWorkspace, gotConfig, gotTarget, err := parseExtActionTarget("ext status", []string{"skill.demo", "--workspace", workspace, "--config", "cfg.json"}, io.Discard)
	if err != nil {
		t.Fatalf("parseExtActionTarget with positional target failed: %v", err)
	}
	if gotTarget != "skill.demo" {
		t.Fatalf("expected positional target skill.demo, got %q", gotTarget)
	}
	if gotConfig != "cfg.json" {
		t.Fatalf("expected config cfg.json, got %q", gotConfig)
	}
	if gotWorkspace == "" {
		t.Fatal("expected resolved workspace path")
	}

	gotWorkspace, gotConfig, gotTarget, err = parseExtActionTarget("ext status", []string{"--workspace", workspace, "--config", "cfg2.json", "skill.demo2"}, io.Discard)
	if err != nil {
		t.Fatalf("parseExtActionTarget with trailing arg failed: %v", err)
	}
	if gotTarget != "skill.demo2" {
		t.Fatalf("expected parsed trailing target skill.demo2, got %q", gotTarget)
	}
	if gotConfig != "cfg2.json" {
		t.Fatalf("expected config cfg2.json, got %q", gotConfig)
	}
	if gotWorkspace == "" {
		t.Fatal("expected resolved workspace path")
	}

	_, _, _, err = parseExtActionTarget("ext status", []string{"--workspace", workspace}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "usage: bytemind ext status <source-or-extension-id>") {
		t.Fatalf("expected missing target usage error, got %v", err)
	}

	gotWorkspace, gotConfig, err = parseExtCommonFlags("ext list", []string{"--workspace", workspace, "--config", "list.json"}, io.Discard)
	if err != nil {
		t.Fatalf("parseExtCommonFlags failed: %v", err)
	}
	if gotWorkspace == "" {
		t.Fatal("expected workspace from parseExtCommonFlags")
	}
	if gotConfig != "list.json" {
		t.Fatalf("expected config list.json, got %q", gotConfig)
	}
}

func TestRenderExtStatusesEmptyAndFallbackMessage(t *testing.T) {
	var out bytes.Buffer
	renderExtStatuses(&out, nil)
	if !strings.Contains(out.String(), "no extensions discovered") {
		t.Fatalf("expected empty-state message, got %q", out.String())
	}

	out.Reset()
	renderExtStatuses(&out, []extensionspkg.ExtensionInfo{
		{
			ID:     "skill.alpha",
			Kind:   extensionspkg.ExtensionSkill,
			Status: extensionspkg.ExtensionStatusActive,
			Health: extensionspkg.HealthSnapshot{},
		},
	})
	output := out.String()
	if !strings.Contains(output, "skill.alpha") || !strings.Contains(output, " -") {
		t.Fatalf("expected fallback '-' health message in output, got %q", output)
	}
}

func writeExtSkillFixture(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.json"), []byte(`{"name":"`+name+`"}`), 0o644); err != nil {
		t.Fatalf("write skill.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# /"+name+"\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}
}
