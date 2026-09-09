package tui

import (
	"fmt"
	"strings"

	"github.com/spowers42/project_organizer/core"
)

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

// sortBodyRowsByPriority returns the flattened rows with each run of sibling
// Task rows reordered Priority-first (stable) — a run of loose Tasks, or one
// Milestone's own Tasks. Milestone headers stay where they are and no Task
// crosses a header, so the display sort respects the body's levels (ADR 0001).
func sortBodyRowsByPriority(rows []bodyRow) []bodyRow {
	out := make([]bodyRow, 0, len(rows))
	for i := 0; i < len(rows); {
		if rows[i].kind == milestoneHeadRow {
			out = append(out, rows[i])
			i++
			continue
		}
		runKind := rows[i].kind // looseTaskRow or milestoneTaskRow
		j := i
		for j < len(rows) && rows[j].kind == runKind {
			j++
		}
		out = append(out, priorityFirstRun(rows[i:j])...)
		i = j
	}
	return out
}

// priorityFirstRun reorders one run of Task rows Priority-first, reusing
// core.TasksByPriority so the view and the domain agree on what the order is.
func priorityFirstRun(run []bodyRow) []bodyRow {
	if len(run) < 2 {
		return run
	}
	byID := make(map[int64]bodyRow, len(run))
	tasks := make([]core.Task, len(run))
	for i, r := range run {
		tasks[i] = r.task
		byID[r.task.ID] = r
	}
	sorted := core.TasksByPriority(tasks)
	out := make([]bodyRow, len(run))
	for i, tk := range sorted {
		out[i] = byID[tk.ID]
	}
	return out
}

// projectBody is the Project view's cursor over the flattened body: the loaded
// entries, the selectable rows built from them, which one is selected, and any
// body-load error. It maps a cursor position to the core.BodyRef /
// taskFormTarget an action needs and follows the selection across reloads. It
// holds no core.Body and performs no persistence — the screen calls core for
// every mutation.
//
// sortByPriority is a display toggle: when on, the rows are shown Priority-first
// within each list (loose Tasks, and each Milestone's own Tasks) without ever
// crossing a Milestone boundary. It reorders nothing stored — the entries, and
// so the Project body (ADR 0001), are untouched.
type projectBody struct {
	entries        []core.BodyEntry
	rows           []bodyRow
	sel            int
	sortByPriority bool
	loadErr        error
}

// buildRows flattens the loaded entries into selectable rows, applying the
// Priority-first display sort when it is on.
func (p *projectBody) buildRows() {
	rows := flattenBody(p.entries)
	if p.sortByPriority {
		rows = sortBodyRowsByPriority(rows)
	}
	p.rows = rows
}

// setBody swaps in a freshly loaded body, keeping the selection index valid.
func (p *projectBody) setBody(entries []core.BodyEntry, loadErr error) {
	p.loadErr = loadErr
	p.entries = entries
	p.buildRows()
	if p.sel >= len(p.rows) {
		p.sel = 0
	}
}

// apply swaps in a freshly loaded body and moves the selection onto the row
// named by keep, clamping when that row is gone.
func (p *projectBody) apply(entries []core.BodyEntry, keep bodyRowKey) {
	p.loadErr = nil
	p.entries = entries
	p.buildRows()
	if p.selectKey(keep) {
		return
	}
	if p.sel >= len(p.rows) {
		p.sel = 0
	}
}

// selectKey moves the selection onto the row named by keep, reporting whether
// such a row is present.
func (p *projectBody) selectKey(keep bodyRowKey) bool {
	for i, r := range p.rows {
		if r.key() == keep {
			p.sel = i
			return true
		}
	}
	return false
}

// toggleSort flips the Priority-first display sort and rebuilds the rows,
// keeping the cursor on the same entry when it can. Nothing stored changes.
func (p *projectBody) toggleSort() {
	var keep bodyRowKey
	if r, ok := p.selectedRow(); ok {
		keep = r.key()
	}
	p.sortByPriority = !p.sortByPriority
	p.buildRows()
	if !p.selectKey(keep) && p.sel >= len(p.rows) {
		p.sel = 0
	}
}

// up moves the selection one row towards the top.
func (p *projectBody) up() {
	if p.sel > 0 {
		p.sel--
	}
}

// down moves the selection one row towards the bottom.
func (p *projectBody) down() {
	if p.sel < len(p.rows)-1 {
		p.sel++
	}
}

