package tui

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// seedMilestoneReturning adds a Milestone and returns it.
func seedMilestoneReturning(t *testing.T, c *core.Core, projectID int64, name string) core.Milestone {
	t.Helper()
	m, err := c.AddMilestone(context.Background(), projectID, name)
	if err != nil {
		t.Fatalf("AddMilestone(%q): %v", name, err)
	}
	return m
}

// seedMilestoneTasks adds titled Tasks inside a Milestone in order.
func seedMilestoneTasks(t *testing.T, c *core.Core, milestoneID int64, titles ...string) {
	t.Helper()
	for _, title := range titles {
		if _, err := c.AddMilestoneTask(context.Background(), milestoneID, core.TaskInput{Title: title}); err != nil {
			t.Fatalf("AddMilestoneTask(%q): %v", title, err)
		}
	}
}

// milestoneTaskTitles is a Milestone's Task titles in stored order.
func milestoneTaskTitles(t *testing.T, c *core.Core, milestoneID int64) []string {
	t.Helper()
	tasks, err := c.MilestoneTasks(context.Background(), milestoneID)
	if err != nil {
		t.Fatalf("MilestoneTasks: %v", err)
	}
	out := make([]string, len(tasks))
	for i, tk := range tasks {
		out[i] = tk.Title
	}
	return out
}

// The Project view renders a Milestone's Tasks nested under it, in order.
func TestProjectViewRendersMilestoneTasksNested(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Nest")
	seedTasks(t, c, p.ID, "loose lead")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1", "a2")
	seedTasks(t, c, p.ID, "loose tail")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	view := v.View()
	leadAt := strings.Index(view, "[ ] loose lead")
	alphaAt := strings.Index(view, "◆ Alpha")
	a1At := strings.Index(view, "[ ] a1")
	a2At := strings.Index(view, "[ ] a2")
	tailAt := strings.Index(view, "[ ] loose tail")
	if leadAt < 0 || alphaAt < 0 || a1At < 0 || a2At < 0 || tailAt < 0 {
		t.Fatalf("view = %q, want every body row", view)
	}
	if leadAt >= alphaAt || alphaAt >= a1At || a1At >= a2At || a2At >= tailAt {
		t.Errorf("row order = (lead %d, Alpha %d, a1 %d, a2 %d, tail %d), want nested order",
			leadAt, alphaAt, a1At, a2At, tailAt)
	}
	// Nested Tasks are indented past the Milestone header.
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "] a1") && !strings.HasPrefix(line, "    ") {
			t.Errorf("nested Task line %q, want it indented", line)
		}
	}
}

// Pressing a with a Milestone selected adds a Task inside that Milestone.
func TestProjectViewAddTaskIntoSelectedMilestone(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Add inside")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	// "Alpha" is the only row and is selected.

	v.Update(key("a"))
	if !v.overlay.active() {
		t.Fatal("pressing a did not open the add-Task form")
	}
	if !strings.Contains(v.overlay.render(), "Add Task to Milestone") {
		t.Errorf("overlay = %q, want the Milestone-scoped heading", v.overlay.render())
	}
	for _, msg := range typeString("nested step") {
		v.Update(msg)
	}
	runCmd(v.Update, v.Update(key("enter")))

	if v.overlay.active() {
		t.Error("form still open after a successful add")
	}
	if got := milestoneTaskTitles(t, c, m.ID); !slices.Equal(got, []string{"nested step"}) {
		t.Fatalf("milestone tasks = %v, want the new one inside the Milestone", got)
	}
	if got := bodyTitles(t, c, p.ID); len(got) != 0 {
		t.Errorf("loose tasks = %v, want none — the Task went into the Milestone", got)
	}
	if !strings.Contains(v.View(), "] nested step") {
		t.Errorf("view = %q, want the nested Task rendered", v.View())
	}
}

// With a nested Task selected, a adds another Task into the same Milestone.
func TestProjectViewAddTaskFromNestedSelection(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Add sibling")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down")) // select the nested "a1"

	v.Update(key("a"))
	for _, msg := range typeString("a2") {
		v.Update(msg)
	}
	runCmd(v.Update, v.Update(key("enter")))

	if got := milestoneTaskTitles(t, c, m.ID); !slices.Equal(got, []string{"a1", "a2"}) {
		t.Errorf("milestone tasks = %v, want both siblings inside the Milestone", got)
	}
}

// shift+up on a nested Task reorders it within its Milestone; the selection
// follows and the new order persists.
func TestProjectViewMoveMilestoneTaskUp(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Reorder nested")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1", "a2", "a3")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down")) // a1
	v.Update(key("down")) // a2
	runCmd(v.Update, v.Update(key("shift+up")))

	if got := milestoneTaskTitles(t, c, m.ID); !slices.Equal(got, []string{"a2", "a1", "a3"}) {
		t.Errorf("milestone tasks = %v, want a2 moved ahead of a1", got)
	}
	if got := selectedBodyLabel(v); got != "a2" {
		t.Errorf("selection = %q, want it to follow the moved Task", got)
	}
}

