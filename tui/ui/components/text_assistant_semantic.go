package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

func renderSemanticAssistantLine(line string, width int) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}

	listPrefix, core, isList := splitSemanticListPrefix(trimmed)
	if core == "" {
		core = trimmed
	}

	if isDocumentTitleLine(core) {
		wrapped := strings.Split(wrapPlainText(core, width), "\n")
		for i := range wrapped {
			wrapped[i] = assistantHeading1Style.Render(wrapped[i])
		}
		return joinWithListPrefix(listPrefix, wrapped, isList)
	}

	if isStageTitleLine(core) {
		wrapped := strings.Split(wrapPlainText(core, max(8, width-runewidth.StringWidth(listPrefix))), "\n")
		for i := range wrapped {
			wrapped[i] = assistantHeading2Style.Render(wrapped[i])
		}
		wrapped = applyIntentStyleToLines(core, wrapped)
		return joinWithListPrefix(listPrefix, wrapped, isList)
	}

	if isSectionTitleLine(core) {
		wrapped := strings.Split(wrapPlainText(core, max(8, width-runewidth.StringWidth(listPrefix))), "\n")
		for i := range wrapped {
			wrapped[i] = accentStyle.Render(wrapped[i])
		}
		wrapped = applyIntentStyleToLines(core, wrapped)
		return joinWithListPrefix(listPrefix, wrapped, isList)
	}

	label, rest, ok := splitSemanticLabel(core)
	if !ok {
		wrapped := strings.Split(wrapPlainText(core, max(8, width-runewidth.StringWidth(listPrefix))), "\n")
		wrapped = applyIntentStyleToLines(core, wrapped)
		return joinWithListPrefix(listPrefix, wrapped, isList)
	}
	prefix := accentStyle.Render(label)
	if rest == "" {
		lines := []string{prefix}
		lines = applyIntentStyleToLines(core, lines)
		return joinWithListPrefix(listPrefix, lines, isList)
	}

	prefixWidth := runewidth.StringWidth(listPrefix) + runewidth.StringWidth(label+" ")
	contentWidth := max(8, width-prefixWidth)
	wrapped := strings.Split(wrapPlainText(rest, contentWidth), "\n")
	lines := make([]string, 0, len(wrapped))
	separatorGap := " "
	continuationIndent := strings.Repeat(" ", runewidth.StringWidth(label+" "))
	if strings.HasSuffix(label, "：") {
		separatorGap = ""
		continuationIndent = strings.Repeat(" ", runewidth.StringWidth(label))
	}
	for i, part := range wrapped {
		if i == 0 {
			lines = append(lines, prefix+separatorGap+part)
			continue
		}
		lines = append(lines, continuationIndent+part)
	}
	lines = applyIntentStyleToLines(core, lines)
	return joinWithListPrefix(listPrefix, lines, isList)
}

func isStageTitleLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(strings.ToLower(trimmed), "phase ")
}

func isSectionTitleLine(line string) bool {
	line = strings.TrimSpace(line)
	if !(strings.HasSuffix(line, ":") || strings.HasSuffix(line, "：")) {
		return false
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(line, "："), ":"))
	if body == "" || runewidth.StringWidth(body) > 20 {
		return false
	}
	return !strings.Contains(body, " ")
}

func splitSemanticLabel(line string) (label string, rest string, ok bool) {
	sep := "："
	idx := strings.Index(line, sep)
	if idx < 0 {
		sep = ":"
		idx = strings.Index(line, sep)
	}
	if idx <= 0 {
		return "", "", false
	}
	left := strings.TrimSpace(line[:idx])
	right := strings.TrimSpace(line[idx+len(sep):])
	if left == "" || runewidth.StringWidth(left) > 10 {
		return "", "", false
	}
	if strings.Contains(left, " ") {
		return "", "", false
	}
	return left + sep, right, true
}

func applyLineIntentStyle(rawLine, renderedLine string) string {
	line := strings.TrimSpace(strings.ToLower(rawLine))
	switch {
	case hasAnyPrefix(line, "注意", "警告", "warning", "warn", "! "):
		return warnStyle.Render(renderedLine)
	case hasAnyPrefix(line, "错误", "失败", "error", "fatal", "x "):
		return errorStyle.Render(renderedLine)
	case hasAnyPrefix(line, "成功", "完成", "success", "ok "):
		return doneStyle.Render(renderedLine)
	case hasAnyPrefix(line, "提示", "说明", "信息", "info", "note", "hint"):
		return mutedStyle.Copy().Faint(false).Render(renderedLine)
	default:
		return renderedLine
	}
}

func applyIntentStyleToLines(rawLine string, lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i := range lines {
		out[i] = applyLineIntentStyle(rawLine, lines[i])
	}
	return out
}

func splitSemanticListPrefix(line string) (prefix string, core string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return trimmed[:2], strings.TrimSpace(trimmed[2:]), true
	}
	if marker, rest, found := splitOrderedListItem(trimmed); found {
		return marker + " ", strings.TrimSpace(rest), true
	}
	return "", trimmed, false
}

func joinWithListPrefix(prefix string, lines []string, isList bool) string {
	if len(lines) == 0 {
		return ""
	}
	if !isList || strings.TrimSpace(prefix) == "" {
		return strings.Join(lines, "\n")
	}
	prefixGlyph := strings.TrimSpace(prefix)
	prefixIndent := strings.Repeat(" ", runewidth.StringWidth(prefix))
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == 0 {
			out = append(out, prefixGlyph+" "+line)
			continue
		}
		out = append(out, prefixIndent+line)
	}
	return strings.Join(out, "\n")
}

func isDocumentTitleLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if strings.ContainsAny(line, ":：!?！？") {
		return false
	}
	if runewidth.StringWidth(line) > 24 {
		return false
	}
	for _, suffix := range []string{"总览", "总结", "概览", "计划", "清单", "说明", "Overview", "Summary", "Plan", "Checklist", "Notes"} {
		if strings.HasSuffix(line, suffix) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
