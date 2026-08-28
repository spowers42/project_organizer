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

// errTaskDueDateFormat is returned by taskForm.input when the due-date field
// holds text that is not a taskDueDateLayout date. The owning screen maps it to
// a message through errorMessage, like the core.Err* sentinels.
var errTaskDueDateFormat = errors.New("due date must be YYYY-MM-DD")

// taskFormField identifies the focused row of a taskForm.
type taskFormField int

const (
	taskFieldTitle taskFormField = iota
	taskFieldDueDate
	taskFieldCount
)

// taskForm is the shared add / edit overlay for a loose Task: a title and an
// optional due date (YYYY-MM-DD). It holds no Core and performs no persistence —
// the screen that owns it reads input() and calls core.
type taskForm struct {
	heading    string
	titleInput textInput
	due        textInput
	focus      taskFormField
}

// newTaskForm builds a form under heading. When initial is non-nil it starts in
// edit mode, pre-filled from that Task; otherwise it is a blank add form.
func newTaskForm(heading string, initial *core.Task) taskForm {
	f := taskForm{
		heading:    heading,
		titleInput: newTextInput(""),
		due:        newTextInput(""),
		focus:      taskFieldTitle,
	}
	if initial != nil {
		f.titleInput = newTextInput(initial.Title)
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
	case "tab", "down":
		f.focus = (f.focus + 1) % taskFieldCount
		return false, false
	case "shift+tab", "up":
		f.focus = (f.focus - 1 + taskFieldCount) % taskFieldCount
		return false, false
	case "backspace":
		f.editFocused(func(t *textInput) { t.backspace() })
		return false, false
	default:
		// Printable input: KeyRunes for ordinary characters, KeySpace for the
		// space bar (Bubble Tea reports it as its own type but still fills
		// Runes). Titles allow spaces.
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			for _, r := range msg.Runes {
				f.editFocused(func(t *textInput) { t.insertRune(r) })
			}
		}
		return false, false
	}
}

// editFocused applies edit to whichever text field currently holds focus.
func (f *taskForm) editFocused(edit func(*textInput)) {
	switch f.focus {
	case taskFieldTitle:
		edit(&f.titleInput)
	case taskFieldDueDate:
		edit(&f.due)
	}
}

// input is the value the form currently describes, ready for core.AddTask /
// core.EditTask. A blank due-date field yields a nil DueDate; a malformed one
// is errTaskDueDateFormat so the owning screen can surface it and keep the form
// open. A whitespace-only title is left for core to reject.
func (f taskForm) input() (core.TaskInput, error) {
	in := core.TaskInput{Title: f.titleInput.String()}
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
	b.WriteString("            (YYYY-MM-DD, or blank for none)\n")
	b.WriteString("\ntab: next field   enter: save   esc: cancel\n")
	return b.String()
}
