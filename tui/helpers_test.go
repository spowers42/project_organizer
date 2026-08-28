package tui

import (
	"context"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
	"github.com/spowers42/project_organizer/internal/store"
)

// key builds the tea.KeyMsg the models see for a named key or a literal rune
// string.
func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
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
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// typeString is the sequence of rune KeyMsgs for typing s character by
// character.
func typeString(s string) []tea.KeyMsg {
	msgs := make([]tea.KeyMsg, 0, len(s))
	for _, r := range s {
		msgs = append(msgs, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return msgs
}

// testCategories is a fixed Category list for the form / filter tests.
func testCategories() []core.Category {
	return []core.Category{
		{ID: 10, Name: "Programming"},
		{ID: 20, Name: "Course"},
		{ID: 30, Name: "Other"},
	}
}

// newTestCore wires a Core over a fresh temp-file database.
func newTestCore(t *testing.T) *core.Core {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "organizer.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return core.New(st, core.SystemClock{}, core.NewRand(1))
}

// firstCategoryID returns the id of the first seeded Category.
func firstCategoryID(t *testing.T, c *core.Core) int64 {
	t.Helper()
	cats, err := c.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats) == 0 {
		t.Fatal("no seeded Categories")
	}
	return cats[0].ID
}

// drainInit runs the commands from a screen's Init to completion and feeds the
// resulting messages back into update, so the screen reaches its loaded state.
func drainInit(update func(tea.Msg) tea.Cmd, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			update(c())
		}
		return
	}
	update(msg)
}
