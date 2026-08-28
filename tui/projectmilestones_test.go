package tui

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// seedMilestone adds a named Milestone to a Project through core.
func seedMilestone(t *testing.T, c *core.Core, projectID int64, name string) {
	t.Helper()
	if _, err := c.AddMilestone(context.Background(), projectID, name); err != nil {
		t.Fatalf("AddMilestone(%q): %v", name, err)
	}
}

// bodyLabelsOf renders a Project's stored body as ordered "task:" / "ms:"
// labels.
func bodyLabelsOf(t *testing.T, c *core.Core, projectID int64) []string {
	t.Helper()
	body, err := c.ProjectBody(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ProjectBody: %v", err)
	}
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

// Pressing m opens the name form; a name plus enter adds a Milestone that shows
// as one body entry.
func TestProjectViewAddMilestoneFlow(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Ship it")
	seedTasks(t, c, p.ID, "setup")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("m"))
	if !v.overlay.active() {
		t.Fatal("pressing m did not open the Milestone form")
	}
	for _, msg := range typeString("Alpha release") {
		v.Update(msg)
	}
	runCmd(v.Update, v.Update(key("enter")))

	if v.overlay.active() {
		t.Error("form still open after a successful add")
	}
	if got := bodyLabelsOf(t, c, p.ID); !slices.Equal(got, []string{"task:setup", "ms:Alpha release"}) {
		t.Fatalf("body = %v, want the Milestone as one trailing entry", got)
	}
	if !strings.Contains(v.View(), "◆ Alpha release  (Milestone)") {
		t.Errorf("view = %q, want the Milestone row rendered", v.View())
	}
}

// The body view lists loose Tasks and Milestones interleaved in stored order.
func TestProjectViewRendersInterleavedBody(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Interleave")
	seedTasks(t, c, p.ID, "setup")
	seedMilestone(t, c, p.ID, "Alpha")
	seedTasks(t, c, p.ID, "cleanup")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	view := v.View()
	setupAt := strings.Index(view, "[ ] setup")
	alphaAt := strings.Index(view, "◆ Alpha")
	cleanupAt := strings.Index(view, "[ ] cleanup")
	if setupAt < 0 || alphaAt < 0 || cleanupAt < 0 {
		t.Fatalf("view = %q, want all three body rows", view)
	}
	if setupAt >= alphaAt || alphaAt >= cleanupAt {
		t.Errorf("row order in view = (setup %d, Alpha %d, cleanup %d), want stored order", setupAt, alphaAt, cleanupAt)
	}
}

// shift+up on a selected Milestone reorders it earlier; the selection follows
// and the new order persists.
func TestProjectViewMoveMilestoneUp(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Reorder")
	seedTasks(t, c, p.ID, "first")
	seedMilestone(t, c, p.ID, "Beta")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("down")) // select "Beta"
	runCmd(v.Update, v.Update(key("shift+up")))

	if got := bodyLabelsOf(t, c, p.ID); !slices.Equal(got, []string{"ms:Beta", "task:first"}) {
		t.Errorf("body = %v, want Beta moved above first", got)
	}
	if got := selectedBodyLabel(v); got != "Beta" {
		t.Errorf("selection = %q, want it to follow the moved Milestone", got)
	}
}

// shift+down on a selected Milestone reorders it later; the selection follows
// and the new order persists.
func TestProjectViewMoveMilestoneDown(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Reorder")
	seedMilestone(t, c, p.ID, "Beta")
	seedTasks(t, c, p.ID, "last")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	// "Beta" is first and selected.

	runCmd(v.Update, v.Update(key("shift+down")))

	if got := bodyLabelsOf(t, c, p.ID); !slices.Equal(got, []string{"task:last", "ms:Beta"}) {
		t.Errorf("body = %v, want Beta moved below last", got)
	}
	if got := selectedBodyLabel(v); got != "Beta" {
		t.Errorf("selection = %q, want it to follow the moved Milestone", got)
	}
}

// A loose Task reorders through the Milestone boundary: before, between, and
// after Milestones.
func TestProjectViewLooseTaskMovesPastMilestones(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Weave")
	seedMilestone(t, c, p.ID, "Alpha")
	seedMilestone(t, c, p.ID, "Beta")
	seedTasks(t, c, p.ID, "weaver") // body: Alpha, Beta, weaver

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("down"))
	v.Update(key("down")) // select "weaver"

	runCmd(v.Update, v.Update(key("shift+up")))
	if got := bodyLabelsOf(t, c, p.ID); !slices.Equal(got, []string{"ms:Alpha", "task:weaver", "ms:Beta"}) {
		t.Fatalf("body = %v, want the Task between the Milestones", got)
	}
	runCmd(v.Update, v.Update(key("shift+up")))
	if got := bodyLabelsOf(t, c, p.ID); !slices.Equal(got, []string{"task:weaver", "ms:Alpha", "ms:Beta"}) {
		t.Fatalf("body = %v, want the Task ahead of both Milestones", got)
	}
	if got := selectedBodyLabel(v); got != "weaver" {
		t.Errorf("selection = %q, want it to track the moved Task", got)
	}
}

// The Task-only keys do nothing while a Milestone row is selected.
func TestProjectViewTaskKeysInertOnMilestone(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Guard")
	seedMilestone(t, c, p.ID, "Alpha")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	// "Alpha" is the only entry, so it is selected.

	if cmd := v.Update(key("t")); cmd != nil {
		t.Error("edit-Task key produced a command on a Milestone row")
	}
	if v.overlay.active() {
		t.Error("edit-Task key opened a form on a Milestone row")
	}
	if cmd := v.Update(key(" ")); cmd != nil {
		t.Error("toggle-done key produced a command on a Milestone row")
	}
}

// An empty Milestone at the front of the body leaves the dashboard Next step
// unchanged.
func TestEmptyMilestoneLeavesDashboardNextStep(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Next")
	seedTasks(t, c, p.ID, "alpha", "beta")
	seedMilestone(t, c, p.ID, "Gate")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down"))
	v.Update(key("down")) // select "Gate"
	runCmd(v.Update, v.Update(key("shift+up")))
	runCmd(v.Update, v.Update(key("shift+up"))) // body: Gate, alpha, beta

	d := newDashboard(c)
	drainInit(d.Update, d.Init())
	if step, ok := d.nextSteps[p.ID]; !ok || step.Title != "alpha" {
		t.Errorf("nextSteps[%d] = (%+v, %v), want %q despite the leading empty Milestone", p.ID, step, ok, "alpha")
	}
}
