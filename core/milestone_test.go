package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// mustAddMilestone adds a Milestone through core and fails the test on error.
func mustAddMilestone(t *testing.T, c *core.Core, projectID int64, name string) core.Milestone {
	t.Helper()
	m, err := c.AddMilestone(context.Background(), projectID, name)
	if err != nil {
		t.Fatalf("AddMilestone(%q): %v", name, err)
	}
	return m
}

// projectBodyLabels is the Project body rendered as ordered "task:" / "ms:"
// labels.
func projectBodyLabels(t *testing.T, c *core.Core, projectID int64) []string {
	t.Helper()
	body, err := c.ProjectBody(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ProjectBody: %v", err)
	}
	return bodyLabels(body)
}

func TestAddMilestoneAppendsAsOneBodyEntry(t *testing.T) {
	c, _ := newTestCore(t)
	p := mustCreateProject(t, c, "Ship it", categoryID(t, c, "Programming"))

	mustAddTask(t, c, p.ID, "loose one")
	m := mustAddMilestone(t, c, p.ID, "Alpha release")

	if m.ProjectID != p.ID || m.Name != "Alpha release" || m.ID == 0 {
		t.Fatalf("milestone = %+v, want it stored under project %d", m, p.ID)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:loose one", "ms:Alpha release"}) {
		t.Errorf("body = %v, want the Milestone as one trailing entry", got)
	}
}

func TestAddMilestoneTrimsAndRejectsEmptyName(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))

	m, err := c.AddMilestone(ctx, p.ID, "  Design  ")
	if err != nil {
		t.Fatalf("AddMilestone with padding: %v", err)
	}
	if m.Name != "Design" {
		t.Errorf("name = %q, want it trimmed", m.Name)
	}
	if _, err := c.AddMilestone(ctx, p.ID, "   "); !errors.Is(err, core.ErrEmptyMilestoneName) {
		t.Errorf("error = %v, want ErrEmptyMilestoneName", err)
	}
}

func TestAddMilestoneUnknownProjectErrors(t *testing.T) {
	c, _ := newTestCore(t)

	if _, err := c.AddMilestone(context.Background(), 9090, "Alpha"); !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("error = %v, want ErrProjectNotFound", err)
	}
}

func TestProjectBodyInterleavesTasksAndMilestones(t *testing.T) {
	c, _ := newTestCore(t)
	p := mustCreateProject(t, c, "Interleave", categoryID(t, c, "Programming"))

	// Add in a deliberately mixed order: Task, Milestone, Task, Milestone.
	mustAddTask(t, c, p.ID, "setup")
	mustAddMilestone(t, c, p.ID, "Alpha")
	mustAddTask(t, c, p.ID, "cleanup")
	mustAddMilestone(t, c, p.ID, "Beta")

	got := projectBodyLabels(t, c, p.ID)
	want := []string{"task:setup", "ms:Alpha", "task:cleanup", "ms:Beta"}
	if !equalStrings(got, want) {
		t.Errorf("body = %v, want it in the stored insertion order %v", got, want)
	}
}

func TestLooseTaskMovesBeforeBetweenAndAfterMilestones(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Weave", categoryID(t, c, "Programming"))

	mustAddMilestone(t, c, p.ID, "Alpha")
	mustAddMilestone(t, c, p.ID, "Beta")
	tk := mustAddTask(t, c, p.ID, "weaver") // body: Alpha, Beta, weaver

	// After Milestones (as added).
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"ms:Alpha", "ms:Beta", "task:weaver"}) {
		t.Fatalf("body = %v, want the Task last", got)
	}

	// Between the two Milestones. MoveTask returns the reordered body directly.
	returned, err := c.MoveTask(ctx, tk.ID, core.MoveUp)
	if err != nil {
		t.Fatalf("MoveTask(up): %v", err)
	}
	if !equalStrings(bodyLabels(returned), []string{"ms:Alpha", "task:weaver", "ms:Beta"}) {
		t.Errorf("returned body = %v, want the moved Task reflected without a reload", bodyLabels(returned))
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"ms:Alpha", "task:weaver", "ms:Beta"}) {
		t.Fatalf("body = %v, want the Task between the Milestones", got)
	}

	// Before both Milestones.
	if _, err := c.MoveTask(ctx, tk.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveTask(up): %v", err)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:weaver", "ms:Alpha", "ms:Beta"}) {
		t.Fatalf("body = %v, want the Task first", got)
	}

	// Up again from the top edge is a no-op.
	if _, err := c.MoveTask(ctx, tk.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveTask(up) at edge: %v", err)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:weaver", "ms:Alpha", "ms:Beta"}) {
		t.Errorf("body = %v, want it unchanged past the top edge", got)
	}
}

