package core_test

import (
	"context"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// milestoneByID finds a Milestone in a Project's body by id, for reading its
// CompletionAcked flag — GetMilestone is not exported, and ProjectBody is the
// read path that fills it in.
func milestoneByID(t *testing.T, c *core.Core, projectID, milestoneID int64) core.Milestone {
	t.Helper()
	body, err := c.ProjectBody(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ProjectBody: %v", err)
	}
	for _, e := range body {
		if e.Kind == core.MilestoneEntry && e.Milestone.ID == milestoneID {
			return *e.Milestone
		}
	}
	t.Fatalf("milestone %d not found in project %d body", milestoneID, projectID)
	return core.Milestone{}
}

// Completing every Task in a Milestone but one reports no signal; completing
// the last one does, exactly once — a second, unrelated completion elsewhere
// in the Milestone (there is none left) never re-fires it on its own.
func TestSetTaskDoneSignalsOnlyWhenTheLastTaskCompletes(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Ship it", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	first := mustAddMilestoneTask(t, c, m.ID, "first")
	last := mustAddMilestoneTask(t, c, m.ID, "last")

	_, signal, err := c.SetTaskDone(ctx, first.ID, true)
	if err != nil {
		t.Fatalf("SetTaskDone(first): %v", err)
	}
	if signal != nil {
		t.Errorf("signal = %+v, want nil — the Milestone still has an incomplete Task", signal)
	}

	_, signal, err = c.SetTaskDone(ctx, last.ID, true)
	if err != nil {
		t.Fatalf("SetTaskDone(last): %v", err)
	}
	if signal == nil || signal.ID != m.ID {
		t.Fatalf("signal = %+v, want the completed Milestone %d", signal, m.ID)
	}
	if signal.CompletionAcked {
		t.Errorf("signal.CompletionAcked = true, want false before any acknowledgement")
	}
}

// A loose Task's completion never signals — it has no Milestone to complete.
func TestSetTaskDoneOnLooseTaskNeverSignals(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Loose", categoryID(t, c, "Programming"))
	task := mustAddTask(t, c, p.ID, "solo")

	_, signal, err := c.SetTaskDone(ctx, task.ID, true)
	if err != nil {
		t.Fatalf("SetTaskDone: %v", err)
	}
	if signal != nil {
		t.Errorf("signal = %+v, want nil for a loose Task", signal)
	}
}

// AckMilestoneComplete suppresses the signal until a later change reopens it —
// un-completing the Task that finished the Milestone clears the
// acknowledgement, and re-completing it signals again.
func TestAckMilestoneCompleteSuppressesUntilAChangeClearsIt(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Ack", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	only := mustAddMilestoneTask(t, c, m.ID, "only")

	_, signal, err := c.SetTaskDone(ctx, only.ID, true)
	if err != nil {
		t.Fatalf("SetTaskDone: %v", err)
	}
	if signal == nil {
		t.Fatal("signal = nil, want the completed Milestone")
	}

	if _, err := c.AckMilestoneComplete(ctx, m.ID); err != nil {
		t.Fatalf("AckMilestoneComplete: %v", err)
	}
	if got := milestoneByID(t, c, p.ID, m.ID); !got.CompletionAcked {
		t.Errorf("CompletionAcked = false after AckMilestoneComplete, want true")
	}

	// Un-completing clears the acknowledgement...
	if _, signal, err = c.SetTaskDone(ctx, only.ID, false); err != nil {
		t.Fatalf("SetTaskDone(false): %v", err)
	}
	if signal != nil {
		t.Errorf("signal = %+v, want nil for un-completing", signal)
	}
	if got := milestoneByID(t, c, p.ID, m.ID); got.CompletionAcked {
		t.Errorf("CompletionAcked = true after un-completing, want cleared")
	}

	// ...so re-completing signals again.
	if _, signal, err = c.SetTaskDone(ctx, only.ID, true); err != nil {
		t.Fatalf("SetTaskDone(true again): %v", err)
	}
	if signal == nil || signal.ID != m.ID {
		t.Fatalf("signal = %+v, want the Milestone to signal again after the change", signal)
	}
}

// Adding a Task to an already-acknowledged, complete Milestone clears the
// acknowledgement, whether through AddMilestoneTask or a cross-level move.
func TestAddingATaskClearsAnExistingAcknowledgement(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Grow", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	only := mustAddMilestoneTask(t, c, m.ID, "only")

	if _, _, err := c.SetTaskDone(ctx, only.ID, true); err != nil {
		t.Fatalf("SetTaskDone: %v", err)
	}
	if _, err := c.AckMilestoneComplete(ctx, m.ID); err != nil {
		t.Fatalf("AckMilestoneComplete: %v", err)
	}

	if _, err := c.AddMilestoneTask(ctx, m.ID, core.TaskInput{Title: "another"}); err != nil {
		t.Fatalf("AddMilestoneTask: %v", err)
	}
	if got := milestoneByID(t, c, p.ID, m.ID); got.CompletionAcked {
		t.Errorf("CompletionAcked = true after adding a Task, want cleared")
	}
}

func TestMoveTaskToMilestoneClearsAnExistingAcknowledgement(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Grow", categoryID(t, c, "Programming"))
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	only := mustAddMilestoneTask(t, c, m.ID, "only")
	loose := mustAddTask(t, c, p.ID, "loose")

	if _, _, err := c.SetTaskDone(ctx, only.ID, true); err != nil {
		t.Fatalf("SetTaskDone: %v", err)
	}
	if _, err := c.AckMilestoneComplete(ctx, m.ID); err != nil {
		t.Fatalf("AckMilestoneComplete: %v", err)
	}

	if _, err := c.MoveTaskToMilestone(ctx, loose.ID, m.ID); err != nil {
		t.Fatalf("MoveTaskToMilestone: %v", err)
	}
	if got := milestoneByID(t, c, p.ID, m.ID); got.CompletionAcked {
		t.Errorf("CompletionAcked = true after moving a Task in, want cleared")
	}
}
