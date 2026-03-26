package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBg      = lipgloss.Color("#0F172A")
	colorPanel   = lipgloss.Color("#111827")
	colorBorder  = lipgloss.Color("#243244")
	colorAccent  = lipgloss.Color("#5EEAD4")
	colorUser    = lipgloss.Color("#FDBA74")
	colorTool    = lipgloss.Color("#FDE68A")
	colorMuted   = lipgloss.Color("#94A3B8")
	colorDanger  = lipgloss.Color("#FCA5A5")
	colorSuccess = lipgloss.Color("#86EFAC")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E2E8F0")).
			Background(lipgloss.Color("#0B1220")).
			Padding(0, 1)

	tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1FAE5")).
			Background(lipgloss.Color("#12313A")).
			Padding(0, 1).
			MarginLeft(1)

	statusTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FEF3C7")).
			Background(lipgloss.Color("#3B2F17")).
			Padding(0, 1).
			MarginLeft(1)

	subtleBorderStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				BorderStyle(lipgloss.NormalBorder()).
				BorderTop(false).
				BorderLeft(false).
				BorderRight(false).
				BorderBottom(true).
				BorderForeground(colorBorder).
				Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
			Background(colorPanel).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	landingLogoStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	landingTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E2E8F0")).
				Bold(true)

	landingInputStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorAccent).
				Padding(0, 1).
				Background(colorPanel)

	sidebarStyle = lipgloss.NewStyle().
			Background(colorBg).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	sectionStyle = lipgloss.NewStyle().
			MarginBottom(1)

	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#E2E8F0")).
				Underline(true)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1).
			Background(colorPanel)

	modeTabStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(lipgloss.Color("#132030")).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1).
			MarginRight(1).
			MarginBottom(1)

	cardTitleStyle = lipgloss.NewStyle().Bold(true)

	chatAssistantStyle = lipgloss.NewStyle().
				BorderLeft(true).
				BorderForeground(colorAccent).
				Padding(0, 1).
				Background(lipgloss.Color("#10222A"))

	chatUserStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderForeground(colorUser).
			Padding(0, 1).
			Background(lipgloss.Color("#2B1E15"))

	chatToolStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderForeground(colorTool).
			Padding(0, 1).
			Background(lipgloss.Color("#2C2815"))

	chatSystemStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderForeground(colorMuted).
			Padding(0, 1).
			Background(lipgloss.Color("#17202B"))

	modalBoxStyle = lipgloss.NewStyle().
			Background(colorPanel).
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	codeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#0B1220")).
			Padding(1, 1)

	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	accentStyle = lipgloss.NewStyle().Foreground(colorAccent)
	doneStyle   = lipgloss.NewStyle().Foreground(colorSuccess)
	warnStyle   = lipgloss.NewStyle().Foreground(colorTool)
	errorStyle  = lipgloss.NewStyle().Foreground(colorDanger)
)

func spacer(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Width(width).Render("")
}
