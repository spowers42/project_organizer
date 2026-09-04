package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// mustAddMilestoneTask adds a Task inside a Milestone through core and fails the
// test on error.
func mustAddMilestoneTask(t *testing.T, c *core.Core, milestoneID int64, title string) core.Task {
	t.Helper()
	task, err := c.AddMilestoneTask(context.Background(), milestoneID, core.TaskInput{Title: title})
	if err != nil {
		t.Fatalf("AddMilestoneTask(%q): %v", title, err)
	}
	return task
}

// milestoneTaskTitles is the ordered titles of a Milestone's Tasks as core
// reports them.
func milestoneTaskTitles(t *testing.T, c *core.Core, milestoneID int64) []string {
	t.Helper()
	tasks, err := c.MilestoneTasks(context.Background(), milestoneID)
	if err != nil {
		t.Fatalf("MilestoneTasks: %v", err)
	}
	return taskTitles(tasks)
}

func TestAddMilestoneTaskAppendsInMilestoneOrder(t *testing.T) {
	c, _ := newTestCore(t)
	p := mustCreateProject(t, c, "Ship it", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")

	first := mustAddMilestoneTask(t, c, m.ID, "wire it up")
	mustAddMilestoneTask(t, c, m.ID, "test it")
	mustAddMilestoneTask(t, c, m.ID, "document it")

	if first.MilestoneID == nil || *first.MilestoneID != m.ID {
		t.Errorf("MilestoneID = %v, want %d", first.MilestoneID, m.ID)
	}
	if first.ProjectID != p.ID {
		t.Errorf("ProjectID = %d, want the Milestone's project %d", first.ProjectID, p.ID)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"wire it up", "test it", "document it"}) {
		t.Errorf("milestone tasks = %v, want them in insertion order", got)
	}
	// Milestone Tasks never show up as loose Project Tasks.
	loose, err := c.ProjectTasks(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if len(loose) != 0 {
		t.Errorf("loose tasks = %v, want none (all are inside the Milestone)", taskTitles(loose))
	}
}

func TestAddMilestoneTaskValidation(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")

	if _, err := c.AddMilestoneTask(ctx, m.ID, core.TaskInput{Title: "   "}); !errors.Is(err, core.ErrEmptyTaskTitle) {
		t.Errorf("empty title: error = %v, want ErrEmptyTaskTitle", err)
	}
	if _, err := c.AddMilestoneTask(ctx, 9999, core.TaskInput{Title: "orphan"}); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("unknown milestone: error = %v, want ErrMilestoneNotFound", err)
	}

	padded, err := c.AddMilestoneTask(ctx, m.ID, core.TaskInput{Title: "  trim me  "})
	if err != nil {
		t.Fatalf("AddMilestoneTask(padded): %v", err)
	}
	if padded.Title != "trim me" {
		t.Errorf("Title = %q, want it trimmed", padded.Title)
	}
}

func TestAddTaskAfterDropsBelowTheCursorSlot(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Insert", categoryID(t, c, "Programming"))

	first := mustAddTask(t, c, p.ID, "first")
	mustAddTask(t, c, p.ID, "third")
	gate := mustAddMilestone(t, c, p.ID, "Gate") // body: first, third, Gate

	// Insert "second" just after "first".
	inserted, err := c.AddTaskAfter(ctx, p.ID, core.BodyRef{Kind: core.TaskEntry, ID: first.ID}, core.TaskInput{Title: "second"})
	if err != nil {
		t.Fatalf("AddTaskAfter: %v", err)
	}
	if inserted.MilestoneID != nil {
		t.Errorf("inserted.MilestoneID = %v, want nil (a loose Task)", inserted.MilestoneID)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:first", "task:second", "task:third", "ms:Gate"}) {
		t.Errorf("body = %v, want second between first and third with Gate still last", got)
	}

	// A Milestone is a valid anchor: the loose Task lands right after it, still
	// loose — never inside it.
	afterGate, err := c.AddTaskAfter(ctx, p.ID, core.BodyRef{Kind: core.MilestoneEntry, ID: gate.ID}, core.TaskInput{Title: "tail"})
	if err != nil {
		t.Fatalf("AddTaskAfter(after Milestone): %v", err)
	}
	if afterGate.MilestoneID != nil {
		t.Errorf("afterGate.MilestoneID = %v, want nil (still a loose Task)", afterGate.MilestoneID)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:first", "task:second", "task:third", "ms:Gate", "task:tail"}) {
		t.Errorf("body = %v, want tail as a loose Task right after Gate", got)
	}

	// A zero anchor inserts at the front; Next step follows.
	if _, err := c.AddTaskAfter(ctx, p.ID, core.BodyRef{}, core.TaskInput{Title: "zero"}); err != nil {
		t.Fatalf("AddTaskAfter(front): %v", err)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:zero", "task:first", "task:second", "task:third", "ms:Gate", "task:tail"}) {
		t.Errorf("body = %v, want zero first", got)
	}
	assertNextStep(t, c, p.ID, looseTaskByTitle(t, c, p.ID, "zero").ID, "zero")
}

