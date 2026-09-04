package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestMoveTaskToMilestoneAppendsAtTheEndAndClosesTheBodyGap(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Cross", categoryID(t, c, "Programming"))

	mustAddTask(t, c, p.ID, "first")
	middle := mustAddTask(t, c, p.ID, "middle")
	mustAddTask(t, c, p.ID, "last")
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	mustAddMilestoneTask(t, c, m.ID, "a1")
	// body: first, middle, last, Alpha[a1]

	moved, err := c.MoveTaskToMilestone(ctx, middle.ID, m.ID)
	if err != nil {
		t.Fatalf("MoveTaskToMilestone: %v", err)
	}
	if moved.MilestoneID == nil || *moved.MilestoneID != m.ID {
		t.Errorf("MilestoneID = %v, want %d", moved.MilestoneID, m.ID)
	}

	// It leaves the Project body...
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:first", "task:last", "ms:Alpha"}) {
		t.Errorf("body = %v, want middle gone and the gap closed", got)
	}
	// ...and lands last in the Milestone's own order.
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"a1", "middle"}) {
		t.Errorf("milestone tasks = %v, want middle appended at the end", got)
	}

	// A subsequent loose Task still lands at the end of the body, not clashing
	// with the vacated position.
	mustAddTask(t, c, p.ID, "tail")
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:first", "task:last", "ms:Alpha", "task:tail"}) {
		t.Errorf("body = %v, want tail appended cleanly after the move", got)
	}
}

func TestMoveTaskToMilestoneIntoAnEmptyMilestone(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Cross empty", categoryID(t, c, "Programming"))
	task := mustAddTask(t, c, p.ID, "loose")
	m := mustAddMilestone(t, c, p.ID, "Empty")

	if _, err := c.MoveTaskToMilestone(ctx, task.ID, m.ID); err != nil {
		t.Fatalf("MoveTaskToMilestone: %v", err)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"loose"}) {
		t.Errorf("milestone tasks = %v, want the Task alone", got)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"ms:Empty"}) {
		t.Errorf("body = %v, want only the Milestone left", got)
	}
}

func TestMoveTaskToMilestoneAfterLandsAtTheChosenSpot(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Cross at a spot", categoryID(t, c, "Programming"))
	loose := mustAddTask(t, c, p.ID, "loose")
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	a1 := mustAddMilestoneTask(t, c, m.ID, "a1")
	mustAddMilestoneTask(t, c, m.ID, "a2")

	if _, err := c.MoveTaskToMilestoneAfter(ctx, loose.ID, m.ID, a1.ID); err != nil {
		t.Fatalf("MoveTaskToMilestoneAfter: %v", err)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"a1", "loose", "a2"}) {
		t.Errorf("milestone tasks = %v, want loose landed right after a1", got)
	}

	// afterTaskID 0 lands it at the front.
	tail := mustAddTask(t, c, p.ID, "tail")
	if _, err := c.MoveTaskToMilestoneAfter(ctx, tail.ID, m.ID, 0); err != nil {
		t.Fatalf("MoveTaskToMilestoneAfter(front): %v", err)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"tail", "a1", "loose", "a2"}) {
		t.Errorf("milestone tasks = %v, want tail first", got)
	}
}

func TestMoveTaskToBodyAppendsAtTheEndAndClosesTheMilestoneGap(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Cross back", categoryID(t, c, "Programming"))
	mustAddTask(t, c, p.ID, "lead")
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	mustAddMilestoneTask(t, c, m.ID, "a1")
	a2 := mustAddMilestoneTask(t, c, m.ID, "a2")
	a3 := mustAddMilestoneTask(t, c, m.ID, "a3")
	// body: lead, Alpha[a1, a2, a3]

	moved, err := c.MoveTaskToBody(ctx, a2.ID)
	if err != nil {
		t.Fatalf("MoveTaskToBody: %v", err)
	}
	if moved.MilestoneID != nil {
		t.Errorf("MilestoneID = %v, want nil (now loose)", moved.MilestoneID)
	}

	// The Milestone closes the gap a2 left...
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"a1", "a3"}) {
		t.Errorf("milestone tasks = %v, want a2 gone and the gap closed", got)
	}
	// ...and a2 lands last in the Project body.
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:lead", "ms:Alpha", "task:a2"}) {
		t.Errorf("body = %v, want a2 appended at the end", got)
	}

	// The Milestone's remaining order still re-sequences cleanly afterward.
	if _, err := c.MoveMilestoneTask(ctx, a3.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveMilestoneTask: %v", err)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"a3", "a1"}) {
		t.Errorf("milestone tasks = %v, want a3 and a1 swapped", got)
	}
}

func TestMoveTaskToBodyAfterLandsAtTheChosenSpot(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Cross back at a spot", categoryID(t, c, "Programming"))
	first := mustAddTask(t, c, p.ID, "first")
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	a1 := mustAddMilestoneTask(t, c, m.ID, "a1")
	a2 := mustAddMilestoneTask(t, c, m.ID, "a2")
	mustAddTask(t, c, p.ID, "last")
	// body: first, Alpha[a1, a2], last

	after := core.BodyRef{Kind: core.TaskEntry, ID: first.ID}
	if _, err := c.MoveTaskToBodyAfter(ctx, a1.ID, after); err != nil {
		t.Fatalf("MoveTaskToBodyAfter: %v", err)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:first", "task:a1", "ms:Alpha", "task:last"}) {
		t.Errorf("body = %v, want a1 landed right after first", got)
	}

	// A zero anchor lands it at the front.
	if _, err := c.MoveTaskToBodyAfter(ctx, a2.ID, core.BodyRef{}); err != nil {
		t.Fatalf("MoveTaskToBodyAfter(front): %v", err)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:a2", "task:first", "task:a1", "ms:Alpha", "task:last"}) {
		t.Errorf("body = %v, want a2 first", got)
	}
}

