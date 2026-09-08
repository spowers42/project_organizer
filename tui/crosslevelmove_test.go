package tui

import (
	"context"
	"slices"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// > on a loose Task opens a Milestone picker; choosing one crosses the Task
// into that Milestone, an explicit action distinct from an ordinary reorder.
func TestProjectViewMoveTaskIntoMilestone(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Cross")
	seedTasks(t, c, p.ID, "loose")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key(">")) // "loose" is selected first

	if !v.overlay.active() {
		t.Fatal("> on a loose Task did not open the Milestone picker")
	}
	runCmd(v.Update, v.Update(key("enter"))) // only one Milestone: Alpha

	if got := bodyTitles(t, c, p.ID); len(got) != 0 {
		t.Errorf("loose tasks = %v, want none — the Task crossed into the Milestone", got)
	}
	if got := milestoneTaskTitles(t, c, m.ID); !slices.Equal(got, []string{"a1", "loose"}) {
		t.Errorf("milestone tasks = %v, want the moved Task appended", got)
	}
	if got := selectedBodyLabel(v); got != "loose" {
		t.Errorf("selection = %q, want it to follow the moved Task", got)
	}
}

// > with no Milestone in the body stays inert and reports why.
func TestProjectViewMoveTaskIntoMilestoneWithNoneStaysInert(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "No milestones")
	seedTasks(t, c, p.ID, "loose")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key(">"))

	if v.overlay.active() {
		t.Fatal("> opened an overlay with no Milestone to move into")
	}
	if got := bodyTitles(t, c, p.ID); !slices.Equal(got, []string{"loose"}) {
		t.Errorf("body = %v, want the Task untouched", got)
	}
}

// < on a Milestone Task crosses it back out to the Project body as a loose
// Task, an explicit action distinct from an ordinary reorder.
func TestProjectViewMoveTaskOutToBody(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Cross back")
	seedTasks(t, c, p.ID, "lead")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1", "a2")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down")) // "lead" -> "Alpha"
	v.Update(key("down")) // "Alpha" -> "a1"
	runCmd(v.Update, v.Update(key("<")))

	if got := milestoneTaskTitles(t, c, m.ID); !slices.Equal(got, []string{"a2"}) {
		t.Errorf("milestone tasks = %v, want a1 gone and the gap closed", got)
	}
	if got := bodyTitles(t, c, p.ID); !slices.Equal(got, []string{"lead", "a1"}) {
		t.Errorf("loose tasks = %v, want a1 appended to the Project body", got)
	}
	if got := selectedBodyLabel(v); got != "a1" {
		t.Errorf("selection = %q, want it to follow the moved Task", got)
	}
}

// < on a loose Task and > on a Milestone Task both stay inert — a plain
// within-level reorder never crosses the boundary through these keys either.
func TestProjectViewCrossLevelKeysStayInertOnTheWrongSideOfTheBoundary(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Wrong side")
	seedTasks(t, c, p.ID, "loose")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("<")) // "loose" is selected: no Milestone to leave

	if got := bodyTitles(t, c, p.ID); !slices.Equal(got, []string{"loose"}) {
		t.Errorf("body = %v, want the loose Task untouched by <", got)
	}

	v.Update(key("down")) // -> "Alpha"
	v.Update(key("down")) // -> "a1"
	v.Update(key(">"))    // a1 is already inside a Milestone: > only moves loose Tasks in

	if v.overlay.active() {
		t.Fatal("> opened the Milestone picker for a Task already inside a Milestone")
	}
	if got := milestoneTaskTitles(t, c, m.ID); !slices.Equal(got, []string{"a1"}) {
		t.Errorf("milestone tasks = %v, want a1 untouched by >", got)
	}
}

// Completing the last incomplete Task in a Milestone opens the confirm/decline
// prompt exactly once; confirming acknowledges it and stops the re-prompt.
func TestProjectViewMilestoneCompletionPromptConfirm(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Prompt")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "only")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down"))                // "Alpha" -> "only"
	runCmd(v.Update, v.Update(key(" "))) // complete "only"

	if !v.overlay.active() {
		t.Fatal("completing the Milestone's last Task did not open the prompt")
	}
	runCmd(v.Update, v.Update(key("y")))

	if v.overlay.active() {
		t.Error("overlay still open after confirming")
	}
	if got := milestoneByIDInBody(t, c, p.ID, m.ID); !got.CompletionAcked {
		t.Errorf("CompletionAcked = false after confirming, want true")
	}
}

// Declining also acknowledges the Milestone — the prompt does not re-show —
// but leaves it open for more Tasks.
func TestProjectViewMilestoneCompletionPromptDecline(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Prompt decline")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "only")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.Update(key("down")) // "Alpha" -> "only"
	runCmd(v.Update, v.Update(key(" ")))
	runCmd(v.Update, v.Update(key("n")))

	if v.overlay.active() {
		t.Error("overlay still open after declining")
	}
	if got := milestoneByIDInBody(t, c, p.ID, m.ID); !got.CompletionAcked {
		t.Errorf("CompletionAcked = false after declining, want true (acknowledged, but still open)")
	}
}

// Moving the current Next-step Task into a Milestone changes the dashboard's
// Next step to reflect its new location.
func TestCrossLevelMoveChangesDashboardNextStep(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "Next step")
	seedTasks(t, c, p.ID, "loose")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1")

	d := newDashboard(c)
	drainInit(d.Update, d.Init())
	if step, ok := d.nextSteps[p.ID]; !ok || step.Title != "loose" {
		t.Fatalf("nextSteps[%d] = (%+v, %v), want %q before the move", p.ID, step, ok, "loose")
	}

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	runCmd(v.Update, v.Update(key(">")))
	runCmd(v.Update, v.Update(key("enter")))

	d = newDashboard(c)
	drainInit(d.Update, d.Init())
	if step, ok := d.nextSteps[p.ID]; !ok || step.Title != "a1" {
		t.Errorf("nextSteps[%d] = (%+v, %v), want %q now that \"loose\" moved into the Milestone", p.ID, step, ok, "a1")
	}
}

// milestoneByIDInBody finds a Milestone in a Project's body by id.
func milestoneByIDInBody(t *testing.T, c *core.Core, projectID, milestoneID int64) core.Milestone {
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