func TestAddTaskAfterErrorCases(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Programming"))
	loose := mustAddTask(t, c, p.ID, "loose")
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	nested := mustAddMilestoneTask(t, c, m.ID, "nested")

	looseRef := core.BodyRef{Kind: core.TaskEntry, ID: loose.ID}
	if _, err := c.AddTaskAfter(ctx, 9090, looseRef, core.TaskInput{Title: "x"}); !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("unknown project: error = %v, want ErrProjectNotFound", err)
	}
	if _, err := c.AddTaskAfter(ctx, p.ID, looseRef, core.TaskInput{Title: "  "}); !errors.Is(err, core.ErrEmptyTaskTitle) {
		t.Errorf("blank title: error = %v, want ErrEmptyTaskTitle", err)
	}
	if _, err := c.AddTaskAfter(ctx, p.ID, core.BodyRef{Kind: core.TaskEntry, ID: 7777}, core.TaskInput{Title: "x"}); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown loose-Task anchor: error = %v, want ErrTaskNotFound", err)
	}
	if _, err := c.AddTaskAfter(ctx, p.ID, core.BodyRef{Kind: core.MilestoneEntry, ID: 7777}, core.TaskInput{Title: "x"}); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("unknown Milestone anchor: error = %v, want ErrMilestoneNotFound", err)
	}
	// A Milestone Task is not a top-level body slot.
	if _, err := c.AddTaskAfter(ctx, p.ID, core.BodyRef{Kind: core.TaskEntry, ID: nested.ID}, core.TaskInput{Title: "x"}); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("milestone task as anchor: error = %v, want ErrTaskNotFound", err)
	}
}

func TestAddMilestoneTaskAfterDropsBelowTheCursorSlot(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Insert nested", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	a1 := mustAddMilestoneTask(t, c, m.ID, "a1")
	mustAddMilestoneTask(t, c, m.ID, "a3")

	inserted, err := c.AddMilestoneTaskAfter(ctx, m.ID, a1.ID, core.TaskInput{Title: "a2"})
	if err != nil {
		t.Fatalf("AddMilestoneTaskAfter: %v", err)
	}
	if inserted.MilestoneID == nil || *inserted.MilestoneID != m.ID {
		t.Errorf("inserted.MilestoneID = %v, want %d", inserted.MilestoneID, m.ID)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"a1", "a2", "a3"}) {
		t.Errorf("milestone tasks = %v, want a2 inserted after a1", got)
	}

	// afterTaskID 0 inserts at the front of the Milestone.
	if _, err := c.AddMilestoneTaskAfter(ctx, m.ID, 0, core.TaskInput{Title: "a0"}); err != nil {
		t.Fatalf("AddMilestoneTaskAfter(front): %v", err)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"a0", "a1", "a2", "a3"}) {
		t.Errorf("milestone tasks = %v, want a0 first", got)
	}
}

func TestAddMilestoneTaskAfterErrorCases(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	nested := mustAddMilestoneTask(t, c, m.ID, "nested")
	loose := mustAddTask(t, c, p.ID, "loose")

	if _, err := c.AddMilestoneTaskAfter(ctx, 9090, nested.ID, core.TaskInput{Title: "x"}); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("unknown milestone: error = %v, want ErrMilestoneNotFound", err)
	}
	if _, err := c.AddMilestoneTaskAfter(ctx, 9090, 0, core.TaskInput{Title: "x"}); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("unknown milestone (front): error = %v, want ErrMilestoneNotFound", err)
	}
	if _, err := c.AddMilestoneTaskAfter(ctx, m.ID, nested.ID, core.TaskInput{Title: " "}); !errors.Is(err, core.ErrEmptyTaskTitle) {
		t.Errorf("blank title: error = %v, want ErrEmptyTaskTitle", err)
	}
	if _, err := c.AddMilestoneTaskAfter(ctx, m.ID, 7777, core.TaskInput{Title: "x"}); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown afterTaskID: error = %v, want ErrTaskNotFound", err)
	}
	// A loose Task is not a Task of this Milestone.
	if _, err := c.AddMilestoneTaskAfter(ctx, m.ID, loose.ID, core.TaskInput{Title: "x"}); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("loose task as anchor: error = %v, want ErrTaskNotFound", err)
	}
}

