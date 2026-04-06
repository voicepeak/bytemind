package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bytemind/internal/agent"
	"bytemind/internal/assets"
	"bytemind/internal/config"
	"bytemind/internal/provider"
	"bytemind/internal/session"
	"bytemind/internal/tui"
)

var runTUIProgram = tui.Run

func runTUI(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "", "Path to config file")
	model := fs.String("model", "", "Override model name")
	sessionID := fs.String("session", "", "Resume an existing session")
	streamOverride := fs.String("stream", "", "Override streaming: true or false")
	workspaceOverride := fs.String("workspace", "", "Workspace to operate on; defaults to current directory")
	maxIterations := fs.Int("max-iterations", 0, "Override execution budget for this run")

	if err := fs.Parse(args); err != nil {
		return err
	}

	workspace, err := resolveWorkspace(*workspaceOverride)
	if err != nil {
		return err
	}
	if err := ensureAPIConfigForTUI(workspace, *configPath, stdin, stdout); err != nil {
		return err
	}

	cfg, err := config.Load(workspace, *configPath)
	if err != nil {
		return err
	}
	if err := applyRuntimeOverrides(&cfg, *model, *streamOverride, *maxIterations); err != nil {
		return err
	}

	interactive := isInteractiveStdin(stdin)
	check := provider.Availability{Ready: true}
	if interactive {
		check = provider.CheckAvailability(context.Background(), cfg.Provider)
	}

	guide := tui.StartupGuide{}
	if !check.Ready {
		guide = buildStartupGuide(cfg, check, workspace, *configPath)
	}

	var app *agent.Runner
	var store *session.Store
	var sess *session.Session
	if guide.Active && interactive {
		app, store, sess, err = bootstrapForTUI(*configPath, *model, *sessionID, *streamOverride, *workspaceOverride, *maxIterations, stdin, stdout)
	} else {
		app, store, sess, err = bootstrap(*configPath, *model, *sessionID, *streamOverride, *workspaceOverride, *maxIterations, stdin, stdout)
	}
	if err != nil {
		return err
	}
	if app == nil || store == nil || sess == nil {
		return errors.New("internal error: bootstrap returned nil runtime")
	}
	home, err := config.EnsureHomeLayout()
	if err != nil {
		return err
	}
	imageStore, err := assets.NewFileAssetStore(home)
	if err != nil {
		return err
	}

	return runTUIProgram(tui.Options{
		Runner:       app,
		Store:        store,
		Session:      sess,
		ImageStore:   imageStore,
		Config:       cfg,
		Workspace:    sess.Workspace,
		StartupGuide: guide,
	})
}

func applyRuntimeOverrides(cfg *config.Config, modelOverride, streamOverride string, maxIterations int) error {
	if modelOverride != "" {
		cfg.Provider.Model = modelOverride
	}
	if streamOverride != "" {
		parsed, err := strconv.ParseBool(streamOverride)
		if err != nil {
			return fmt.Errorf("invalid -stream value: %w", err)
		}
		cfg.Stream = parsed
	}
	if maxIterations > 0 {
		cfg.MaxIterations = maxIterations
	}
	return nil
}

