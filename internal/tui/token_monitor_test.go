package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTokenUsageSetUsageClampsValues(t *testing.T) {
	c := newTokenUsageComponent()
	c.displayUsed = 0
	_ = c.SetUsage(9000, 5000)
	if c.used != 9000 || c.total != 5000 {
		t.Fatalf("expected fixed total quota with true used value, got used=%d total=%d", c.used, c.total)
	}
	_ = c.SetUsage(-5, 1000)
	if c.used != 0 {
		t.Fatalf("expected used to clamp to zero, got %d", c.used)
	}
}

func TestTokenUsageHoverShowsPercentage(t *testing.T) {
	c := newTokenUsageComponent()
	c.displayUsed = 1250
	_ = c.SetUsage(1250, 5000)
	c.SetBounds(10, 2, 20, 2)

	_, consumed := c.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		X:      12,
		Y:      2,
	})
	if !consumed {
		t.Fatalf("expected hover motion over badge to be consumed")
	}
	if !strings.Contains(c.usageText(), "%") {
		t.Fatalf("expected percentage text on hover, got %q", c.usageText())
	}
}

func TestTokenUsageClickShowsPopupAndTickHides(t *testing.T) {
	c := newTokenUsageComponent()
	c.displayUsed = 1000
	_ = c.SetUsage(1000, 5000)
	c.SetBounds(0, 0, 20, 2)

	cmd, consumed := c.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      1,
		Y:      0,
	})
	if !consumed {
		t.Fatalf("expected click on badge to be consumed")
	}
	if cmd == nil {
		t.Fatalf("expected click to schedule follow-up tick")
	}
	if !c.popup {
		t.Fatalf("expected popup to be visible after click")
	}

	c.popupUntil = time.Now().Add(-time.Millisecond)
	_, _ = c.Update(tokenMonitorTickMsg(time.Now()))
	if c.popup {
		t.Fatalf("expected popup to auto-hide after timeout")
	}
}

func TestTokenUsagePopupUsesRealBreakdown(t *testing.T) {
	c := newTokenUsageComponent()
	_ = c.SetUsage(300, 5000)
	c.SetBreakdown(120, 140, 40)
	c.popup = true

	view := c.PopupView()
	for _, want := range []string{"Input:   120", "Output:  140", "Context: 40"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected popup to contain %q, got %q", want, view)
		}
	}
}

func TestFormatIntWithCommas(t *testing.T) {
	if got := formatInt(1234567); got != "1,234,567" {
		t.Fatalf("expected comma formatted number, got %q", got)
	}
}
