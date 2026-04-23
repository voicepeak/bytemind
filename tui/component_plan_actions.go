package tui

import (
	"fmt"
	"io"
	"strings"

	"bytemind/internal/llm"
	planpkg "bytemind/internal/plan"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	planActionStartExecution = "start execution"
	planActionAdjustPlan     = "adjust plan"
)

type planActionItem struct {
	Shortcut        string
	TitleText       string
	DescriptionText string
	Action          string
}

func (i planActionItem) FilterValue() string { return i.Action }
func (i planActionItem) Title() string       { return i.TitleText }
func (i planActionItem) Description() string { return i.DescriptionText }

type planActionDelegate struct{}

func (d planActionDelegate) Height() int  { return 4 }
func (d planActionDelegate) Spacing() int { return 1 }
func (d planActionDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}

func (d planActionDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	action, ok := item.(planActionItem)
	if !ok {
		return
	}

	selected := index == m.Index()
	width := max(24, m.Width())
	bodyWidth := max(16, width-4)

	titleStyle := strongStyle.Copy()
	descStyle := mutedStyle.Copy()
	cardStyle := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorTool)

	badgeKind := "info"
	if selected {
		cardStyle = cardStyle.
			BorderForeground(colorAccent).
			Background(lipgloss.Color("#141E2A"))
		titleStyle = titleStyle.Foreground(colorAccent)
		descStyle = descStyle.Foreground(lipgloss.Color("#D6E4F5"))
		badgeKind = "accent"
	}

	title := lipgloss.JoinHorizontal(
		lipgloss.Left,
		renderPillBadge(action.Shortcut, badgeKind),
		" ",
		titleStyle.Render(action.TitleText),
	)
	desc := descStyle.Width(bodyWidth).Render(action.Description())
	content := lipgloss.JoinVertical(lipgloss.Left, title, desc)
	fmt.Fprint(w, cardStyle.Render(content)) //nolint:errcheck
}

type planActionPicker struct {
	list list.Model
}

func newPlanActionPicker(width int) *planActionPicker {
	items := []list.Item{
		planActionItem{
			Shortcut:        "A",
			TitleText:       "切到 Build 模式，开始执行",
			DescriptionText: "按当前计划基线进入执行，直接从第一步开始落地。",
			Action:          planActionStartExecution,
		},
		planActionItem{
			Shortcut:        "B",
			TitleText:       "继续微调计划",
			DescriptionText: "保持在 Plan 模式，继续补充范围、风险或验收标准。",
			Action:          planActionAdjustPlan,
		},
	}

	l := list.New(items, planActionDelegate{}, max(32, width), 10)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetFilteringEnabled(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.SetStatusBarItemName("action", "actions")
	l.ResetSelected()

	return &planActionPicker{list: l}
}

func (m *model) syncPlanActionPicker() {
	if m == nil {
		return
	}
	if !m.canShowPlanActionPicker() {
		m.closePlanActionPicker()
		return
	}
	if m.planAction == nil {
		m.planAction = newPlanActionPicker(m.chatPanelInnerWidth())
	}
	m.planActionOpen = true
	m.updatePlanActionPickerSize()
}

func (m *model) canShowPlanActionPicker() bool {
	if m == nil || m.mode != modePlan || m.busy {
		return false
	}
	return planpkg.CanStartExecution(m.plan)
}

func (m *model) closePlanActionPicker() {
	if m == nil {
		return
	}
	m.planActionOpen = false
}

func (m *model) updatePlanActionPickerSize() {
	if m == nil || m.planAction == nil {
		return
	}
	width := min(72, max(36, m.chatPanelInnerWidth()))
	m.planAction.list.SetSize(width, 10)
}

func (m model) renderPlanActionPicker() string {
	if m.planAction == nil || !m.planActionOpen {
		return ""
	}
	picker := m.planAction.list
	boxWidth := min(76, max(40, m.chatPanelInnerWidth()))
	picker.SetSize(boxWidth-modalBoxStyle.GetHorizontalFrameSize()-2, 10)
	lines := []string{
		modalTitleStyle.Render("下一步"),
		mutedStyle.Render("A/B 直接选，或用 ↑/↓ 后按 Enter 确认"),
		"",
		picker.View(),
	}
	return modalBoxStyle.Width(boxWidth).Render(strings.Join(lines, "\n"))
}

func (m model) handlePlanActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(msg.String())) {
	case "esc":
		m.closePlanActionPicker()
		m.statusNote = "已收起执行选项。"
		return m, nil
	case "a", "1":
		return m.executePlanActionSelection(planActionStartExecution)
	case "b", "2":
		return m.executePlanActionSelection(planActionAdjustPlan)
	case "enter":
		if action, ok := m.selectedPlanAction(); ok {
			return m.executePlanActionSelection(action)
		}
		return m, nil
	}

	if m.planAction == nil {
		return m, nil
	}
	var cmd tea.Cmd
	m.planAction.list, cmd = m.planAction.list.Update(msg)
	return m, cmd
}

func (m model) selectedPlanAction() (string, bool) {
	if m.planAction == nil {
		return "", false
	}
	item, ok := m.planAction.list.SelectedItem().(planActionItem)
	if !ok {
		return "", false
	}
	return item.Action, true
}

func (m model) executePlanActionSelection(action string) (tea.Model, tea.Cmd) {
	displayText := planActionDisplayText(action)
	m.closePlanActionPicker()

	if action == planActionStartExecution {
		state, err := preparePlanForContinuation(m.plan)
		if err != nil {
			m.statusNote = err.Error()
			return m, nil
		}
		m.plan = state
		m.mode = modeBuild
		if m.sess != nil {
			m.sess.Mode = planpkg.ModeBuild
			m.sess.Plan = copyPlanState(state)
			if m.store != nil {
				if err := m.store.Save(m.sess); err != nil {
					m.statusNote = err.Error()
					return m, nil
				}
			}
		}
	}

	return m.submitPreparedPrompt(RunPromptInput{
		UserMessage: llm.NewUserTextMessage(action),
		DisplayText: displayText,
	}, displayText)
}

func planActionDisplayText(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case planActionStartExecution:
		return "A. 切到 Build 模式，开始执行"
	case planActionAdjustPlan:
		return "B. 继续微调计划"
	default:
		return strings.TrimSpace(action)
	}
}

func stripPlanActionTailFromAnswer(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	endTag := "</proposed_plan>"
	index := strings.LastIndex(strings.ToLower(content), strings.ToLower(endTag))
	if index < 0 {
		return content
	}
	index += len(endTag)
	prefix := strings.TrimRight(content[:index], "\n")
	suffix := strings.TrimSpace(content[index:])
	if !looksLikePlanActionTail(suffix) {
		return content
	}
	return prefix
}

func looksLikePlanActionTail(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "start execution") && strings.Contains(normalized, "adjust plan") {
		return true
	}
	if strings.Contains(normalized, "切到 build") || strings.Contains(normalized, "继续微调计划") {
		return true
	}
	return strings.Contains(normalized, "choose next step") ||
		strings.Contains(normalized, "下一步") ||
		strings.Contains(normalized, "reply with 1") ||
		strings.Contains(normalized, "开始执行") ||
		strings.Contains(normalized, "调整计划")
}