func TestMilestoneTasksUnknownMilestoneErrors(t *testing.T) {
	c, _ := newTestCore(t)

	if _, err := c.MilestoneTasks(context.Background(), 4242); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("error = %v, want ErrMilestoneNotFound", err)
	}
}

func TestMoveMilestoneTaskReordersWithinTheMilestone(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Reorder", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	mustAddMilestoneTask(t, c, m.ID, "one")
	two := mustAddMilestoneTask(t, c, m.ID, "two")
	mustAddMilestoneTask(t, c, m.ID, "three")

	got, err := c.MoveMilestoneTask(ctx, two.ID, core.MoveUp)
	if err != nil {
		t.Fatalf("MoveMilestoneTask(up): %v", err)
	}
	if !equalStrings(taskTitles(got), []string{"two", "one", "three"}) {
		t.Errorf("returned order = %v, want two moved ahead of one", taskTitles(got))
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"two", "one", "three"}) {
		t.Errorf("reloaded order = %v, want the move to persist", got)
	}

	// Down past "three".
	if _, err := c.MoveMilestoneTask(ctx, two.ID, core.MoveDown); err != nil {
		t.Fatalf("MoveMilestoneTask(down): %v", err)
	}
	if _, err := c.MoveMilestoneTask(ctx, two.ID, core.MoveDown); err != nil {
		t.Fatalf("MoveMilestoneTask(down): %v", err)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"one", "three", "two"}) {
		t.Errorf("order = %v, want two at the end", got)
	}
	// Down again from the bottom edge is a no-op.
	if _, err := c.MoveMilestoneTask(ctx, two.ID, core.MoveDown); err != nil {
		t.Fatalf("MoveMilestoneTask(down) at edge: %v", err)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"one", "three", "two"}) {
		t.Errorf("order = %v, want it unchanged past the bottom edge", got)
	}
}

func TestMoveMilestoneTaskErrorCases(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	inside := mustAddMilestoneTask(t, c, m.ID, "inside")
	loose := mustAddTask(t, c, p.ID, "loose")

	if _, err := c.MoveMilestoneTask(ctx, inside.ID, 99); !errors.Is(err, core.ErrInvalidMove) {
		t.Errorf("bad direction: error = %v, want ErrInvalidMove", err)
	}
	if _, err := c.MoveMilestoneTask(ctx, 7070, core.MoveUp); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown id: error = %v, want ErrTaskNotFound", err)
	}
	if _, err := c.MoveMilestoneTask(ctx, loose.ID, core.MoveUp); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("loose task: error = %v, want ErrTaskNotFound (no Milestone scope)", err)
	}
}

// A Milestone reordered in the body carries its Tasks, in their order, with it.
func TestReorderingMilestoneKeepsItsTasks(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Travel", categoryID(t, c, "Programming"))

	mustAddTask(t, c, p.ID, "loose first")
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	mustAddMilestoneTask(t, c, m.ID, "a1")
	mustAddMilestoneTask(t, c, m.ID, "a2")
	mustAddTask(t, c, p.ID, "loose last") // body: loose first, Alpha[a1,a2], loose last

	if _, err := c.MoveMilestone(ctx, m.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveMilestone(up): %v", err)
	}

	body, err := c.ProjectBody(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectBody: %v", err)
	}
	if !equalStrings(bodyLabels(body), []string{"ms:Alpha", "task:loose first", "task:loose last"}) {
		t.Fatalf("body = %v, want Alpha moved to the front", bodyLabels(body))
	}
	if body[0].Kind != core.MilestoneEntry {
		t.Fatalf("body[0] kind = %v, want the Milestone", body[0].Kind)
	}
	if got := taskTitles(body[0].Milestone.Tasks); !equalStrings(got, []string{"a1", "a2"}) {
		t.Errorf("milestone tasks after the move = %v, want them intact and in order", got)
	}
}

