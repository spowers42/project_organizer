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
	body      projectBody // flattened body plus cursor and selection
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

// taskSavedMsg is the result of an edit or a complete-toggle on a Task. A nil
// err means the change persisted.
type taskSavedMsg struct {
	err error
}

// entryAddedMsg is the result of adding a body entry — a Task or a Milestone. On
// success it carries the reloaded body plus the new entry's row key, so it shows
// in place with the selection moved onto it.
type entryAddedMsg struct {
	body []core.BodyEntry
	sel  bodyRowKey
	err  error
}

// bodyMovedMsg is the result of reordering a row — a loose Task, a Milestone, or
// a Task inside a Milestone. On success it carries the reloaded body plus the
// moved row's key, so the selection can follow it.
type bodyMovedMsg struct {
	body []core.BodyEntry
	sel  bodyRowKey
	err  error
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

// editTask rewrites a Task's title and due date from the form's fields.
func (v *projectViewModel) editTask(id int64, in core.TaskInput) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.EditTask(context.Background(), id, in)
		return taskSavedMsg{err: err}
	}
}

// addTaskCmd runs a Task-creating core call, then reloads the body so the new
// Task shows in its place with the selection moved onto it.
func (v *projectViewModel) addTaskCmd(create func(context.Context) (core.Task, error)) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		t, err := create(ctx)
		if err != nil {
			return entryAddedMsg{err: err}
		}
		kind := looseTaskRow
		if t.MilestoneID != nil {
			kind = milestoneTaskRow
		}
		body, err := v.core.ProjectBody(ctx, v.projectID)
		return entryAddedMsg{body: body, sel: bodyRowKey{kind: kind, id: t.ID}, err: err}
	}
}

// addMilestoneCmd runs a Milestone-creating core call, then reloads the body so
// the new Milestone shows in its place with the selection moved onto it.
func (v *projectViewModel) addMilestoneCmd(create func(context.Context) (core.Milestone, error)) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		m, err := create(ctx)
		if err != nil {
			return entryAddedMsg{err: err}
		}
		body, err := v.core.ProjectBody(ctx, v.projectID)
		return entryAddedMsg{body: body, sel: bodyRowKey{kind: milestoneHeadRow, id: m.ID}, err: err}
	}
}

// toggleTask flips the selected Task's completion flag.
func (v *projectViewModel) toggleTask(task core.Task) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.SetTaskDone(context.Background(), task.ID, !task.Done)
		return taskSavedMsg{err: err}
	}
}

