package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// ideaFormField identifies the focused row of an ideaForm.
type ideaFormField int

const (
	ideaFieldName ideaFormField = iota
	ideaFieldDescription
	ideaFieldCategory
	ideaFieldNotes
	ideaFieldCount
)

// ideaFormIndent aligns a continuation line under the value column of the
// "> Label:      " rows the form draws.
const ideaFormIndent = "             "

// ideaForm is the capture overlay for an Idea: a name, a description, a
// Category chosen from the seeded list, and optional freeform notes
// (multi-line). It holds no Core and performs no persistence — the dashboard
// reads input() and calls core.
type ideaForm struct {
	title  string
	name   textInput
	desc   textInput
	cats   picker
	catIDs []int64
	notes  textArea
	focus  ideaFormField
}

// newIdeaForm builds a form over the shared Category list. When initial is
// non-nil the form starts in edit mode, pre-filled from that Idea (including
// its Category); otherwise it is a blank capture form defaulting to the first
// Category.
func newIdeaForm(title string, categories []core.Category, initial *core.Idea) ideaForm {
	labels := make([]string, len(categories))
	ids := make([]int64, len(categories))
	for i, c := range categories {
		labels[i] = c.Name
		ids[i] = c.ID
	}

	f := ideaForm{
		title:  title,
		name:   newTextInput(""),
		desc:   newTextInput(""),
		catIDs: ids,
		notes:  newTextArea(""),
		focus:  ideaFieldName,
	}
	start := 0
	if initial != nil {
		f.name = newTextInput(initial.Name)
		f.desc = newTextInput(initial.Description)
		f.notes = newTextArea(initial.Notes)
		for i, id := range ids {
			if id == initial.CategoryID {
				start = i
				break
			}
		}
	}
	f.cats = newPicker(labels, start)
	return f
}

// update advances the form for one key. done is true once the user submits
// (submitted true) or cancels (submitted false); the dashboard then reads
// input() and calls core, or drops the form.
func (f *ideaForm) update(msg tea.KeyMsg) (done, submitted bool) {
	switch msg.String() {
	case "esc":
		return true, false
	case "enter":
		return true, true
	case "alt+enter":
		if f.focus == ideaFieldNotes {
			f.notes.newline()
		}
		return false, false
	case "tab", "down":
		f.focus = (f.focus + 1) % ideaFieldCount
		return false, false
	case "shift+tab", "up":
		f.focus = (f.focus - 1 + ideaFieldCount) % ideaFieldCount
		return false, false
	case "left":
		if f.focus == ideaFieldCategory {
			f.cats.up()
		}
		return false, false
	case "right":
		if f.focus == ideaFieldCategory {
			f.cats.down()
		}
		return false, false
	case "backspace":
		if e := f.focused(); e != nil {
			e.backspace()
		}
		return false, false
	default:
		if e := f.focused(); e != nil && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) {
			for _, r := range msg.Runes {
				e.insertRune(r)
			}
		}
		return false, false
	}
}

// focused is the text field the focus is currently on; nil when the Category
// picker holds focus.
func (f *ideaForm) focused() textEntry {
	switch f.focus {
	case ideaFieldName:
		return &f.name
	case ideaFieldDescription:
		return &f.desc
	case ideaFieldNotes:
		return &f.notes
	default:
		return nil
	}
}

// input is the value the form currently describes, ready for core.CreateIdea.
// A whitespace-only name and a missing Category are left for core to reject.
func (f ideaForm) input() core.IdeaInput {
	var categoryID int64
	if i := f.cats.selectedIndex(); i >= 0 && i < len(f.catIDs) {
		categoryID = f.catIDs[i]
	}
	return core.IdeaInput{
		Name:        f.name.String(),
		Description: f.desc.String(),
		Notes:       f.notes.String(),
		CategoryID:  categoryID,
	}
}

// render draws the whole form with the focused row marked.
func (f ideaForm) render() string {
	var b strings.Builder
	b.WriteString(f.title)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s Name:        %s\n", rowMarker(f.focus == ideaFieldName), f.name.render(f.focus == ideaFieldName))
	fmt.Fprintf(&b, "%s Description: %s\n", rowMarker(f.focus == ideaFieldDescription), f.desc.render(f.focus == ideaFieldDescription))
	fmt.Fprintf(&b, "%s Category:    %s\n", rowMarker(f.focus == ideaFieldCategory), f.cats.value())
	if f.focus == ideaFieldCategory {
		b.WriteString(indentLines(f.cats.render(), "    "))
	}

	noteLines := f.notes.lines(f.focus == ideaFieldNotes)
	fmt.Fprintf(&b, "%s Notes:       %s\n", rowMarker(f.focus == ideaFieldNotes), noteLines[0])
	for _, line := range noteLines[1:] {
		b.WriteString(ideaFormIndent + line + "\n")
	}
	if f.focus == ideaFieldNotes {
		b.WriteString(ideaFormIndent + "(alt+enter for a new line)\n")
	}

	b.WriteString("\ntab: next field   ←/→: choose Category   enter: save   esc: cancel\n")
	return b.String()
}
