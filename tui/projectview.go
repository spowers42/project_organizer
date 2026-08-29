package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// projectViewModel is the second screen: one Project's fields and its ordered
// body — loose Tasks and Milestones interleaved in a single user-ordered
// sequence — with actions to edit the Project, move it through its lifecycle,
// add / edit / complete Tasks, add Milestones, and reorder any body entry. It
// reads core and mutates through core; it holds no domain logic.
type projectViewModel struct {
	core      *core.Core
	projectID int64
	project   core.Project
	cats      []core.Category
	body      []core.BodyEntry
	bodySel   int
	bodyErr   error
	loaded    bool
	loadErr   error
	status    string
	overlay   overlayHost
}

// newProjectView builds the screen bound to one Project id; Init loads it.
func newProjectView(c *core.Core, id int64) *projectViewModel {
	return &projectViewModel{core: c, projectID: id}
}

// projectLoadedMsg carries the result of loading the viewed Project.
type projectLoadedMsg struct {
	project core.Project
	err     error
}

// projectArchivedMsg is the result of archiving the viewed Project.
type projectArchivedMsg struct {
	err error
}

// bodyLoadedMsg carries the result of loading the Project's ordered body.
type bodyLoadedMsg struct {
	body []core.BodyEntry
	err  error
}

// taskSavedMsg is the result of an add / edit / complete-toggle on a Task. A
// nil err means the change persisted.
type taskSavedMsg struct {
	err error
}

// milestoneSavedMsg is the result of adding a Milestone. A nil err means it
// persisted.
type milestoneSavedMsg struct {
	err error
}

// bodyMovedMsg is the result of reordering a body entry (a loose Task or a
// Milestone) within the Project body. On success it carries the body in its new
// order plus a BodyRef for the moved entry, so the selection can follow it.
type bodyMovedMsg struct {
	body  []core.BodyEntry
	moved core.BodyRef
	err   error
}

// Init loads the Project, its body, and the shared Category list.
func (v *projectViewModel) Init() tea.Cmd {
	return tea.Batch(v.loadProject, v.loadBody, loadCategoriesCmd(v.core))
}

// loadProject reads the viewed Project by id.
func (v *projectViewModel) loadProject() tea.Msg {
	p, err := v.core.GetProject(context.Background(), v.projectID)
	return projectLoadedMsg{project: p, err: err}
}

// loadBody reads the Project's loose Tasks and Milestones in interleaved body
// order.
func (v *projectViewModel) loadBody() tea.Msg {
	entries, err := v.core.ProjectBody(context.Background(), v.projectID)
	return bodyLoadedMsg{body: entries, err: err}
}

// addTask appends a loose Task to the Project from the form's fields.
func (v *projectViewModel) addTask(in core.TaskInput) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.AddTask(context.Background(), v.projectID, in)
		return taskSavedMsg{err: err}
	}
}

// editTask rewrites a Task's title and due date from the form's fields.
func (v *projectViewModel) editTask(id int64, in core.TaskInput) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.EditTask(context.Background(), id, in)
		return taskSavedMsg{err: err}
	}
}

// toggleTask flips the selected Task's completion flag.
func (v *projectViewModel) toggleTask(task core.Task) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.SetTaskDone(context.Background(), task.ID, !task.Done)
		return taskSavedMsg{err: err}
	}
}

// addMilestone appends a Milestone with the given name to the Project body.
func (v *projectViewModel) addMilestone(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.AddMilestone(context.Background(), v.projectID, name)
		return milestoneSavedMsg{err: err}
	}
}

// moveEntry reorders the selected body entry one slot up or down, dispatching to
// the Task or Milestone mover by its kind.
func (v *projectViewModel) moveEntry(e core.BodyEntry, dir core.MoveDir) tea.Cmd {
	ref := e.Ref()
	return func() tea.Msg {
		var (
			body []core.BodyEntry
			err  error
		)
		if ref.Kind == core.MilestoneEntry {
			body, err = v.core.MoveMilestone(context.Background(), ref.ID, dir)
		} else {
			body, err = v.core.MoveTask(context.Background(), ref.ID, dir)
		}
		return bodyMovedMsg{body: body, moved: ref, err: err}
	}
}

// editProject persists the form's fields onto the viewed Project.
func (v *projectViewModel) editProject(in core.ProjectInput) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.EditProject(context.Background(), v.projectID, in)
		return projectSavedMsg{err: err}
	}
}

