package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// ideaForm is the capture overlay for an Idea: a name, a description, and a
// Category chosen from the seeded list — the same three fields as
// projectForm, held before the Idea is actionable (CONTEXT.md). It holds no
// Core and performs no persistence — the dashboard reads input() and calls
// core.
type ideaForm struct {
	title  string
	name   textInput
	desc   textInput
	cats   picker
	catIDs []int64
	focus  formField
}

// newIdeaForm builds a blank capture form over the shared Category list,
// defaulting to the first Category.
func newIdeaForm(title string, categories []core.Category) ideaForm {
	labels := make([]string, len(categories))
	ids := make([]int64, len(categories))
	for i, c := range categories {
		labels[i] = c.Name
		ids[i] = c.ID
	}
	return ideaForm{
		title:  title,
		name:   newTextInput(""),
		desc:   newTextInput(""),
		cats:   newPicker(labels, 0),
		catIDs: ids,
		focus:  fieldName,
	}
}

// update advances the form for one key. done is true once the user submits
// (submitted true) or cancels (submitted false); the dashboard then reads
// input() and calls core, or drops the form. Mirrors projectForm.update.
func (f *ideaForm) update(msg tea.KeyMsg) (done, submitted bool) {
	switch msg.String() {
	case "esc":
		return true, false
	case "enter":
		return true, true
	case "tab", "down":
		f.focus = (f.focus + 1) % fieldCount
		return false, false
	case "shift+tab", "up":
		f.focus = (f.focus - 1 + fieldCount) % fieldCount
		return false, false
	case "left":
		if f.focus == fieldCategory {
			f.cats.up()
		}
		return false, false
	case "right":
		if f.focus == fieldCategory {
			f.cats.down()
		}
		return false, false
	case "backspace":
		f.editFocused(func(t *textInput) { t.backspace() })
		return false, false
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			for _, r := range msg.Runes {
				f.editFocused(func(t *textInput) { t.insertRune(r) })
			}
		}
		return false, false
	}
}

// editFocused applies edit to whichever text field currently holds focus. It
// is a no-op when the Category picker is focused.
func (f *ideaForm) editFocused(edit func(*textInput)) {
	switch f.focus {
	case fieldName:
		edit(&f.name)
	case fieldDescription:
		edit(&f.desc)
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
		CategoryID:  categoryID,
	}
}

// render draws the whole form with the focused row marked.
func (f ideaForm) render() string {
	var b strings.Builder
	b.WriteString(f.title)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s Name:        %s\n", rowMarker(f.focus == fieldName), f.name.render(f.focus == fieldName))
	fmt.Fprintf(&b, "%s Description: %s\n", rowMarker(f.focus == fieldDescription), f.desc.render(f.focus == fieldDescription))
	fmt.Fprintf(&b, "%s Category:    %s\n", rowMarker(f.focus == fieldCategory), f.cats.value())
	if f.focus == fieldCategory {
		b.WriteString(indentLines(f.cats.render(), "    "))
	}
	b.WriteString("\ntab: next field   ←/→: choose Category   enter: save   esc: cancel\n")
	return b.String()
}