func TestMoveMilestoneReordersWithinTheBody(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Reorder", categoryID(t, c, "Programming"))

	mustAddTask(t, c, p.ID, "first")
	beta := mustAddMilestone(t, c, p.ID, "Beta")
	mustAddTask(t, c, p.ID, "last") // body: first, Beta, last

	body, err := c.MoveMilestone(ctx, beta.ID, core.MoveUp)
	if err != nil {
		t.Fatalf("MoveMilestone(up): %v", err)
	}
	if !equalStrings(bodyLabels(body), []string{"ms:Beta", "task:first", "task:last"}) {
		t.Errorf("returned body = %v, want Beta moved ahead of first", bodyLabels(body))
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"ms:Beta", "task:first", "task:last"}) {
		t.Errorf("reloaded body = %v, want the move to persist", got)
	}

	// Move it back down past "first".
	if _, err := c.MoveMilestone(ctx, beta.ID, core.MoveDown); err != nil {
		t.Fatalf("MoveMilestone(down): %v", err)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"task:first", "ms:Beta", "task:last"}) {
		t.Errorf("body = %v, want Beta back between the Tasks", got)
	}
}

func TestMoveMilestoneEdgeAndErrorCases(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Edges", categoryID(t, c, "Programming"))
	alpha := mustAddMilestone(t, c, p.ID, "Alpha")
	mustAddTask(t, c, p.ID, "tail")

	if _, err := c.MoveMilestone(ctx, alpha.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveMilestone(up) at top edge: %v", err)
	}
	if got := projectBodyLabels(t, c, p.ID); !equalStrings(got, []string{"ms:Alpha", "task:tail"}) {
		t.Errorf("body = %v, want it unchanged at the top edge", got)
	}

	if _, err := c.MoveMilestone(ctx, alpha.ID, 42); !errors.Is(err, core.ErrInvalidMove) {
		t.Errorf("error = %v, want ErrInvalidMove for a bad direction", err)
	}
	if _, err := c.MoveMilestone(ctx, 7070, core.MoveUp); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("error = %v, want ErrMilestoneNotFound for an unknown id", err)
	}
}

// A move touches only the Project that owns the entry; a sibling Project's body
// keeps its order.
func TestMoveMilestoneLeavesOtherProjectsUntouched(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	prog := categoryID(t, c, "Programming")

	p1 := mustCreateProject(t, c, "One", prog)
	mustAddTask(t, c, p1.ID, "p1 first")
	m1 := mustAddMilestone(t, c, p1.ID, "P1 Gate")

	p2 := mustCreateProject(t, c, "Two", prog)
	mustAddTask(t, c, p2.ID, "p2 first")
	mustAddMilestone(t, c, p2.ID, "P2 Gate")

	if _, err := c.MoveMilestone(ctx, m1.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveMilestone(up): %v", err)
	}

	if got := projectBodyLabels(t, c, p1.ID); !equalStrings(got, []string{"ms:P1 Gate", "task:p1 first"}) {
		t.Errorf("p1 body = %v, want its Milestone moved up", got)
	}
	if got := projectBodyLabels(t, c, p2.ID); !equalStrings(got, []string{"task:p2 first", "ms:P2 Gate"}) {
		t.Errorf("p2 body = %v, want it untouched by the move in p1", got)
	}
}

func TestEmptyMilestoneDoesNotChangeNextStep(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Next", categoryID(t, c, "Programming"))

	first := mustAddTask(t, c, p.ID, "first")
	second := mustAddTask(t, c, p.ID, "second")

	step, ok, err := c.NextStep(ctx, p.ID)
	if err != nil || !ok || step.ID != first.ID {
		t.Fatalf("baseline NextStep = (%+v, %v, %v), want %q", step, ok, err, "first")
	}

	// Drop an empty Milestone at the front and between the Tasks.
	m := mustAddMilestone(t, c, p.ID, "Alpha")
	if _, err := c.MoveMilestone(ctx, m.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveMilestone(up): %v", err)
	}
	if _, err := c.MoveMilestone(ctx, m.ID, core.MoveUp); err != nil {
		t.Fatalf("MoveMilestone(up): %v", err)
	}
	// body: Alpha, first, second

	step, ok, err = c.NextStep(ctx, p.ID)
	if err != nil || !ok || step.ID != first.ID {
		t.Errorf("NextStep with a leading empty Milestone = (%+v, %v, %v), want %q", step, ok, err, "first")
	}

	// Complete "first"; the empty Milestone still contributes nothing.
	if _, err := c.SetTaskDone(ctx, first.ID, true); err != nil {
		t.Fatalf("SetTaskDone: %v", err)
	}
	step, ok, err = c.NextStep(ctx, p.ID)
	if err != nil || !ok || step.ID != second.ID {
		t.Errorf("NextStep = (%+v, %v, %v), want %q", step, ok, err, "second")
	}
}
