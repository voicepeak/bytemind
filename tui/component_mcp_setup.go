package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"bytemind/internal/config"
	"bytemind/internal/mcpctl"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	mcpSetupPresetGithub     = "github"
	mcpSetupGithubTokenEnv   = "GITHUB_PERSONAL_ACCESS_TOKEN"
	mcpSetupCommandInputHint = "Input `cancel` to abort setup."
	mcpSetupUsage            = "usage: /mcp setup <id>"
)

type mcpSetupStep string

const (
	mcpSetupStepCommand     mcpSetupStep = "command"
	mcpSetupStepArgs        mcpSetupStep = "args"
	mcpSetupStepEnv         mcpSetupStep = "env"
	mcpSetupStepGithubToken mcpSetupStep = "github_token"
	mcpSetupStepConfirm     mcpSetupStep = "confirm"
)

type mcpSetupSession struct {
	preset string
	step   mcpSetupStep
	req    mcpctl.AddRequest
}

func (m *model) startMCPSetup(input string, fields []string) error {
	if m == nil {
		return fmt.Errorf("model is unavailable")
	}
	if m.mcpService == nil {
		return fmt.Errorf("mcp service is unavailable")
	}
	if m.mcpSetup != nil {
		return fmt.Errorf("mcp setup is already in progress, type `cancel` to abort")
	}
	if len(fields) != 3 {
		return fmt.Errorf(mcpSetupUsage)
	}

	serverID := strings.ToLower(strings.TrimSpace(fields[2]))
	if serverID == "" {
		return fmt.Errorf(mcpSetupUsage)
	}

	session := mcpSetupSession{
		step: mcpSetupStepCommand,
		req: mcpctl.AddRequest{
			ID:              serverID,
			Name:            serverID,
			AutoStart:       mcpSetupBoolPtr(true),
			StartupTimeoutS: config.DefaultMCPStartupTimeoutSeconds,
			CallTimeoutS:    config.DefaultMCPCallTimeoutSeconds,
			MaxConcurrency:  config.DefaultMCPMaxConcurrency,
		},
	}
	intro := []string{
		fmt.Sprintf("MCP setup wizard started for `%s`.", serverID),
		mcpSetupCommandInputHint,
	}

	if serverID == mcpSetupPresetGithub {
		session.preset = mcpSetupPresetGithub
		session.step = mcpSetupStepGithubToken
		session.req.Name = "GitHub MCP"
		session.req.Command = "npx"
		session.req.Args = []string{"-y", "@modelcontextprotocol/server-github"}
		intro = append(intro,
			"Preset auto-detected: github",
			"Step 1/2: Enter GitHub personal access token for `GITHUB_PERSONAL_ACCESS_TOKEN`.",
		)
	} else {
		intro = append(intro,
			"Step 1/4: Enter stdio command (example: npx).",
		)
	}

	m.mcpSetup = &session
	m.commandOpen = false
	m.helpOpen = false
	m.skillsOpen = false
	m.sessionsOpen = false
	m.closeMentionPalette()
	m.input.Focus()
	m.appendCommandExchange(input, strings.Join(intro, "\n"))
	m.statusNote = "MCP setup started."
	return nil
}