// setLifecycle moves the viewed Project to state.
func (v *projectViewModel) setLifecycle(state core.Lifecycle) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.SetProjectLifecycle(context.Background(), v.projectID, state)
		return projectSavedMsg{err: err}
	}
}

// archiveProject soft-deletes the viewed Project into the Archive.
func (v *projectViewModel) archiveProject() tea.Cmd {
	return func() tea.Msg {
		return projectArchivedMsg{err: v.core.ArchiveProject(context.Background(), v.projectID)}
	}
}

// Update advances the Project view for one message.
func (v *projectViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectLoadedMsg:
		v.project, v.loadErr, v.loaded = msg.project, msg.err, true
		return nil
	case bodyLoadedMsg:
		v.body, v.bodyErr = msg.body, msg.err
		if v.bodySel >= len(v.body) {
			v.bodySel = 0
		}
		return nil
	case taskSavedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil // keep the overlay open to fix and retry
		}
		v.overlay.close()
		v.status = "Saved."
		return v.loadBody
	case milestoneSavedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil // keep the overlay open to fix and retry
		}
		v.overlay.close()
		v.status = "Milestone added."
		return v.loadBody
	case bodyMovedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil
		}
		v.body, v.bodyErr = msg.body, nil
		for i, e := range v.body {
			if e.Ref() == msg.moved {
				v.bodySel = i
				break
			}
		}
		return nil
	case categoriesLoadedMsg:
		v.cats = msg.cats
		if msg.err != nil {
			v.status = errorMessage(msg.err)
		}
		return nil
	case projectSavedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil // keep the overlay open to fix and retry
		}
		v.overlay.close()
		v.status = "Saved."
		return v.loadProject
	case projectArchivedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil
		}
		// The Project is gone from every view; return to the dashboard, which
		// reloads without it.
		return func() tea.Msg { return backToDashboardMsg{} }
	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

// handleKey routes a key to the open overlay, or to the screen's own actions.
func (v *projectViewModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	if cmd, handled := v.overlay.handleKey(msg); handled {
		return cmd
	}

	switch msg.String() {
	case "esc", "b":
		return func() tea.Msg { return backToDashboardMsg{} }
	case "q":
		return tea.Quit
	case "up", "k":
		if v.bodySel > 0 {
			v.bodySel--
		}
	case "down", "j":
		if v.bodySel < len(v.body)-1 {
			v.bodySel++
		}
	case "shift+up", "K":
		if v.ready() && v.hasBodySelection() {
			return v.moveEntry(v.body[v.bodySel], core.MoveUp)
		}
	case "shift+down", "J":
		if v.ready() && v.hasBodySelection() {
			return v.moveEntry(v.body[v.bodySel], core.MoveDown)
		}
	case "a":
		if v.ready() {
			f := newTaskForm("Add Task", nil)
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, 0) })
			v.status = ""
		}
	case "m":
		if v.ready() {
			f := newMilestoneForm("Add Milestone")
			v.overlay.open(&f, func() tea.Cmd { return v.addMilestone(f.name()) })
			v.status = ""
		}
	case "t":
		if task, ok := v.selectedTask(); ok && v.ready() {
			f := newTaskForm("Edit Task", &task)
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, task.ID) })
			v.status = ""
		}
	case " ":
		if task, ok := v.selectedTask(); ok && v.ready() {
			return v.toggleTask(task)
		}
	case "e":
		if v.ready() {
			p := v.project
			f := newProjectForm("Edit Project", v.cats, &p)
			v.overlay.open(&f, func() tea.Cmd { return v.editProject(f.input()) })
			v.status = ""
		}
	case "s":
		if v.ready() {
			lo := newListOverlay(lifecycleLabels(), lifecycleIndex(v.project.Lifecycle),
				"Set lifecycle state", "↑/↓: choose   enter: set   esc: cancel")
			v.overlay.open(&lo, func() tea.Cmd {
				return v.setLifecycle(lifecycleOrder[lo.selectedIndex()])
			})
			v.status = ""
		}
	case "d":
		if v.ready() {
			cu := newConfirm(fmt.Sprintf(
				"Archive %q? It moves to the Archive and leaves every view. Recover it with the archive CLI.",
				v.project.Name))
			v.overlay.open(&cu, func() tea.Cmd { return v.archiveProject() })
			v.status = ""
		}
	}
	return nil
}

