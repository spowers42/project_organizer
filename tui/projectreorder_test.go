package tui

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// seedTasks adds titled loose Tasks to a Project in order.
func seedTasks(t *testing.T, c *core.Core, projectID int64, titles ...string) {
	t.Helper()
	for _, title := range titles {
		if _, err := c.AddTask(context.Background(), projectID, core.TaskInput{Title: title}); err != nil {
			t.Fatalf("AddTask(%q): %v", title, err)
		}
	}
}

// bodyTitles is the Project's loose Task titles in stored body order.
func bodyTitles(t *testing.T, c *core.Core, projectID int64) []string {
	t.Helper()
	tasks, err := c.ProjectTasks(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	out := make([]string, len(tasks))
	for i, tk := range tasks {
		out[i] = tk.Title
	}
	return out
}

// selectedBodyLabel is the title (Task) or name (Milestone) of the Project
// view's currently selected body entry, or "" when nothing is selected.
func selectedBodyLabel(v *projectViewModel) string {
	r, ok := v.body.selectedRow()
	if !ok {
		return ""
	}
	if r.kind == milestoneHeadRow {
		return r.milestone.Name
	}
	return r.task.Title
}

// shift+down on the selected Task reorders the stored body and keeps the
// selection on the moved Task.
func TestProjectViewMoveTaskDown(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Re-plan")
	seedTasks(t, c, p.ID, "alpha", "beta", "gamma")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	runCmd(v.Update, v.Update(key("shift+down"))) // move "alpha" past "beta"

	if got := bodyTitles(t, c, p.ID); !slices.Equal(got, []string{"beta", "alpha", "gamma"}) {
		t.Errorf("body = %v, want alpha moved down one slot", got)
	}
	if got := selectedBodyLabel(v); got != "alpha" {
		t.Errorf("selection = %q, want it to follow the moved Task", got)
	}
	if !strings.Contains(v.View(), "> [ ] alpha") {
		t.Errorf("view = %q, want the caret on the moved Task", v.View())
	}
}

// shift+up on a lower Task reorders it earlier and the selection tracks it.
// K is the capital-letter alias, mirroring the k/j selection keys.
func TestProjectViewMoveTaskUp(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Re-plan")
	seedTasks(t, c, p.ID, "alpha", "beta", "gamma")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("down")) // select "beta"
	runCmd(v.Update, v.Update(key("K")))

	if got := bodyTitles(t, c, p.ID); !slices.Equal(got, []string{"beta", "alpha", "gamma"}) {
		t.Errorf("body = %v, want beta moved up one slot", got)
	}
	if got := selectedBodyLabel(v); got != "beta" {
		t.Errorf("selection = %q, want it to follow the moved Task", got)
	}
}

// Moving the first Task up is a no-op that leaves the body and selection alone.
func TestProjectViewMoveTaskUpAtTopEdgeIsNoOp(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Re-plan")
	seedTasks(t, c, p.ID, "alpha", "beta")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	runCmd(v.Update, v.Update(key("shift+up")))

	if got := bodyTitles(t, c, p.ID); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Errorf("body = %v, want it unchanged at the top edge", got)
	}
	if v.body.sel != 0 {
		t.Errorf("body.sel = %d, want 0", v.body.sel)
	}
}

// Reordering so a different incomplete Task comes first changes the dashboard's
// Next step after a reload.
func TestReorderChangesDashboardNextStep(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Re-plan")
	seedTasks(t, c, p.ID, "alpha", "beta")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down")) // select "beta"
	runCmd(v.Update, v.Update(key("shift+up")))

	d := newDashboard(c)
	drainInit(d.Update, d.Init())
	if step, ok := d.nextSteps[p.ID]; !ok || step.Title != "beta" {
		t.Errorf("nextSteps[%d] = (%+v, %v), want %q after the reorder", p.ID, step, ok, "beta")
	}
}