func (m *model) handleMCPSetupSubmission(rawValue string) (handled bool, cmd tea.Cmd, err error) {
	if m == nil || m.mcpSetup == nil {
		return false, nil, nil
	}
	handled = true
	value := strings.TrimSpace(rawValue)
	if strings.HasPrefix(value, "/") && !strings.EqualFold(value, "/mcp cancel") {
		return true, nil, fmt.Errorf("mcp setup in progress: continue setup or input `/mcp cancel`")
	}

	if isMCPSetupCancelInput(value) {
		m.mcpSetup = nil
		m.input.Focus()
		m.appendMCPSetupAssistant("MCP setup canceled.")
		m.statusNote = "MCP setup canceled."
		return true, nil, nil
	}

	setup := m.mcpSetup
	switch setup.step {
	case mcpSetupStepCommand:
		command := strings.TrimSpace(value)
		if command == "" {
			return true, nil, fmt.Errorf("command is required")
		}
		setup.req.Command = command
		setup.step = mcpSetupStepArgs
		m.input.Focus()
		m.appendMCPSetupAssistant(strings.Join([]string{
			fmt.Sprintf("Command set to `%s`.", setup.req.Command),
			"Step 2/4: Enter args as comma-separated values (example: -y,@modelcontextprotocol/server-github).",
			"Input `skip` for no args.",
		}, "\n"))
		m.statusNote = "MCP setup step 2/4"
		return true, nil, nil

	case mcpSetupStepArgs:
		if isMCPSetupSkipInput(value) {
			setup.req.Args = nil
		} else {
			setup.req.Args = splitMCPSetupCSV(value)
		}
		setup.step = mcpSetupStepEnv
		m.input.Focus()
		m.appendMCPSetupAssistant(strings.Join([]string{
			"Step 3/4: Enter env entries as `KEY=VALUE` separated by commas.",
			"Input `skip` for no env.",
		}, "\n"))
		m.statusNote = "MCP setup step 3/4"
		return true, nil, nil

	case mcpSetupStepEnv:
		if isMCPSetupSkipInput(value) {
			setup.req.Env = nil
		} else {
			env, parseErr := parseMCPSetupEnv(value)
			if parseErr != nil {
				return true, nil, parseErr
			}
			setup.req.Env = env
		}
		setup.step = mcpSetupStepConfirm
		m.input.Focus()
		m.appendMCPSetupAssistant(buildMCPSetupConfirmText(setup, "Step 4/4"))
		m.statusNote = "MCP setup confirmation"
		return true, nil, nil

	case mcpSetupStepGithubToken:
		token := strings.TrimSpace(value)
		if token == "" {
			return true, nil, fmt.Errorf("github token is required")
		}
		if setup.req.Env == nil {
			setup.req.Env = map[string]string{}
		}
		setup.req.Env[mcpSetupGithubTokenEnv] = token
		setup.step = mcpSetupStepConfirm
		m.input.Focus()
		m.appendMCPSetupAssistant(buildMCPSetupConfirmText(setup, "Step 2/2"))
		m.statusNote = "MCP setup confirmation"
		return true, nil, nil

	case mcpSetupStepConfirm:
		if isMCPSetupConfirmInput(value) {
			req := setup.req
			m.mcpSetup = nil
			return true, m.startMCPSetupApply(req), nil
		}
		if isMCPSetupRejectInput(value) {
			m.mcpSetup = nil
			m.input.Focus()
			m.appendMCPSetupAssistant("MCP setup canceled.")
			m.statusNote = "MCP setup canceled."
			return true, nil, nil
		}
		return true, nil, fmt.Errorf("please input `yes` to apply, or `no`/`cancel` to abort")

	default:
		m.mcpSetup = nil
		return true, nil, fmt.Errorf("mcp setup state is invalid, please restart with `/mcp setup <id>`")
	}
}

func (m *model) startMCPSetupApply(req mcpctl.AddRequest) tea.Cmd {
	if m == nil {
		return nil
	}
	if m.mcpService == nil {
		m.statusNote = "mcp service is unavailable"
		return nil
	}
	m.markMCPSetupApplying()
	commandInput := "/mcp setup " + strings.TrimSpace(req.ID)
	if commandInput == "/mcp setup " {
		commandInput = "/mcp setup"
	}

	if m.async == nil {
		response, status, err := executeMCPSetupApply(m.mcpService, req)
		if err != nil {
			m.statusNote = err.Error()
			return nil
		}
		m.appendCommandExchange(commandInput, response)
		if strings.TrimSpace(status) != "" {
			m.statusNote = status
		}
		return nil
	}

	if m.mcpCommandPending {
		m.statusNote = "an MCP command is already running"
		return nil
	}
	m.mcpCommandPending = true
	m.statusNote = "Applying MCP setup..."
	asyncCh := m.async
	service := m.mcpService
	go func() {
		response, status, err := executeMCPSetupApply(service, req)
		asyncCh <- mcpCommandResultMsg{
			Input:    commandInput,
			Response: response,
			Status:   status,
			Err:      err,
		}
	}()
	return nil
}