// hasSelection reports whether sel points at a loaded row.
func (p *projectBody) hasSelection() bool {
	return p.sel >= 0 && p.sel < len(p.rows)
}

// selectedRow returns the currently selected row, if any.
func (p *projectBody) selectedRow() (bodyRow, bool) {
	if !p.hasSelection() {
		return bodyRow{}, false
	}
	return p.rows[p.sel], true
}

// selectedTask returns the selected row as a Task when it is one — a loose Task
// or a Task inside a Milestone. The Task-only actions (edit, toggle done) use it
// to stay inert on a Milestone header.
func (p *projectBody) selectedTask() (core.Task, bool) {
	r, ok := p.selectedRow()
	if !ok || r.kind == milestoneHeadRow {
		return core.Task{}, false
	}
	return r.task, true
}

// looseInsertTarget decides where the "add Task" command (a) drops a new loose
// Task: always in the Project body, just below the selection. On a loose Task it
// lands right after that Task; on a Milestone header or one of its nested Tasks
// it lands right after that whole Milestone (a loose Task never falls inside
// one); with no selection it appends to the Project body.
func (p *projectBody) looseInsertTarget() (taskFormTarget, string) {
	return taskFormTarget{afterRef: p.slotBelowCursor()}, "Add Task"
}

// milestoneInsertTarget decides where the "add Task to Milestone" command (A)
// drops a new Task: inside the Milestone at or containing the cursor. On a
// header it becomes the Milestone's first Task; on a nested Task it lands right
// after that one. ok is false when the cursor is not on or inside any Milestone,
// so the command stays inert there.
func (p *projectBody) milestoneInsertTarget() (taskFormTarget, string, bool) {
	r, ok := p.selectedRow()
	if !ok {
		return taskFormTarget{}, "", false
	}
	switch r.kind {
	case milestoneHeadRow:
		return taskFormTarget{milestoneID: r.milestone.ID}, "Add Task to Milestone", true
	case milestoneTaskRow:
		return taskFormTarget{milestoneID: r.milestoneID, afterTaskID: r.task.ID}, "Add Task to Milestone", true
	default:
		return taskFormTarget{}, "", false
	}
}

// slotBelowCursor is the body slot a new entry — a loose Task (a) or a Milestone
// (m) — should land just after: the selection's place in the body. On a loose
// Task or a Milestone header it is that entry; on a nested Task it is the
// enclosing Milestone (a Milestone cannot nest). With no selection it is the
// zero BodyRef, which inserts at the front.
func (p *projectBody) slotBelowCursor() core.BodyRef {
	r, ok := p.selectedRow()
	if !ok {
		return core.BodyRef{}
	}
	switch r.kind {
	case milestoneHeadRow:
		return core.BodyRef{Kind: core.MilestoneEntry, ID: r.milestone.ID}
	case milestoneTaskRow:
		return core.BodyRef{Kind: core.MilestoneEntry, ID: r.milestoneID}
	default:
		return core.BodyRef{Kind: core.TaskEntry, ID: r.task.ID}
	}
}

// milestones lists the body's Milestones in body order, for the "move into a
// Milestone" picker (>).
func (p *projectBody) milestones() []core.Milestone {
	var ms []core.Milestone
	for _, r := range p.rows {
		if r.kind == milestoneHeadRow {
			ms = append(ms, r.milestone)
		}
	}
	return ms
}

// render draws the body — loose Tasks and Milestones interleaved, each
// Milestone's own Tasks nested beneath it — with a caret against the selected
// row.
func (p *projectBody) render() string {
	return renderBodyRows(p.rows, p.sel, p.loadErr)
}

// renderBodyRows lists the Project's body — loose Tasks and Milestones
// interleaved in stored order, each Milestone's own Tasks nested beneath it —
// with a caret against the selected row. A Task shows a completion checkbox, a
// Priority star when set, any due date, and its notes; a Milestone shows a
// diamond and its name. An empty body shows a hint; a load failure is surfaced,
// not hidden.
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
		fmt.Fprintf(&b, "%s%s%s %s%s%s\n", marker, indent, box, priorityStar(r.task.Priority), r.task.Title, due)
		if strings.TrimSpace(r.task.Notes) != "" {
			for _, line := range strings.Split(r.task.Notes, "\n") {
				fmt.Fprintf(&b, "%s%s\n", noteIndent, line)
			}
		}
	}
	return b.String()
}
