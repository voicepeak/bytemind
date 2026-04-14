package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderChatCopySection(item chatEntry, width int) string {
	title := strings.TrimSpace(item.Title)
	status := strings.TrimSpace(item.Status)
	if status == "final" {
		status = ""
	}
	switch item.Kind {
	case "assistant":
		if strings.EqualFold(item.Status, "thinking") || strings.EqualFold(item.Status, "thinking_done") {
			title = ""
			status = ""
		}
	case "user":
		if strings.TrimSpace(item.Meta) != "" {
			title = strings.TrimSpace(item.Meta)
		}
	}

	if title == "" {
		switch item.Kind {
		case "assistant":
			title = assistantLabel
		case "user":
			title = "You"
		case "tool":
			title = "Tool"
		default:
			title = "Message"
		}
	}
	if status != "" {
		title += "  " + status
	}

	body := strings.TrimRight(formatChatBody(item, width), "\n")
	if strings.TrimSpace(body) == "" {
		return title
	}
	return title + "\n" + body
}

func renderChatCard(item chatEntry, width int) string {
	border := chatAssistantStyle
	switch item.Kind {
	case "user":
		border = chatUserStyle
	case "tool":
		border = chatAssistantStyle
	case "system":
		border = chatSystemStyle
	default:
		if item.Status == "thinking" || item.Status == "thinking_done" {
			border = chatThinkingStyle
		}
	}
	contentWidth := max(8, width-border.GetHorizontalFrameSize())
	rendered := border.Width(contentWidth).Render(renderChatSection(item, contentWidth))
	if item.Kind != "tool" {
		return rendered
	}

	sep := lipgloss.NewStyle().Foreground(colorTool).Render("|")
	lines := strings.Split(rendered, "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) == "" {
			lines[i] = "  " + lines[i]
			continue
		}
		lines[i] = sep + " " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func renderChatSection(item chatEntry, width int) string {
	title := cardTitleStyle.Foreground(colorAccent)
	bodyStyle := chatBodyStyle
	toolCallTitle := cardTitleStyle.Foreground(lipgloss.Color("#E5B567")).Bold(true)
	toolResultTitle := cardTitleStyle.Foreground(lipgloss.Color("#7AC7FF")).Bold(true)
	status := item.Status
	displayTitle := item.Title
	if status == "final" {
		status = ""
	}
	switch item.Kind {
	case "user":
		title = cardTitleStyle.Foreground(colorUser)
	case "tool":
		if strings.HasPrefix(displayTitle, "Tool Result | ") {
			title = toolResultTitle
		} else {
			title = toolCallTitle
		}
		bodyStyle = toolBodyStyle
		status = ""
	case "system":
		title = cardTitleStyle.Foreground(colorMuted)
	default:
		if item.Status == "thinking" || item.Status == "thinking_done" {
			if item.Status == "thinking_done" {
				title = cardTitleStyle.Foreground(colorThinkingDone)
				bodyStyle = thinkingDoneBodyStyle
			} else {
				title = cardTitleStyle.Foreground(colorThinking)
				bodyStyle = thinkingBodyStyle
			}
			displayTitle = ""
			status = ""
		}
	}
	if item.Kind == "assistant" && (item.Status == "thinking" || item.Status == "thinking_done") {
		return bodyStyle.Width(width).Render(strings.TrimRight(wrapPlainText(item.Body, width), "\n"))
	}
	headContent := title.Render(displayTitle)
	if item.Kind == "user" && strings.TrimSpace(item.Meta) != "" {
		headContent = mutedStyle.Copy().Faint(true).Render(item.Meta)
	}
	if status != "" {
		headContent = lipgloss.JoinHorizontal(lipgloss.Left, headContent, mutedStyle.Render("  "+status))
	}
	head := lipgloss.NewStyle().
		Width(width).
		Render(headContent)
	if item.Kind == "tool" && strings.TrimSpace(item.Body) == "" {
		return head
	}
	body := bodyStyle.Width(width).Render(formatChatBody(item, width))
	return lipgloss.JoinVertical(lipgloss.Left, head, body)
}
