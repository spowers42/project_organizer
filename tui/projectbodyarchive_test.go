package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// Declining the archive confirmation on a loose Task leaves it untouched.
func TestProjectViewArchiveTaskDeclined(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "keep the Task")
	seedTasks(t, c, p.ID, "loose one")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	r, ok := v.body.selectedRow()
	if !ok || r.kind != looseTaskRow {
		t.Fatalf("selectedRow = %+v, %v, want a loose Task selected", r, ok)
	}
	taskID := r.task.ID

	v.Update(key("x"))
	if !v.overlay.active() {
		t.Fatal("pressing x did not open the archive confirmation")
	}
	if cmd := v.Update(key("n")); cmd != nil {
		t.Errorf("declining produced a command %v, want none", cmd())
	}
	if v.overlay.active() {
		t.Error("confirmation still open after declining")
	}
	if _, err := c.GetTask(ctx, taskID); err != nil {
		t.Errorf("GetTask after decline = %v, want the Task still live", err)
	}
}

// Confirming the archive on a loose Task removes it from the Project body.
func TestProjectViewArchiveTaskConfirmed(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "drop the Task")
	seedTasks(t, c, p.ID, "keeper", "doomed")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.body.down() // select "doomed"
	r, ok := v.body.selectedRow()
	if !ok || r.task.Title != "doomed" {
		t.Fatalf("selectedRow = %+v, %v, want %q selected", r, ok, "doomed")
	}
	taskID := r.task.ID

	v.Update(key("x"))
	if !v.overlay.active() {
		t.Fatal("pressing x did not open the archive confirmation")
	}
	cmd := v.Update(key("y"))
	if cmd == nil {
		t.Fatal("confirming produced no command")
	}
	runCmd(v.Update, cmd)

	if v.overlay.active() {
		t.Error("confirmation still open after confirming")
	}
	if _, err := c.GetTask(ctx, taskID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("GetTask after archive = %v, want ErrTaskNotFound", err)
	}
	titles := bodyTitles(t, c, p.ID)
	if len(titles) != 1 || titles[0] != "keeper" {
		t.Errorf("remaining loose Tasks = %v, want only %q", titles, "keeper")
	}
	// The cursor lands on a valid row rather than out of bounds.
	if !v.body.hasSelection() {
		t.Error("no selection after archiving the last row, want the cursor to land somewhere sane")
	}
}

// Archiving a Milestone cascades to its Tasks: both the header and its nested
// Tasks vanish from the Project body.
func TestProjectViewArchiveMilestoneCascadesToItsTasks(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "drop the Milestone")
	seedTasks(t, c, p.ID, "loose lead")
	m := seedMilestoneReturning(t, c, p.ID, "Alpha")
	seedMilestoneTasks(t, c, m.ID, "a1", "a2")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	v.body.down() // move off "loose lead" onto the Milestone header
	r, ok := v.body.selectedRow()
	if !ok || r.kind != milestoneHeadRow {
		t.Fatalf("selectedRow = %+v, %v, want the Milestone header selected", r, ok)
	}

	v.Update(key("x"))
	if !v.overlay.active() {
		t.Fatal("pressing x did not open the archive confirmation")
	}
	if !strings.Contains(v.overlay.render(), "Alpha") {
		t.Errorf("confirm text = %q, want it to name the Milestone", v.overlay.render())
	}
	cmd := v.Update(key("y"))
	if cmd == nil {
		t.Fatal("confirming produced no command")
	}
	runCmd(v.Update, cmd)

	if v.overlay.active() {
		t.Error("confirmation still open after confirming")
	}
	if _, err := c.GetMilestone(ctx, m.ID); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("GetMilestone after archive = %v, want ErrMilestoneNotFound", err)
	}
	view := v.View()
	if strings.Contains(view, "Alpha") || strings.Contains(view, "a1") || strings.Contains(view, "a2") {
		t.Errorf("view = %q, want the Milestone and its Tasks gone", view)
	}
	if !strings.Contains(view, "loose lead") {
		t.Errorf("view = %q, want the untouched loose Task to remain", view)
	}
}
