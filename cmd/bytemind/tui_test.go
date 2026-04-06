package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	itui "bytemind/internal/tui"
)

func TestRunTUIBuildsOptionsAndInvokesProgram(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	writeTestConfig(t, workspace, map[string]any{
		"provider": map[string]any{
			"type":     "openai-compatible",
			"base_url": "https://api.openai.com/v1",
			"model":    "gpt-5.4-mini",
			"api_key":  "test-key",
		},
		"stream": false,
	})

	sentinel := errors.New("stop program")
	original := runTUIProgram
	t.Cleanup(func() {
		runTUIProgram = original
	})

	called := false
	runTUIProgram = func(opts itui.Options) error {
		called = true
		if opts.Runner == nil || opts.Store == nil || opts.Session == nil {
			t.Fatalf("expected runner/store/session to be initialized")
		}
		if opts.ImageStore == nil {
			t.Fatalf("expected image store to be initialized")
		}
		if opts.Workspace != workspace {
			t.Fatalf("expected workspace %q, got %q", workspace, opts.Workspace)
		}
		if opts.Config.Provider.Model != "gpt-5.4" {
			t.Fatalf("expected model override to apply, got %q", opts.Config.Provider.Model)
		}
		if !opts.Config.Stream {
			t.Fatalf("expected stream override to apply")
		}
		if opts.Config.MaxIterations != 9 {
			t.Fatalf("expected max-iterations override to apply, got %d", opts.Config.MaxIterations)
		}
		return sentinel
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runTUI([]string{
		"-workspace", workspace,
		"-model", "gpt-5.4",
		"-stream", "true",
		"-max-iterations", "9",
	}, strings.NewReader(""), &stdout, &stderr)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if !called {
		t.Fatal("expected tui program runner to be called")
	}
}

func TestRunTUIRejectsInvalidStreamValue(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	writeTestConfig(t, workspace, map[string]any{
		"provider": map[string]any{
			"type":     "openai-compatible",
			"base_url": "https://api.openai.com/v1",
			"model":    "gpt-5.4-mini",
			"api_key":  "test-key",
		},
		"stream": false,
	})

	original := runTUIProgram
	t.Cleanup(func() {
		runTUIProgram = original
	})
	runTUIProgram = func(opts itui.Options) error {
		t.Fatalf("did not expect tui program runner on invalid stream")
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runTUI([]string{"-workspace", workspace, "-stream", "invalid"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected invalid stream error")
	}
	if !strings.Contains(err.Error(), "invalid -stream value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTUIFailsWhenHomeLayoutCannotBeCreated(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	writeTestConfig(t, workspace, map[string]any{
		"provider": map[string]any{
			"type":     "openai-compatible",
			"base_url": "https://api.openai.com/v1",
			"model":    "gpt-5.4-mini",
			"api_key":  "test-key",
		},
		"stream": false,
	})

	blockParent := t.TempDir()
	blockFile := filepath.Join(blockParent, "not-a-dir")
	if err := os.WriteFile(blockFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BYTEMIND_HOME", filepath.Join(blockFile, "child"))

	original := runTUIProgram
	t.Cleanup(func() {
		runTUIProgram = original
	})
	runTUIProgram = func(opts itui.Options) error {
		t.Fatalf("did not expect tui program runner when home layout fails")
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runTUI([]string{"-workspace", workspace}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected home layout creation error")
	}
}

func TestRunTUIFailsWhenImageCachePathIsAFile(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	writeTestConfig(t, workspace, map[string]any{
		"provider": map[string]any{
			"type":     "openai-compatible",
			"base_url": "https://api.openai.com/v1",
			"model":    "gpt-5.4-mini",
			"api_key":  "test-key",
		},
		"stream": false,
	})

	home := t.TempDir()
	t.Setenv("BYTEMIND_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "image-cache"), []byte("block-dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	original := runTUIProgram
	t.Cleanup(func() {
		runTUIProgram = original
	})
	runTUIProgram = func(opts itui.Options) error {
		t.Fatalf("did not expect tui program runner when image store init fails")
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runTUI([]string{"-workspace", workspace}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected image store initialization error")
	}
}

func TestEnsureAPIConfigForTUIPromptsAndWritesWorkspaceConfig(t *testing.T) {
	workspace := t.TempDir()
	home := filepath.Join(t.TempDir(), ".bytemind-home")
	t.Setenv("BYTEMIND_HOME", home)

	var stdout bytes.Buffer
	input := strings.NewReader("https://api.openai.com/v1\ntest-key\ngpt-5.4\n")
	if err := ensureAPIConfigForTUI(workspace, "", input, &stdout); err != nil {
		t.Fatalf("expected interactive setup to succeed, got %v", err)
	}

	configPath := filepath.Join(workspace, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected workspace config to be written, got %v", err)
	}
	var cfg struct {
		Provider map[string]any `json:"provider"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("expected valid json config file, got %v", err)
	}
	if got := strings.TrimSpace(anyToString(cfg.Provider["api_key"])); got != "test-key" {
		t.Fatalf("expected saved api_key test-key, got %q", got)
	}
	if got := strings.TrimSpace(anyToString(cfg.Provider["base_url"])); got != "https://api.openai.com/v1" {
		t.Fatalf("expected default base_url, got %q", got)
	}
	if got := strings.TrimSpace(anyToString(cfg.Provider["model"])); got != "gpt-5.4" {
		t.Fatalf("expected model to be saved, got %q", got)
	}
	if !strings.Contains(stdout.String(), "配置已写入") {
		t.Fatalf("expected setup output to mention config path, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "未检测到可用 API 配置") {
		t.Fatalf("expected setup output to mention missing config, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "OpenAI-compatible") {
		t.Fatalf("expected setup output to mention OpenAI-compatible format, got %q", stdout.String())
	}
	for _, prompt := range []string{"url: ", "key: ", "model: "} {
		if !strings.Contains(stdout.String(), prompt) {
			t.Fatalf("expected setup prompt %q, got %q", prompt, stdout.String())
		}
	}
}

func TestEnsureAPIConfigForTUIFailsWhenInputIsMissing(t *testing.T) {
	workspace := t.TempDir()
	home := filepath.Join(t.TempDir(), ".bytemind-home")
	t.Setenv("BYTEMIND_HOME", home)

	var stdout bytes.Buffer
	err := ensureAPIConfigForTUI(workspace, "", strings.NewReader(""), &stdout)
	if err == nil {
		t.Fatal("expected missing input to abort setup")
	}
	if !strings.Contains(err.Error(), "初始化已取消") {
		t.Fatalf("unexpected setup error: %v", err)
	}
}

func TestRunTUIBootstrapsAfterInteractiveSetup(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	home := filepath.Join(t.TempDir(), ".bytemind-home")
	t.Setenv("BYTEMIND_HOME", home)

	sentinel := errors.New("stop program")
	original := runTUIProgram
	t.Cleanup(func() {
		runTUIProgram = original
	})

	called := false
	runTUIProgram = func(opts itui.Options) error {
		called = true
		if opts.Config.Provider.APIKey != "wizard-key" {
			t.Fatalf("expected wizard api key in runtime config, got %q", opts.Config.Provider.APIKey)
		}
		if opts.Config.Provider.Model != "gpt-5.4-mini" {
			t.Fatalf("expected wizard model in runtime config, got %q", opts.Config.Provider.Model)
		}
		return sentinel
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runTUI(
		[]string{"-workspace", workspace},
		strings.NewReader("https://api.openai.com/v1\nwizard-key\ngpt-5.4-mini\n"),
		&stdout,
		&stderr,
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if !called {
		t.Fatal("expected tui program runner to be called after setup")
	}

	data, err := os.ReadFile(filepath.Join(workspace, "config.json"))
	if err != nil {
		t.Fatalf("expected interactive setup to create workspace config, got %v", err)
	}
	if !strings.Contains(string(data), "wizard-key") || !strings.Contains(string(data), "gpt-5.4-mini") {
		t.Fatalf("expected saved config to include wizard values, got %q", string(data))
	}
}

func anyToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