func TestNextStepWalksTheMixedBody(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Walk", categoryID(t, c, "Programming"))

	loose := mustAddTask(t, c, p.ID, "loose lead")
	m1 := mustAddMilestone(t, c, p.ID, "Alpha")
	a1 := mustAddMilestoneTask(t, c, m1.ID, "a1")
	a2 := mustAddMilestoneTask(t, c, m1.ID, "a2")
	m2 := mustAddMilestone(t, c, p.ID, "Beta")
	b1 := mustAddMilestoneTask(t, c, m2.ID, "b1")
	// body: loose lead, Alpha[a1,a2], Beta[b1]

	// A not-done loose Task before the first Milestone is the Next step.
	assertNextStep(t, c, p.ID, loose.ID, "loose lead")

	// Complete the loose Task: the first Milestone is now the first incomplete
	// entry, so its first incomplete Task is the Next step.
	mustSetDone(t, c, loose.ID)
	assertNextStep(t, c, p.ID, a1.ID, "a1")

	// Complete a1: Next step advances within the Milestone.
	mustSetDone(t, c, a1.ID)
	assertNextStep(t, c, p.ID, a2.ID, "a2")

	// Complete a2: Alpha has no incomplete Task, so it is skipped and Beta's
	// first incomplete Task is the Next step.
	mustSetDone(t, c, a2.ID)
	assertNextStep(t, c, p.ID, b1.ID, "b1")

	// Complete b1: every entry is done/empty, so there is no Next step.
	mustSetDone(t, c, b1.ID)
	if _, ok, err := c.NextStep(ctx, p.ID); err != nil || ok {
		t.Errorf("NextStep with everything done = (ok %v, err %v), want none", ok, err)
	}
}

// An empty Milestone and an all-done Milestone are both skipped, and a Project
// made only of those reports no Next step.
func TestNextStepSkipsEmptyAndAllDoneMilestones(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Skips", categoryID(t, c, "Programming"))

	empty := mustAddMilestone(t, c, p.ID, "Empty")
	done := mustAddMilestone(t, c, p.ID, "Done")
	d1 := mustAddMilestoneTask(t, c, done.ID, "d1")
	live := mustAddMilestone(t, c, p.ID, "Live")
	l1 := mustAddMilestoneTask(t, c, live.ID, "l1")
	_ = empty

	mustSetDone(t, c, d1.ID)
	assertNextStep(t, c, p.ID, l1.ID, "l1")

	// Now finish the live Milestone too: nothing incomplete anywhere.
	mustSetDone(t, c, l1.ID)
	if _, ok, err := c.NextStep(ctx, p.ID); err != nil || ok {
		t.Errorf("NextStep = (ok %v, err %v), want none once every Milestone is empty or done", ok, err)
	}
}

// The dashboard Next step for an Active Project reflects Milestone contents.
func TestDashboardNextStepReflectsMilestones(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Board", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	inside := mustAddMilestoneTask(t, c, m.ID, "inside step")

	rows, err := c.Dashboard(ctx)
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if len(rows) != 1 || rows[0].NextStep == nil || rows[0].NextStep.ID != inside.ID {
		t.Fatalf("dashboard rows = %+v, want the Milestone's Task as the Next step", rows)
	}

	mustSetDone(t, c, inside.ID)
	rows, err = c.Dashboard(ctx)
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if len(rows) != 1 || rows[0].NextStep != nil {
		t.Errorf("dashboard Next step = %+v, want none once the Milestone Task is done", rows[0].NextStep)
	}
}

// assertNextStep fails unless the Project's Next step is the given Task.
func assertNextStep(t *testing.T, c *core.Core, projectID, wantID int64, wantTitle string) {
	t.Helper()
	step, ok, err := c.NextStep(context.Background(), projectID)
	if err != nil || !ok {
		t.Fatalf("NextStep = (%+v, %v, %v), want %q", step, ok, err, wantTitle)
	}
	if step.ID != wantID {
		t.Errorf("NextStep = %q (id %d), want %q (id %d)", step.Title, step.ID, wantTitle, wantID)
	}
}

// mustSetDone marks a Task done through core and fails the test on error.
func mustSetDone(t *testing.T, c *core.Core, id int64) {
	t.Helper()
	if _, err := c.SetTaskDone(context.Background(), id, true); err != nil {
		t.Fatalf("SetTaskDone(%d): %v", id, err)
	}
}

// looseTaskByTitle returns a Project's loose Task with the given title.
func looseTaskByTitle(t *testing.T, c *core.Core, projectID int64, title string) core.Task {
	t.Helper()
	tasks, err := c.ProjectTasks(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	for _, tk := range tasks {
		if tk.Title == title {
			return tk
		}
	}
	t.Fatalf("loose task %q not found in project %d", title, projectID)
	return core.Task{}
}