func executeMCPSetupApply(service MCPService, req mcpctl.AddRequest) (response string, status string, err error) {
	if service == nil {
		return "", "", fmt.Errorf("mcp service is unavailable")
	}
	if strings.TrimSpace(req.ID) == "" {
		return "", "", fmt.Errorf("server id is required")
	}
	if strings.TrimSpace(req.Command) == "" {
		return "", "", fmt.Errorf("server command is required")
	}

	ctx := context.Background()
	addStatus, err := service.Add(ctx, req)
	if err != nil {
		return "", "", err
	}
	serverID := strings.TrimSpace(addStatus.ID)
	if serverID == "" {
		serverID = strings.TrimSpace(req.ID)
	}
	testStatus, err := service.Test(ctx, serverID)
	if err != nil {
		return "", "", err
	}
	enableStatus, err := service.Enable(ctx, serverID, true)
	if err != nil {
		return "", "", err
	}
	if err := service.Reload(ctx); err != nil {
		return "", "", err
	}

	lines := []string{
		fmt.Sprintf("MCP setup completed for `%s`.", serverID),
		fmt.Sprintf("- add: status=%s tools=%d message=%s", addStatus.Status, addStatus.Tools, firstNonEmptyStatus(addStatus.Message, "-")),
		fmt.Sprintf("- test: status=%s tools=%d message=%s", testStatus.Status, testStatus.Tools, firstNonEmptyStatus(testStatus.Message, "-")),
		fmt.Sprintf("- enable: status=%s tools=%d message=%s", enableStatus.Status, enableStatus.Tools, firstNonEmptyStatus(enableStatus.Message, "-")),
		"- reload: done",
	}
	return strings.Join(lines, "\n"), "MCP setup completed.", nil
}

func buildMCPSetupConfirmText(session *mcpSetupSession, stepLabel string) string {
	if session == nil {
		return ""
	}
	req := session.req
	envKeys := make([]string, 0, len(req.Env))
	for key := range req.Env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)

	lines := []string{
		stepLabel + ": Review configuration.",
		fmt.Sprintf("- id: %s", strings.TrimSpace(req.ID)),
		fmt.Sprintf("- name: %s", firstNonEmptyStatus(strings.TrimSpace(req.Name), "-")),
		fmt.Sprintf("- cmd: %s", firstNonEmptyStatus(strings.TrimSpace(req.Command), "-")),
		fmt.Sprintf("- args: %s", firstNonEmptyStatus(strings.Join(req.Args, ","), "-")),
		fmt.Sprintf("- env_keys: %s", firstNonEmptyStatus(strings.Join(envKeys, ","), "-")),
		fmt.Sprintf("- cwd: %s", firstNonEmptyStatus(strings.TrimSpace(req.CWD), "-")),
		fmt.Sprintf("- startup_timeout_s: %d", req.StartupTimeoutS),
		fmt.Sprintf("- call_timeout_s: %d", req.CallTimeoutS),
		fmt.Sprintf("- max_concurrency: %d", req.MaxConcurrency),
	}
	if session.preset == mcpSetupPresetGithub {
		lines = append(lines, "- preset: github")
	}
	lines = append(lines, "Type `yes` to apply, or `no`/`cancel` to abort.")
	return strings.Join(lines, "\n")
}

func (m *model) appendMCPSetupAssistant(body string) {
	if m == nil {
		return
	}
	m.screen = screenChat
	m.appendChat(chatEntry{
		Kind:   "assistant",
		Title:  assistantLabel,
		Body:   strings.TrimSpace(body),
		Status: "final",
	})
}

func (m *model) markMCPSetupApplying() {
	if m == nil {
		return
	}
	m.screen = screenChat
	m.appendChat(chatEntry{
		Kind:   "assistant",
		Title:  assistantLabel,
		Body:   "Applying MCP setup... (Add -> Test -> Enable -> Reload)",
		Status: "pending",
	})
}

func (m *model) finishMCPSetupApplying(success bool) {
	if m == nil {
		return
	}
	for index := len(m.chatItems) - 1; index >= 0; index-- {
		item := &m.chatItems[index]
		if item.Kind != "assistant" || item.Status != "pending" {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(item.Body))
		if !strings.HasPrefix(normalized, "applying mcp setup") {
			continue
		}
		if success {
			item.Status = "final"
			item.Body = "MCP setup apply finished."
		} else {
			item.Status = "warn"
			item.Body = "MCP setup apply failed. Check error/status message."
		}
		return
	}
}

func splitMCPSetupCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func parseMCPSetupEnv(raw string) (map[string]string, error) {
	items := splitMCPSetupCSV(raw)
	if len(items) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(items))
	for _, item := range items {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid env entry %q (expected KEY=VALUE)", item)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("invalid env entry %q (empty key)", item)
		}
		out[key] = parts[1]
	}
	return out, nil
}

func isMCPSetupSkipInput(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || value == "skip" || value == "-" || value == "none"
}

func isMCPSetupCancelInput(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "cancel" || value == "/mcp cancel" || value == "exit" || value == "quit"
}

func isMCPSetupConfirmInput(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "yes" || value == "y" || value == "confirm"
}

func isMCPSetupRejectInput(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "no" || value == "n" || value == "reject" || value == "cancel"
}

func mcpSetupBoolPtr(value bool) *bool {
	return &value
}
