package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// bodyTaskTitles is the loose-Task titles from a body slice, in order, skipping
// any Milestone slots.
func bodyTaskTitles(body []core.BodyEntry) []string {
	var out []string
	for _, e := range body {
		if e.Kind == core.TaskEntry {
			out = append(out, e.Task.Title)
		}
	}
	return out
}

// bodyLabels renders a body slice as "task:<title>" / "ms:<name>" entries, so a
// test can assert the interleaved order.
func bodyLabels(body []core.BodyEntry) []string {
	out := make([]string, len(body))
	for i, e := range body {
		if e.Kind == core.MilestoneEntry {
			out[i] = "ms:" + e.Milestone.Name
		} else {
			out[i] = "task:" + e.Task.Title
		}
	}
	return out
}

// seedBody creates a Project with three loose Tasks in a known order and
// returns the Project plus the three Tasks.
func seedBody(t *testing.T, c *core.Core) (core.Project, core.Task, core.Task, core.Task) {
	t.Helper()
	p := mustCreateProject(t, c, "Re-plan me", categoryID(t, c, "Programming"))
	a := mustAddTask(t, c, p.ID, "first")
	b := mustAddTask(t, c, p.ID, "second")
	d := mustAddTask(t, c, p.ID, "third")
	return p, a, b, d
}

func TestMoveTaskUpSwapsWithThePrecedingEntry(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p, _, second, _ := seedBody(t, c)

	got, err := c.MoveTask(ctx, second.ID, core.MoveUp)
	if err != nil {
		t.Fatalf("MoveTask(up): %v", err)
	}
	if !equalStrings(bodyTaskTitles(got), []string{"second", "first", "third"}) {
		t.Errorf("returned order = %v, want second moved ahead of first", bodyTaskTitles(got))
	}

	// The new order is the stored order, not a transient sort.
	reloaded, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if !equalStrings(taskTitles(reloaded), []string{"second", "first", "third"}) {
		t.Errorf("reloaded order = %v, want the move to persist", taskTitles(reloaded))
	}
}

func TestMoveTaskDownSwapsWithTheFollowingEntry(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p, _, second, _ := seedBody(t, c)

	if _, err := c.MoveTask(ctx, second.ID, core.MoveDown); err != nil {
		t.Fatalf("MoveTask(down): %v", err)
	}

	reloaded, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if !equalStrings(taskTitles(reloaded), []string{"first", "third", "second"}) {
		t.Errorf("reloaded order = %v, want second moved after third", taskTitles(reloaded))
	}
}

func TestMoveTaskUpFromTheFirstSlotIsANoOp(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p, first, _, _ := seedBody(t, c)

	got, err := c.MoveTask(ctx, first.ID, core.MoveUp)
	if err != nil {
		t.Fatalf("MoveTask(up) at the top: %v", err)
	}
	if !equalStrings(bodyTaskTitles(got), []string{"first", "second", "third"}) {
		t.Errorf("order = %v, want it unchanged past the top edge", bodyTaskTitles(got))
	}

	reloaded, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if !equalStrings(taskTitles(reloaded), []string{"first", "second", "third"}) {
		t.Errorf("reloaded order = %v, want it unchanged", taskTitles(reloaded))
	}
}

func TestMoveTaskDownFromTheLastSlotIsANoOp(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p, _, _, third := seedBody(t, c)

	got, err := c.MoveTask(ctx, third.ID, core.MoveDown)
	if err != nil {
		t.Fatalf("MoveTask(down) at the bottom: %v", err)
	}
	if !equalStrings(bodyTaskTitles(got), []string{"first", "second", "third"}) {
		t.Errorf("order = %v, want it unchanged past the bottom edge", bodyTaskTitles(got))
	}

	reloaded, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if !equalStrings(taskTitles(reloaded), []string{"first", "second", "third"}) {
		t.Errorf("reloaded order = %v, want it unchanged", taskTitles(reloaded))
	}
}

func TestMoveTaskChangesTheNextStep(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p, first, second, _ := seedBody(t, c)

	// With every Task open, Next step is the first in body order.
	step, ok, err := c.NextStep(ctx, p.ID)
	if err != nil || !ok || step.ID != first.ID {
		t.Fatalf("NextStep = (%+v, %v, %v), want %q", step, ok, err, "first")
	}

	// Move the second Task above the first; Next step follows the new order.
	if _, err := c.MoveTask(ctx, second.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveTask(up): %v", err)
	}
	step, ok, err = c.NextStep(ctx, p.ID)
	if err != nil || !ok {
		t.Fatalf("NextStep after the move = (%+v, %v, %v)", step, ok, err)
	}
	if step.ID != second.ID {
		t.Errorf("NextStep = %q, want %q now that it is first", step.Title, "second")
	}
}

func TestMoveTaskUnknownIDErrors(t *testing.T) {
	c, _ := newTestCore(t)

	if _, err := c.MoveTask(context.Background(), 8080, core.MoveUp); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("error = %v, want ErrTaskNotFound", err)
	}
}
