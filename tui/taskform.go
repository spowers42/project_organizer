package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// taskDueDateLayout is the date format the Task form accepts for the optional
// due date. A blank field means no due date.
const taskDueDateLayout = "2006-01-02"

// taskFormIndent aligns a continuation line under the value column of the
// "> Label:    " rows the form draws.
const taskFormIndent = "           "

// errTaskDueDateFormat is returned by taskForm.input when the due-date field
// holds text that is not a taskDueDateLayout date. The owning screen maps it to
// a message through errorMessage, like the core.Err* sentinels.
var errTaskDueDateFormat = errors.New("due date must be YYYY-MM-DD")

// taskFormField identifies the focused row of a taskForm.
type taskFormField int

const (
	taskFieldTitle taskFormField = iota
	taskFieldDueDate
	taskFieldNotes
	taskFieldCount
)

// taskForm is the shared add / edit overlay for a loose Task: a title, an
// optional due date (YYYY-MM-DD), and optional freeform notes (multi-line). It
// holds no Core and performs no persistence — the screen that owns it reads
// input() and calls core.
type taskForm struct {
	heading    string
	titleInput textInput
	due        textInput
	notes      textArea
	focus      taskFormField
}

// newTaskForm builds a form under heading. When initial is non-nil it starts in
// edit mode, pre-filled from that Task; otherwise it is a blank add form.
func newTaskForm(heading string, initial *core.Task) taskForm {
	f := taskForm{
		heading:    heading,
		titleInput: newTextInput(""),
		due:        newTextInput(""),
		notes:      newTextArea(""),
		focus:      taskFieldTitle,
	}
	if initial != nil {
		f.titleInput = newTextInput(initial.Title)
		f.notes = newTextArea(initial.Notes)
		if initial.DueDate != nil {
			f.due = newTextInput(initial.DueDate.Format(taskDueDateLayout))
		}
	}
	return f
}

// update advances the form for one key. done is true once the user submits
// (submitted true) or cancels (submitted false).
func (f *taskForm) update(msg tea.KeyMsg) (done, submitted bool) {
	switch msg.String() {
	case "esc":
		return true, false
	case "enter":
		return true, true
	case "alt+enter":
		if f.focus == taskFieldNotes {
			f.notes.newline()
		}
		return false, false
	case "tab", "down":
		f.focus = (f.focus + 1) % taskFieldCount
		return false, false
	case "shift+tab", "up":
		f.focus = (f.focus - 1 + taskFieldCount) % taskFieldCount
		return false, false
	case "backspace":
		if e := f.focused(); e != nil {
			e.backspace()
		}
		return false, false
	default:
		// Printable input: KeyRunes for ordinary characters, KeySpace for the
		// space bar (Bubble Tea reports it as its own type but still fills
		// Runes). Every field allows spaces.
		if e := f.focused(); e != nil && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) {
			for _, r := range msg.Runes {
				e.insertRune(r)
			}
		}
		return false, false
	}
}

// focused is the text field the focus is currently on.
func (f *taskForm) focused() textEntry {
	switch f.focus {
	case taskFieldTitle:
		return &f.titleInput
	case taskFieldDueDate:
		return &f.due
	case taskFieldNotes:
		return &f.notes
	default:
		return nil
	}
}

// input is the value the form currently describes, ready for core.AddTask /
// core.EditTask. A blank due-date field yields a nil DueDate; a malformed one
// is errTaskDueDateFormat so the owning screen can surface it and keep the form
// open. A whitespace-only title is left for core to reject. Notes pass through
// verbatim, empty when untouched.
func (f taskForm) input() (core.TaskInput, error) {
	in := core.TaskInput{Title: f.titleInput.String(), Notes: f.notes.String()}
	raw := strings.TrimSpace(f.due.String())
	if raw == "" {
		return in, nil
	}
	due, err := time.Parse(taskDueDateLayout, raw)
	if err != nil {
		return core.TaskInput{}, errTaskDueDateFormat
	}
	in.DueDate = &due
	return in, nil
}

// render draws the whole form with the focused row marked.
func (f taskForm) render() string {
	var b strings.Builder
	b.WriteString(f.heading)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s Title:    %s\n", rowMarker(f.focus == taskFieldTitle), f.titleInput.render(f.focus == taskFieldTitle))
	fmt.Fprintf(&b, "%s Due date: %s\n", rowMarker(f.focus == taskFieldDueDate), f.due.render(f.focus == taskFieldDueDate))
	b.WriteString(taskFormIndent + "(YYYY-MM-DD, or blank for none)\n")

	noteLines := f.notes.lines(f.focus == taskFieldNotes)
	fmt.Fprintf(&b, "%s Notes:    %s\n", rowMarker(f.focus == taskFieldNotes), noteLines[0])
	for _, line := range noteLines[1:] {
		b.WriteString(taskFormIndent + line + "\n")
	}
	b.WriteString(taskFormIndent + "(alt+enter for a new line)\n")

	b.WriteString("\ntab: next field   enter: save   esc: cancel\n")
	return b.String()
}
