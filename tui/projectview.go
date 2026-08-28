package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// projectViewModel is the second screen: one Project's fields and its ordered
// body of loose Tasks, with actions to edit the Project, move it through its
// lifecycle, and add / edit / complete Tasks. It reads core and mutates through
// core; it holds no domain logic.
type projectViewModel struct {
	core      *core.Core
	projectID int64
	project   core.Project
	cats      []core.Category
	tasks     []core.Task
	taskSel   int
	tasksErr  error
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

// tasksLoadedMsg carries the result of loading the Project's loose Tasks.
type tasksLoadedMsg struct {
	tasks []core.Task
	err   error
}

// taskSavedMsg is the result of an add / edit / complete-toggle on a Task. A
// nil err means the change persisted.
type taskSavedMsg struct {
	err error
}

// Init loads the Project, its Tasks, and the shared Category list.
func (v *projectViewModel) Init() tea.Cmd {
	return tea.Batch(v.loadProject, v.loadTasks, loadCategoriesCmd(v.core))
}

// loadProject reads the viewed Project by id.
func (v *projectViewModel) loadProject() tea.Msg {
	p, err := v.core.GetProject(context.Background(), v.projectID)
	return projectLoadedMsg{project: p, err: err}
}

// loadTasks reads the Project's loose Tasks in body order.
func (v *projectViewModel) loadTasks() tea.Msg {
	ts, err := v.core.ProjectTasks(context.Background(), v.projectID)
	return tasksLoadedMsg{tasks: ts, err: err}
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
	case tasksLoadedMsg:
		v.tasks, v.tasksErr = msg.tasks, msg.err
		if v.taskSel >= len(v.tasks) {
			v.taskSel = 0
		}
		return nil
	case taskSavedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil // keep the overlay open to fix and retry
		}
		v.overlay.close()
		v.status = "Saved."
		return v.loadTasks
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
		if v.taskSel > 0 {
			v.taskSel--
		}
	case "down", "j":
		if v.taskSel < len(v.tasks)-1 {
			v.taskSel++
		}
	case "a":
		if v.ready() {
			f := newTaskForm("Add Task", nil)
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, 0) })
			v.status = ""
		}
	case "t":
		if v.ready() && v.hasTaskSelection() {
			task := v.tasks[v.taskSel]
			f := newTaskForm("Edit Task", &task)
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, task.ID) })
			v.status = ""
		}
	case " ":
		if v.ready() && v.hasTaskSelection() {
			return v.toggleTask(v.tasks[v.taskSel])
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

// hasTaskSelection reports whether taskSel points at a loaded Task.
func (v *projectViewModel) hasTaskSelection() bool {
	return v.taskSel >= 0 && v.taskSel < len(v.tasks)
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
	b.WriteString("\nTasks:\n")
	b.WriteString(renderTaskRows(v.tasks, v.taskSel, v.tasksErr))
	b.WriteString(statusBlock(v.status))
	b.WriteString("\n↑/↓: select Task   space: toggle done   a: add Task   t: edit Task\n")
	b.WriteString("e: edit Project   s: set lifecycle   d: archive   esc: back   q: quit\n")
	return b.String()
}

// renderTaskRows lists the Project's loose Tasks in body order with a caret
// against the selected row, a checkbox for completion, and any due date. An
// empty list shows a hint; a load failure is surfaced, not hidden.
func renderTaskRows(tasks []core.Task, selected int, loadErr error) string {
	if loadErr != nil {
		return "  Could not load Tasks: " + loadErr.Error() + "\n"
	}
	if len(tasks) == 0 {
		return "  (no Tasks yet — press a to add one)\n"
	}
	var b strings.Builder
	for i, task := range tasks {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		box := "[ ]"
		if task.Done {
			box = "[x]"
		}
		due := ""
		if task.DueDate != nil {
			due = "  (due " + task.DueDate.Format(taskDueDateLayout) + ")"
		}
		fmt.Fprintf(&b, "%s%s %s%s\n", marker, box, task.Title, due)
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
