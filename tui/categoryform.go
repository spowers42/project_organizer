package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// categoryForm is the single-field overlay for creating or renaming a
// Category: just its name. It holds no Core and performs no persistence —
// the dashboard reads value() and calls core.
type categoryForm struct {
	title string
	name  textInput
}

// newCategoryForm builds a blank form for a new Category, or — when initial
// is non-nil — a form pre-filled with an existing Category's name, for
// renaming.
func newCategoryForm(title string, initial *core.Category) categoryForm {
	seed := ""
	if initial != nil {
		seed = initial.Name
	}
	return categoryForm{title: title, name: newTextInput(seed)}
}

// update advances the form for one key. done is true once the user submits
// (submitted true) or cancels (submitted false); the dashboard then reads
// value() and calls core, or drops the form.
func (f *categoryForm) update(msg tea.KeyMsg) (done, submitted bool) {
	switch msg.String() {
	case "esc":
		return true, false
	case "enter":
		return true, true
	case "backspace":
		f.name.backspace()
		return false, false
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			for _, r := range msg.Runes {
				f.name.insertRune(r)
			}
		}
		return false, false
	}
}

// value is the name the form currently holds, ready for
// core.CreateCategory or core.RenameCategory. A whitespace-only value is left
// for core to reject.
func (f categoryForm) value() string { return f.name.String() }

// render draws the field and the key-hint line.
func (f categoryForm) render() string {
	var b strings.Builder
	b.WriteString(f.title)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "> Name: %s\n", f.name.render(true))
	b.WriteString("\nenter: save   esc: cancel\n")
	return b.String()
}