// ready reports whether the Project has loaded without error, so the edit and
// lifecycle actions have something to act on.
func (v *projectViewModel) ready() bool {
	return v.loaded && v.loadErr == nil
}

// hasBodySelection reports whether bodySel points at a loaded body entry.
func (v *projectViewModel) hasBodySelection() bool {
	return v.bodySel >= 0 && v.bodySel < len(v.body)
}

// selectedTask returns the selected body entry as a Task when it is one. The
// Task-only actions (edit, toggle done) use it to stay inert on a Milestone.
func (v *projectViewModel) selectedTask() (core.Task, bool) {
	if !v.hasBodySelection() {
		return core.Task{}, false
	}
	e := v.body[v.bodySel]
	if e.Kind != core.TaskEntry {
		return core.Task{}, false
	}
	return *e.Task, true
}

// submitTaskForm turns the form's fields into a core call: an add when editID is
// zero, otherwise an edit of that Task. A malformed due date never reaches core;
// it comes back as a taskSavedMsg error so the form stays open.
func (v *projectViewModel) submitTaskForm(f *taskForm, editID int64) tea.Cmd {
	in, err := f.input()
	if err != nil {
		return func() tea.Msg { return taskSavedMsg{err: err} }
	}
	if editID == 0 {
		return v.addTask(in)
	}
	return v.editTask(editID, in)
}

// View renders the Project view or whichever overlay is open.
func (v *projectViewModel) View() string {
	if v.overlay.active() {
		return v.overlay.render() + statusBlock(v.status)
	}
	if !v.loaded {
		return "Loading Project…\n"
	}
	if v.loadErr != nil {
		return "Could not load Project: " + v.loadErr.Error() + "\n\nesc: back\n"
	}

	var b strings.Builder
	p := v.project
	fmt.Fprintf(&b, "%s\n\n", p.Name)
	fmt.Fprintf(&b, "Description: %s\n", orDash(p.Description))
	fmt.Fprintf(&b, "Category:    %s\n", v.categoryName(p.CategoryID))
	fmt.Fprintf(&b, "Lifecycle:   %s\n", p.Lifecycle)
	b.WriteString("\nBody:\n")
	b.WriteString(renderBodyRows(v.body, v.bodySel, v.bodyErr))
	b.WriteString(statusBlock(v.status))
	b.WriteString("\n↑/↓: select   shift+↑/↓: reorder   space: toggle done   a: add Task   m: add Milestone   t: edit Task\n")
	b.WriteString("e: edit Project   s: set lifecycle   d: archive   esc: back   q: quit\n")
	return b.String()
}

// renderBodyRows lists the Project's body — loose Tasks and Milestones
// interleaved in stored order — with a caret against the selected row. A loose
// Task shows a completion checkbox, any due date, and indented notes; a
// Milestone shows a diamond and its name. An empty body shows a hint; a load
// failure is surfaced, not hidden.
func renderBodyRows(body []core.BodyEntry, selected int, loadErr error) string {
	if loadErr != nil {
		return "  Could not load the body: " + loadErr.Error() + "\n"
	}
	if len(body) == 0 {
		return "  (empty — press a to add a Task or m to add a Milestone)\n"
	}
	var b strings.Builder
	for i, e := range body {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		if e.Kind == core.MilestoneEntry {
			fmt.Fprintf(&b, "%s◆ %s  (Milestone)\n", marker, e.Milestone.Name)
			continue
		}
		task := e.Task
		box := "[ ]"
		if task.Done {
			box = "[x]"
		}
		due := ""
		if task.DueDate != nil {
			due = "  (due " + task.DueDate.Format(taskDueDateLayout) + ")"
		}
		fmt.Fprintf(&b, "%s%s %s%s\n", marker, box, task.Title, due)
		if strings.TrimSpace(task.Notes) != "" {
			for _, line := range strings.Split(task.Notes, "\n") {
				fmt.Fprintf(&b, "      %s\n", line)
			}
		}
	}
	return b.String()
}

// categoryName is the display name for a Category id, or a neutral placeholder
// when the id is not in the loaded list.
func (v *projectViewModel) categoryName(id int64) string {
	for _, c := range v.cats {
		if c.ID == id {
			return c.Name
		}
	}
	return "(unknown Category)"
}

// orDash renders an em dash for an empty optional string.
func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
