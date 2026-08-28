package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func typeString(s string) []tea.KeyMsg {
	msgs := make([]tea.KeyMsg, 0, len(s))
	for _, r := range s {
		msgs = append(msgs, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return msgs
}

func TestConfirmYesAndNoShortcuts(t *testing.T) {
	c := newConfirm("Delete it?")
	done, confirmed := c.update(key("y"))
	if !done || !confirmed {
		t.Errorf("y => done=%v confirmed=%v, want true/true", done, confirmed)
	}

	c = newConfirm("Delete it?")
	done, confirmed = c.update(key("n"))
	if !done || confirmed {
		t.Errorf("n => done=%v confirmed=%v, want true/false", done, confirmed)
	}

	c = newConfirm("Delete it?")
	done, confirmed = c.update(key("esc"))
	if !done || confirmed {
		t.Errorf("esc => done=%v confirmed=%v, want true/false", done, confirmed)
	}
}

func TestConfirmEnterTakesTheHighlightedChoice(t *testing.T) {
	c := newConfirm("Proceed?")
	done, confirmed := c.update(key("enter"))
	if !done || confirmed {
		t.Errorf("enter with default => done=%v confirmed=%v, want true/false", done, confirmed)
	}

	c = newConfirm("Proceed?")
	c.update(key("right")) // toggle to Yes
	done, confirmed = c.update(key("enter"))
	if !done || !confirmed {
		t.Errorf("toggle then enter => done=%v confirmed=%v, want true/true", done, confirmed)
	}
}

func TestConfirmRenderHighlightsCurrentChoice(t *testing.T) {
	c := newConfirm("Sure?")
	if !strings.Contains(c.render(), "> No <") {
		t.Errorf("render = %q, want No highlighted by default", c.render())
	}
	c.update(key("tab"))
	if !strings.Contains(c.render(), "> Yes <") {
		t.Errorf("render = %q, want Yes highlighted after toggle", c.render())
	}
}
