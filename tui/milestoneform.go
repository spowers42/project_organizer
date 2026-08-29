package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// milestoneForm is the single-field overlay for naming a new Milestone. It
// holds no Core and performs no persistence — the screen that owns it reads
// name() and calls core, keeping the overlay open if core rejects the name.
type milestoneForm struct {
	heading string
	field   textInput
}

// newMilestoneForm builds a blank name form under heading.
func newMilestoneForm(heading string) milestoneForm {
	return milestoneForm{heading: heading, field: newTextInput("")}
}

// name is the text currently entered. A whitespace-only value is left for core
// to reject.
func (f milestoneForm) name() string { return f.field.String() }

// update advances the form for one key. done is true once the user submits
// (submitted true) or cancels (submitted false).
func (f *milestoneForm) update(msg tea.KeyMsg) (done, submitted bool) {
	switch msg.String() {
	case "esc":
		return true, false
	case "enter":
		return true, true
	case "backspace":
		f.field.backspace()
		return false, false
	default:
		// Printable input: KeyRunes for ordinary characters, KeySpace for the
		// space bar (Bubble Tea reports it as its own type but still fills
		// Runes).
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			for _, r := range msg.Runes {
				f.field.insertRune(r)
			}
		}
		return false, false
	}
}

// render draws the heading, the field, and the key-hint line.
func (f milestoneForm) render() string {
	return f.heading + "\n\n" +
		"Name: " + f.field.render(true) + "\n" +
		"\nenter: add   esc: cancel\n"
}