// shift+down on a nested Task reorders it later within its Milestone.
func TestProjectViewMoveMilestoneTaskDown(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Reorder nested")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1", "a2", "a3")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down")) // a1
	runCmd(v.Update, v.Update(key("shift+down")))

	if got := milestoneTaskTitles(t, c, m.ID); !slices.Equal(got, []string{"a2", "a1", "a3"}) {
		t.Errorf("milestone tasks = %v, want a1 moved after a2", got)
	}
	if got := selectedBodyLabel(v); got != "a1" {
		t.Errorf("selection = %q, want it to follow the moved Task", got)
	}
}

// Space toggles a nested Task's completion; t edits it.
func TestProjectViewNestedTaskToggleAndEdit(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Nested actions")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down")) // select "a1"

	runCmd(v.Update, v.Update(key(" ")))
	if got := c2MilestoneTask(t, c, m.ID, "a1"); !got.Done {
		t.Errorf("nested Task Done = false after toggle, want true")
	}

	v.Update(key("t"))
	if !v.overlay.active() {
		t.Fatal("pressing t on a nested Task did not open the edit form")
	}
	for i := 0; i < len("a1"); i++ {
		v.Update(key("backspace"))
	}
	for _, msg := range typeString("a1 renamed") {
		v.Update(msg)
	}
	runCmd(v.Update, v.Update(key("enter")))

	if got := milestoneTaskTitles(t, c, m.ID); !slices.Equal(got, []string{"a1 renamed"}) {
		t.Errorf("milestone tasks = %v, want the nested Task renamed", got)
	}
}

// Reordering a Milestone in the body carries its nested Tasks with it in the
// Project view.
func TestProjectViewMoveMilestoneCarriesTasks(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Travel")
	seedTasks(t, c, p.ID, "loose first")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1", "a2")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down")) // select "Alpha" (row 1; row 0 is "loose first")
	runCmd(v.Update, v.Update(key("shift+up")))

	if got := bodyLabelsOf(t, c, p.ID); !slices.Equal(got, []string{"ms:Alpha", "task:loose first"}) {
		t.Fatalf("body = %v, want Alpha moved to the front", got)
	}
	view := v.View()
	alphaAt := strings.Index(view, "◆ Alpha")
	a1At := strings.Index(view, "] a1")
	a2At := strings.Index(view, "] a2")
	looseAt := strings.Index(view, "[ ] loose first")
	if alphaAt < 0 || alphaAt >= a1At || a1At >= a2At || a2At >= looseAt {
		t.Errorf("view order = (Alpha %d, a1 %d, a2 %d, loose %d), want the nested Tasks to travel with Alpha",
			alphaAt, a1At, a2At, looseAt)
	}
	if got := selectedBodyLabel(v); got != "Alpha" {
		t.Errorf("selection = %q, want it to follow the moved Milestone", got)
	}
}

// The dashboard Next step points at the first incomplete Task inside a Milestone
// when the Milestone is the first incomplete entry.
func TestDashboardNextStepPointsIntoMilestone(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Board")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "inside step")

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	if step, ok := d.nextSteps[p.ID]; !ok || step.Title != "inside step" {
		t.Errorf("nextSteps[%d] = (%+v, %v), want the Milestone's Task", p.ID, step, ok)
	}
	if !strings.Contains(d.View(), "Next step: inside step") {
		t.Errorf("view = %q, want the nested Next step shown", d.View())
	}
}

// The dashboard Next step skips an empty Milestone and an all-done Milestone,
// landing on the first later entry with open work.
func TestDashboardNextStepSkipsEmptyAndAllDoneMilestones(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Skips")
	seedMilestoneReturning(t, c, p.ID, "Empty")
	done := seedMilestoneReturning(t, c, p.ID, "Done")
	seedMilestoneTasks(t, c, done.ID, "d1")
	live := seedMilestoneReturning(t, c, p.ID, "Live")
	seedMilestoneTasks(t, c, live.ID, "l1")

	doneTask := c2MilestoneTask(t, c, done.ID, "d1")
	if _, err := c.SetTaskDone(context.Background(), doneTask.ID, true); err != nil {
		t.Fatalf("SetTaskDone: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	if step, ok := d.nextSteps[p.ID]; !ok || step.Title != "l1" {
		t.Errorf("nextSteps[%d] = (%+v, %v), want %q past the empty and all-done Milestones", p.ID, step, ok, "l1")
	}
}

// c2MilestoneTask returns a Milestone Task by title.
func c2MilestoneTask(t *testing.T, c *core.Core, milestoneID int64, title string) core.Task {
	t.Helper()
	tasks, err := c.MilestoneTasks(context.Background(), milestoneID)
	if err != nil {
		t.Fatalf("MilestoneTasks: %v", err)
	}
	for _, tk := range tasks {
		if tk.Title == title {
			return tk
		}
	}
	t.Fatalf("milestone task %q not found", title)
	return core.Task{}
}