func TestMoveTaskCrossLevelReflectsInNextStep(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Next step follows", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	inside := mustAddMilestoneTask(t, c, m.ID, "inside")
	loose := mustAddTask(t, c, p.ID, "loose")
	// body: Alpha[inside], loose — Next step is "inside".
	assertNextStep(t, c, p.ID, inside.ID, "inside")

	// Move "inside" out to the body: Alpha is now empty and skipped, so
	// "loose" becomes the Next step.
	if _, err := c.MoveTaskToBody(ctx, inside.ID); err != nil {
		t.Fatalf("MoveTaskToBody: %v", err)
	}
	assertNextStep(t, c, p.ID, loose.ID, "loose")

	// Move "loose" into the (now empty) Milestone: Alpha becomes the first
	// incomplete entry, with "loose" as its only Task.
	if _, err := c.MoveTaskToMilestone(ctx, loose.ID, m.ID); err != nil {
		t.Fatalf("MoveTaskToMilestone: %v", err)
	}
	assertNextStep(t, c, p.ID, loose.ID, "loose")
}

func TestMoveTaskToMilestoneErrorCases(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p1 := mustCreateProject(t, c, "P1", categoryID(t, c, "Programming"))
	p2 := mustCreateProject(t, c, "P2", categoryID(t, c, "Other"))
	loose := mustAddTask(t, c, p1.ID, "loose")
	m1 := mustAddMilestone(t, c, p1.ID, "Alpha")
	nested := mustAddMilestoneTask(t, c, m1.ID, "nested")
	m2 := mustAddMilestone(t, c, p2.ID, "Beta")

	if _, err := c.MoveTaskToMilestone(ctx, 7777, m1.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown task: error = %v, want ErrTaskNotFound", err)
	}
	if _, err := c.MoveTaskToMilestone(ctx, loose.ID, 8888); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("unknown milestone: error = %v, want ErrMilestoneNotFound", err)
	}
	// Already inside a Milestone — not a loose Task to cross.
	if _, err := c.MoveTaskToMilestone(ctx, nested.ID, m1.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("already-nested task: error = %v, want ErrTaskNotFound", err)
	}
	// A Milestone from a different Project is not a valid destination.
	if _, err := c.MoveTaskToMilestone(ctx, loose.ID, m2.ID); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("cross-project milestone: error = %v, want ErrMilestoneNotFound", err)
	}
	// An unknown afterTaskID on the After variant.
	if _, err := c.MoveTaskToMilestoneAfter(ctx, loose.ID, m1.ID, 9999); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown afterTaskID: error = %v, want ErrTaskNotFound", err)
	}
}

func TestMoveTaskToBodyErrorCases(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Programming"))
	loose := mustAddTask(t, c, p.ID, "loose")

	if _, err := c.MoveTaskToBody(ctx, 7777); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown task: error = %v, want ErrTaskNotFound", err)
	}
	// Already loose — nothing to cross.
	if _, err := c.MoveTaskToBody(ctx, loose.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("already-loose task: error = %v, want ErrTaskNotFound", err)
	}
	// An unknown anchor on the After variant.
	if _, err := c.MoveTaskToBodyAfter(ctx, loose.ID, core.BodyRef{Kind: core.TaskEntry, ID: 9999}); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown anchor: error = %v, want ErrTaskNotFound", err)
	}
}

// A plain within-level reorder never crosses the loose/Milestone boundary —
// MoveTask only swaps top-level slots, and MoveMilestoneTask only swaps within
// one Milestone's own list.
func TestWithinLevelMovesNeverCrossTheBoundary(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Boundary", categoryID(t, c, "Programming"))
	loose := mustAddTask(t, c, p.ID, "loose")
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	a1 := mustAddMilestoneTask(t, c, m.ID, "a1")
	mustAddMilestoneTask(t, c, m.ID, "a2")
	// body: loose, Alpha[a1, a2]

	// Moving the loose Task down swaps it with the Milestone as a top-level
	// slot — it does not enter the Milestone's Task list.
	if _, err := c.MoveTask(ctx, loose.ID, core.MoveDown); err != nil {
		t.Fatalf("MoveTask(down): %v", err)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"ms:Alpha", "task:loose"}) {
		t.Errorf("body = %v, want loose swapped past the Milestone, not into it", got)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"a1", "a2"}) {
		t.Errorf("milestone tasks = %v, want unchanged", got)
	}

	// Moving a1 past the Milestone's last Task only swaps it with a2 — it
	// stays inside the Milestone rather than exiting to the body.
	if _, err := c.MoveMilestoneTask(ctx, a1.ID, core.MoveDown); err != nil {
		t.Fatalf("MoveMilestoneTask(down): %v", err)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !equalStrings(got, []string{"a2", "a1"}) {
		t.Errorf("milestone tasks = %v, want a1 and a2 swapped", got)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"ms:Alpha", "task:loose"}) {
		t.Errorf("body = %v, want it unchanged — a1 stayed inside the Milestone", got)
	}
}
