package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// formField identifies the focused row of a projectForm.
type formField int

const (
	fieldName formField = iota
	fieldDescription
	fieldCategory
	fieldCount
)

// projectForm is the shared create / edit overlay for a Project: a name, a
// description, and a Category chosen from the seeded list. It holds no Core and
// performs no persistence — the screen that owns it reads input() and calls
// core.
type projectForm struct {
	title  string
	name   textInput
	desc   textInput
	cats   picker
	catIDs []int64
	focus  formField
}

// newProjectForm builds a form over the shared Category list. When initial is
// non-nil the form starts in edit mode, pre-filled from that Project (including
// its Category); otherwise it is a blank create form defaulting to the first
// Category.
func newProjectForm(title string, categories []core.Category, initial *core.Project) projectForm {
	labels := make([]string, len(categories))
	ids := make([]int64, len(categories))
	for i, c := range categories {
		labels[i] = c.Name
		ids[i] = c.ID
	}

	f := projectForm{
		title:  title,
		name:   newTextInput(""),
		desc:   newTextInput(""),
		catIDs: ids,
		focus:  fieldName,
	}
	start := 0
	if initial != nil {
		f.name = newTextInput(initial.Name)
		f.desc = newTextInput(initial.Description)
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
// (submitted true) or cancels (submitted false); the owning screen then reads
// input() and calls core, or drops the form.
func (f *projectForm) update(msg tea.KeyMsg) (done, submitted bool) {
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
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				f.editFocused(func(t *textInput) { t.insertRune(r) })
			}
		}
		return false, false
	}
}

func (f *projectForm) editFocused(edit func(*textInput)) {
	switch f.focus {
	case fieldName:
		edit(&f.name)
	case fieldDescription:
		edit(&f.desc)
	}
}

// input is the value the form currently describes, ready for
// core.CreateProject / core.EditProject. Whitespace-only names and a missing
// Category are left for core to reject.
func (f projectForm) input() core.ProjectInput {
	var categoryID int64
	if i := f.cats.selectedIndex(); i >= 0 && i < len(f.catIDs) {
		categoryID = f.catIDs[i]
	}
	return core.ProjectInput{
		Name:        f.name.String(),
		Description: f.desc.String(),
		CategoryID:  categoryID,
	}
}

// render draws the whole form with the focused row marked.
func (f projectForm) render() string {
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

func rowMarker(focused bool) string {
	if focused {
		return ">"
	}
	return " "
}

func indentLines(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}
