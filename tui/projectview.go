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
	rows      []bodyRow // body flattened for selection: entries plus Milestone Tasks
	bodySel   int       // index into rows
	bodyErr   error
	loaded    bool
	loadErr   error
	status    string
	overlay   overlayHost
}

// bodyRowKind classifies one selectable line of the Project body.
type bodyRowKind int

const (
	looseTaskRow bodyRowKind = iota
	milestoneHeadRow
	milestoneTaskRow
)

// bodyRow is one selectable line of the Project body: a loose Task, a Milestone
// header, or a Task nested inside a Milestone. Flattening the interleaved body
// into these rows lets the user walk and reorder the whole nested structure
// with a single selection index.
type bodyRow struct {
	kind        bodyRowKind
	task        core.Task      // set for looseTaskRow and milestoneTaskRow
	milestone   core.Milestone // set for milestoneHeadRow
	milestoneID int64          // owning Milestone id, set for milestoneTaskRow
}

// bodyRowKey identifies a row across a body reload, so the selection can follow
// a moved or edited entry.
type bodyRowKey struct {
	kind bodyRowKind
	id   int64
}

// key is the row's stable identity: the Milestone id for a header, otherwise
// the Task id.
func (r bodyRow) key() bodyRowKey {
	if r.kind == milestoneHeadRow {
		return bodyRowKey{kind: milestoneHeadRow, id: r.milestone.ID}
	}
	return bodyRowKey{kind: r.kind, id: r.task.ID}
}

// flattenBody expands the interleaved body into selectable rows: each Milestone
// header is immediately followed by its ordered Tasks.
func flattenBody(body []core.BodyEntry) []bodyRow {
	var rows []bodyRow
	for _, e := range body {
		if e.Kind == core.MilestoneEntry {
			rows = append(rows, bodyRow{kind: milestoneHeadRow, milestone: *e.Milestone})
			for _, mt := range e.Milestone.Tasks {
				rows = append(rows, bodyRow{kind: milestoneTaskRow, task: mt, milestoneID: e.Milestone.ID})
			}
			continue
		}
		rows = append(rows, bodyRow{kind: looseTaskRow, task: *e.Task})
	}
	return rows
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

// addMilestoneTask appends a Task inside the given Milestone from the form's
// fields.
func (v *projectViewModel) addMilestoneTask(milestoneID int64, in core.TaskInput) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.AddMilestoneTask(context.Background(), milestoneID, in)
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
		v.body, v.bodyErr = msg.body, msg.err
		v.rows = flattenBody(msg.body)
		if v.bodySel >= len(v.rows) {
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
		v.rows = flattenBody(msg.body)
		for i, r := range v.rows {
			if r.key() == msg.sel {
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
		if v.bodySel < len(v.rows)-1 {
			v.bodySel++
		}
	case "shift+up", "K":
		if v.ready() && v.hasBodySelection() {
			return v.moveRow(v.rows[v.bodySel], core.MoveUp)
		}
	case "shift+down", "J":
		if v.ready() && v.hasBodySelection() {
			return v.moveRow(v.rows[v.bodySel], core.MoveDown)
		}
	case "a":
		if v.ready() {
			target := taskFormTarget{}
			heading := "Add Task"
			if mid, ok := v.selectedMilestoneID(); ok {
				target = taskFormTarget{milestoneID: mid}
				heading = "Add Task to Milestone"
			}
			f := newTaskForm(heading, nil)
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, target) })
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
			v.overlay.open(&f, func() tea.Cmd { return v.submitTaskForm(&f, taskFormTarget{editID: task.ID}) })
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

// hasBodySelection reports whether bodySel points at a loaded row.
func (v *projectViewModel) hasBodySelection() bool {
	return v.bodySel >= 0 && v.bodySel < len(v.rows)
}

// selectedRow returns the currently selected row, if any.
func (v *projectViewModel) selectedRow() (bodyRow, bool) {
	if !v.hasBodySelection() {
		return bodyRow{}, false
	}
	return v.rows[v.bodySel], true
}

// selectedTask returns the selected row as a Task when it is one — a loose Task
// or a Task inside a Milestone. The Task-only actions (edit, toggle done) use it
// to stay inert on a Milestone header.
func (v *projectViewModel) selectedTask() (core.Task, bool) {
	r, ok := v.selectedRow()
	if !ok || r.kind == milestoneHeadRow {
		return core.Task{}, false
	}
	return r.task, true
}

// selectedMilestoneID returns the Milestone the selection sits in — the header
// itself, or a Task nested under it — so "add Task" can target that Milestone.
func (v *projectViewModel) selectedMilestoneID() (int64, bool) {
	r, ok := v.selectedRow()
	if !ok {
		return 0, false
	}
	switch r.kind {
	case milestoneHeadRow:
		return r.milestone.ID, true
	case milestoneTaskRow:
		return r.milestoneID, true
	default:
		return 0, false
	}
}

// taskFormTarget says what a submitted Task form does: edit an existing Task
// (editID set), add one inside a Milestone (milestoneID set), or — both zero —
// add a loose Task to the Project body.
type taskFormTarget struct {
	editID      int64
	milestoneID int64
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
		return v.addMilestoneTask(target.milestoneID, in)
	default:
		return v.addTask(in)
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
	b.WriteString(renderBodyRows(v.rows, v.bodySel, v.bodyErr))
	b.WriteString(statusBlock(v.status))
	b.WriteString("\n↑/↓: select   shift+↑/↓: reorder   space: toggle done   a: add Task   m: add Milestone   t: edit Task\n")
	b.WriteString("e: edit Project   s: set lifecycle   d: archive   esc: back   q: quit\n")
	return b.String()
}

// renderBodyRows lists the Project's body — loose Tasks and Milestones
// interleaved in stored order, each Milestone's own Tasks nested beneath it —
// with a caret against the selected row. A Task shows a completion checkbox, any
// due date, and its notes; a Milestone shows a diamond and its name. An empty
// body shows a hint; a load failure is surfaced, not hidden.
func renderBodyRows(rows []bodyRow, selected int, loadErr error) string {
	if loadErr != nil {
		return "  Could not load the body: " + loadErr.Error() + "\n"
	}
	if len(rows) == 0 {
		return "  (empty — press a to add a Task or m to add a Milestone)\n"
	}
	var b strings.Builder
	for i, r := range rows {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		if r.kind == milestoneHeadRow {
			fmt.Fprintf(&b, "%s◆ %s  (Milestone)\n", marker, r.milestone.Name)
			continue
		}
		// A Task inside a Milestone is indented one step past a loose Task.
		indent, noteIndent := "", "      "
		if r.kind == milestoneTaskRow {
			indent, noteIndent = "  ", "        "
		}
		box := "[ ]"
		if r.task.Done {
			box = "[x]"
		}
		due := ""
		if r.task.DueDate != nil {
			due = "  (due " + r.task.DueDate.Format(taskDueDateLayout) + ")"
		}
		fmt.Fprintf(&b, "%s%s%s %s%s\n", marker, indent, box, r.task.Title, due)
		if strings.TrimSpace(r.task.Notes) != "" {
			for _, line := range strings.Split(r.task.Notes, "\n") {
				fmt.Fprintf(&b, "%s%s\n", noteIndent, line)
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