func isInteractiveStdin(stdin io.Reader) bool {
	file, ok := stdin.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func buildStartupGuide(cfg config.Config, check provider.Availability, workspace, explicitConfigPath string) tui.StartupGuide {
	path := configPathHint(workspace, explicitConfigPath)
	envName := strings.TrimSpace(cfg.Provider.APIKeyEnv)
	if envName == "" {
		envName = "BYTEMIND_API_KEY"
	}
	lines := []string{
		"Paste your API key in the input box below and press Enter.",
		"Bytemind will verify it and save it automatically.",
		"Default OpenAI setup only needs API key.",
		"For other providers, set provider.base_url and provider.model too.",
		"Optional: model=<name>  base_url=<url>  provider=<openai-compatible|anthropic>",
		"You can still use /help and /quit commands.",
		"Env fallback: " + envName,
	}
	if path != "" {
		lines = append(lines, "Config file: "+path)
	}
	lines = append(lines, "Issue: "+startupIssueHint(check))

	return tui.StartupGuide{
		Active:       true,
		Title:        "Let's finish setup",
		Status:       "Bytemind will guide you through provider, base_url, model, and API key.",
		Lines:        lines,
		ConfigPath:   path,
		CurrentField: "type",
	}
}

func startupIssueHint(check provider.Availability) string {
	reason := strings.ToLower(strings.TrimSpace(check.Reason))
	switch {
	case strings.Contains(reason, "missing api key"):
		return "No API key is configured yet."
	case strings.Contains(reason, "unauthorized"):
		return "The API key was rejected by the provider."
	case strings.Contains(reason, "failed to reach"):
		return "Cannot reach provider endpoint. Check proxy or network."
	case strings.Contains(reason, "not found"):
		return "Provider endpoint path looks incorrect."
	default:
		if strings.TrimSpace(check.Reason) == "" {
			return "Provider check failed."
		}
		return compactLine(check.Reason, 120)
	}
}

func configPathHint(workspace, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		if abs, err := filepath.Abs(explicit); err == nil {
			return abs
		}
		return explicit
	}

	candidates := []string{
		filepath.Join(workspace, "config.json"),
		filepath.Join(workspace, ".bytemind", "config.json"),
		filepath.Join(workspace, "bytemind.config.json"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	home, err := config.ResolveHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "config.json")
}

func compactLine(raw string, limit int) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\n", " "))
	if len(raw) <= limit {
		return raw
	}
	if limit <= 3 {
		return raw[:limit]
	}
	return raw[:limit-3] + "..."
}

func ensureAPIConfigForTUI(workspace, configPath string, stdin io.Reader, stdout io.Writer) error {
	cfg, err := config.Load(workspace, configPath)
	if err != nil {
		if strings.TrimSpace(configPath) != "" && errors.Is(err, os.ErrNotExist) {
			return runInteractiveConfigSetup(workspace, configPath, config.Default(workspace), stdin, stdout)
		}
		return err
	}
	if strings.TrimSpace(cfg.Provider.ResolveAPIKey()) != "" {
		return nil
	}
	return runInteractiveConfigSetup(workspace, configPath, cfg, stdin, stdout)
}

func runInteractiveConfigSetup(workspace, configPath string, cfg config.Config, stdin io.Reader, stdout io.Writer) error {
	reader := bufio.NewReader(stdin)
	fmt.Fprintln(stdout, "未检测到可用 API 配置，请先完成初始化。")
	fmt.Fprintln(stdout, "配置格式：OpenAI-compatible（依次输入 url / key / model）。")

	baseURL, err := promptSetupValue(reader, stdout, "url")
	if err != nil {
		return err
	}
	apiKey, err := promptSetupValue(reader, stdout, "key")
	if err != nil {
		return err
	}
	modelName, err := promptSetupValue(reader, stdout, "model")
	if err != nil {
		return err
	}

	baseURL, err = validateBaseURL(baseURL)
	if err != nil {
		return err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return errors.New("配置失败: API Key 不能为空")
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return errors.New("配置失败: Model 不能为空")
	}

	cfg.Provider.Type = "openai-compatible"
	cfg.Provider.AutoDetectType = false
	cfg.Provider.BaseURL = baseURL
	cfg.Provider.APIPath = ""
	cfg.Provider.Model = modelName
	cfg.Provider.APIKey = apiKey
	cfg.Provider.APIKeyEnv = ""
	cfg.Provider.AuthHeader = ""
	cfg.Provider.AuthScheme = ""
	cfg.Provider.ExtraHeaders = nil

	targetPath, err := resolveSetupConfigPath(workspace, configPath)
	if err != nil {
		return err
	}
	if err := writeConfigFile(targetPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "配置已写入: %s\n", targetPath)
	return nil
}

func promptSetupValue(reader *bufio.Reader, stdout io.Writer, label string) (string, error) {
	fmt.Fprintf(stdout, "%s: ", strings.TrimSpace(label))

	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		if errors.Is(err, io.EOF) {
			return "", errors.New("初始化已取消: 未收到输入")
		}
		return "", fmt.Errorf("配置失败: %s 不能为空", label)
	}
	return line, nil
}

func validateBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("配置失败: Base URL 不能为空")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("配置失败: Base URL 必须是合法的 http(s) 地址")
	}
	return strings.TrimRight(value, "/"), nil
}

func resolveSetupConfigPath(workspace, configPath string) (string, error) {
	if strings.TrimSpace(configPath) != "" {
		return filepath.Abs(configPath)
	}
	return filepath.Join(workspace, "config.json"), nil
}

func writeConfigFile(path string, cfg config.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