// moveRow reorders the selected row one slot up or down, dispatching by its
// kind: a loose Task and a Milestone reorder within the Project body, a Task
// inside a Milestone within that Milestone. It reloads the body so a Milestone
// move brings its nested Tasks along, and reports the moved row's key so the
// selection can follow.
func (v *projectViewModel) moveRow(r bodyRow, dir core.MoveDir) tea.Cmd {
	key := r.key()
	return func() tea.Msg {
		ctx := context.Background()
		var (
			body []core.BodyEntry
			err  error
		)
		switch r.kind {
		case milestoneHeadRow:
			body, err = v.core.MoveMilestone(ctx, r.milestone.ID, dir)
		case milestoneTaskRow:
			// The Milestone mover returns only that Milestone's Tasks, so reload
			// the whole body to place the nested change back under its header.
			if _, err = v.core.MoveMilestoneTask(ctx, r.task.ID, dir); err == nil {
				body, err = v.core.ProjectBody(ctx, v.projectID)
			}
		default:
			body, err = v.core.MoveTask(ctx, r.task.ID, dir)
		}
		return bodyMovedMsg{body: body, sel: key, err: err}
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
		v.body.setBody(msg.body, msg.err)
		return nil
	case taskSavedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil // keep the overlay open to fix and retry
		}
		v.overlay.close()
		v.status = "Saved."
		return v.loadBody
	case entryAddedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil // keep the overlay open to fix and retry
		}
		v.overlay.close()
		v.status = "Saved."
		v.body.apply(msg.body, msg.sel)
		return nil
	case bodyMovedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil
		}
		v.body.apply(msg.body, msg.sel)
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
		v.body.up()
	case "down", "j":
		v.body.down()
	case "shift+up", "K":
		if r, ok := v.body.selectedRow(); ok && v.ready() {
			return v.moveRow(r, core.MoveUp)
		}
	case "shift+down", "J":
		if r, ok := v.body.selectedRow(); ok && v.ready() {
			return v.moveRow(r, core.MoveDown)
		}
	case "a":
		if v.ready() {
			target, heading := v.body.looseInsertTarget()
			f := newTaskForm(heading, nil)
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, target) })
			v.status = ""
		}
	case "A":
		if v.ready() {
			target, heading, ok := v.body.milestoneInsertTarget()
			if !ok {
				v.status = "No Milestone here — put the cursor on a Milestone or one of its Tasks."
				break
			}
			f := newTaskForm(heading, nil)
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, target) })
			v.status = ""
		}
	case "m":
		if v.ready() {
			anchor := v.body.slotBelowCursor()
			f := newMilestoneForm("Add Milestone")
			v.overlay.open(&f, func() tea.Cmd {
				name := f.name()
				return v.addMilestoneCmd(func(ctx context.Context) (core.Milestone, error) {
					return v.core.AddMilestoneAfter(ctx, v.projectID, anchor, name)
				})
			})
			v.status = ""
		}
	case "t":
		if task, ok := v.body.selectedTask(); ok && v.ready() {
			f := newTaskForm("Edit Task", &task)
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, taskFormTarget{editID: task.ID}) })
			v.status = ""
		}
	case " ":
		if task, ok := v.body.selectedTask(); ok && v.ready() {
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

// taskFormTarget says what a submitted Task form does:
//   - editID set: edit that Task.
//   - milestoneID set: insert into that Milestone just after afterTaskID
//     (afterTaskID 0 puts it first).
//   - afterRef set alone: insert a loose Task just after that body slot — a
//     loose Task or a Milestone.
//   - all zero: append a loose Task to the Project body.
type taskFormTarget struct {
	editID      int64
	milestoneID int64
	afterTaskID int64
	afterRef    core.BodyRef
}

// submitTaskForm turns the form's fields into the core call named by target. A
// malformed due date never reaches core; it comes back as a taskSavedMsg error
// so the form stays open.
func (v *projectViewModel) submitTaskForm(f *taskForm, target taskFormTarget) tea.Cmd {
	in, err := f.input()
	if err != nil {
		return func() tea.Msg { return taskSavedMsg{err: err} }
	}
	switch {
	case target.editID != 0:
		return v.editTask(target.editID, in)
	case target.milestoneID != 0:
		mid, after := target.milestoneID, target.afterTaskID
		return v.addTaskCmd(func(ctx context.Context) (core.Task, error) {
			return v.core.AddMilestoneTaskAfter(ctx, mid, after, in)
		})
	case target.afterRef.ID != 0:
		after := target.afterRef
		return v.addTaskCmd(func(ctx context.Context) (core.Task, error) {
			return v.core.AddTaskAfter(ctx, v.projectID, after, in)
		})
	default:
		return v.addTaskCmd(func(ctx context.Context) (core.Task, error) {
			return v.core.AddTask(ctx, v.projectID, in)
		})
	}
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
	b.WriteString(v.body.render())
	b.WriteString(statusBlock(v.status))
	b.WriteString("\n↑/↓: select   shift+↑/↓: reorder   space: toggle done   t: edit Task\n")
	b.WriteString("a: add Task   A: add Task to Milestone   m: add Milestone\n")
	b.WriteString("e: edit Project   s: set lifecycle   d: archive   esc: back   q: quit\n")
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
