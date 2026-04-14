package tui

import "strings"

func tidyAssistantSpacing(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+4)
	inCodeBlock := false
	prevBlank := true

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if !prevBlank && len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, line)
			inCodeBlock = !inCodeBlock
			prevBlank = false
			continue
		}

		if inCodeBlock {
			out = append(out, line)
			prevBlank = trimmed == ""
			continue
		}

		if trimmed == "" {
			if !prevBlank && len(out) > 0 {
				out = append(out, "")
			}
			prevBlank = true
			continue
		}

		if needsLeadingBlankLine(trimmed) && !prevBlank && len(out) > 0 {
			out = append(out, "")
		}

		out = append(out, line)
		prevBlank = false
	}

	return strings.Join(out, "\n")
}

func needsLeadingBlankLine(line string) bool {
	if strings.HasPrefix(line, "#") {
		return true
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "> ") {
		return true
	}
	if len(line) >= 3 && line[1] == '.' && line[2] == ' ' && line[0] >= '0' && line[0] <= '9' {
		return true
	}
	return false
}

func renderAssistantBody(text string, width int) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	inCodeBlock := false
	prevBlank := true

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			codeLine := strings.TrimRight(wrapPlainText(line, width), "\n")
			if strings.TrimSpace(codeLine) == "" {
				if !prevBlank {
					out = append(out, "")
				}
				prevBlank = true
				continue
			}
			out = append(out, codeStyle.Render(codeLine))
			prevBlank = false
			continue
		}

		plainLine := normalizeAssistantMarkdownLine(line)
		if strings.TrimSpace(plainLine) == "" {
			if !prevBlank {
				out = append(out, "")
			}
			prevBlank = true
			continue
		}

		switch {
		case isMarkdownHeading(strings.TrimSpace(plainLine)):
			out = append(out, renderMarkdownHeading(strings.TrimSpace(plainLine), width))
		case strings.HasPrefix(strings.TrimSpace(plainLine), "> "):
			out = append(out, renderMarkdownQuote(strings.TrimSpace(plainLine), width))
		default:
			out = append(out, renderSemanticAssistantLine(plainLine, width))
		}
		prevBlank = false
	}

	return strings.Join(out, "\n")
}
