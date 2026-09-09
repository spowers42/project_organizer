package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// Pressing p in the Project view toggles the selected Task's Priority star and
// shows it in the body.
func TestProjectViewToggleTaskPriority(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "P")
	task, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "do the thing"})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	runCmd(v.Update, v.Update(key("p")))
	if got := findTask(t, c, p.ID, task.ID); !got.Priority {
		t.Errorf("Task Priority = false after pressing p, want true")
	}
	if !strings.Contains(v.View(), "★ do the thing") {
		t.Errorf("view = %q, want the Task shown with a Priority star", v.View())
	}

	runCmd(v.Update, v.Update(key("p")))
	if got := findTask(t, c, p.ID, task.ID); got.Priority {
		t.Errorf("Task Priority = true after pressing p again, want false")
	}
}

// Pressing P in the Project view toggles the viewed Project's Priority star and
// shows it in the header.
func TestProjectViewToggleProjectPriority(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "P")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	runCmd(v.Update, v.Update(key("P")))

	got, err := c.GetProject(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if !got.Priority {
		t.Errorf("Project Priority = false after pressing P, want true")
	}
	if !strings.Contains(v.View(), "Priority:    ★ starred") {
		t.Errorf("view = %q, want the header to show the Priority star", v.View())
	}

	runCmd(v.Update, v.Update(key("P")))
	if got, _ := c.GetProject(context.Background(), p.ID); got.Priority {
		t.Errorf("Project Priority = true after pressing P again, want false")
	}
}

// Pressing p on the dashboard toggles the selected Project's Priority star and
// the row shows it.
func TestDashboardToggleProjectPriority(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "in flight")

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	runCmd(d.Update, d.Update(key("p")))

	got, err := c.GetProject(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if !got.Priority {
		t.Errorf("Project Priority = false after pressing p, want true")
	}
	if !strings.Contains(d.View(), "★ in flight") {
		t.Errorf("view = %q, want the row to show a Priority star", d.View())
	}
}

// Pressing s on the dashboard sorts the list Priority-first for display only —
// the stored creation order is untouched.
func TestDashboardPriorityFirstSortIsViewOnly(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	first := mustProject(t, c, "first")
	second := mustProject(t, c, "second")
	third := mustProject(t, c, "third")
	if _, err := c.SetProjectPriority(ctx, third.ID, true); err != nil {
		t.Fatalf("SetProjectPriority: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	// Default: creation order.
	if got := d.View(); strings.Index(got, "first") > strings.Index(got, "third") {
		t.Errorf("default view = %q, want creation order (first before third)", got)
	}

	d.Update(key("s"))
	got := d.View()
	if strings.Index(got, "★ third") > strings.Index(got, "second") {
		t.Errorf("sorted view = %q, want the starred Project first", got)
	}
	if !strings.Contains(got, "Sorted Priority-first.") {
		t.Errorf("view = %q, want a status line confirming the sort", got)
	}

	// Stored order is unchanged — ListProjects still returns creation order.
	ps, err := c.ListProjects(ctx, core.ProjectFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(ps) != 3 || ps[0].ID != first.ID || ps[1].ID != second.ID || ps[2].ID != third.ID {
		t.Errorf("stored order = %+v, want it untouched by the view sort", ps)
	}

	// Toggling s again returns to creation order.
	d.Update(key("s"))
	if got := d.View(); !strings.Contains(got, "Sorted in creation order.") {
		t.Errorf("view = %q, want the creation-order status after toggling back", got)
	}
}

// Pressing o in the Project view shows the body Priority-first for display only;
// the stored body order (ADR 0001) is never touched.
func TestProjectViewSortPriorityFirstIsViewOnly(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "P")
	a, _ := c.AddTask(ctx, p.ID, core.TaskInput{Title: "alpha"})
	b, _ := c.AddTask(ctx, p.ID, core.TaskInput{Title: "bravo"})
	cc, _ := c.AddTask(ctx, p.ID, core.TaskInput{Title: "charlie"})
	if _, err := c.SetTaskPriority(ctx, cc.ID, true); err != nil {
		t.Fatalf("SetTaskPriority: %v", err)
	}
	_ = a
	_ = b

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	// Default: body order.
	if got := v.View(); strings.Index(got, "alpha") > strings.Index(got, "charlie") {
		t.Errorf("default view = %q, want stored body order (alpha before charlie)", got)
	}

	runCmd(v.Update, v.Update(key("o")))
	got := v.View()
	if strings.Index(got, "★ charlie") > strings.Index(got, "alpha") {
		t.Errorf("sorted view = %q, want the starred Task shown first", got)
	}
	if !strings.Contains(got, "Priority-first") {
		t.Errorf("view = %q, want a status line noting the display sort", got)
	}

	// Stored order is unchanged.
	tasks, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if len(tasks) != 3 || tasks[0].Title != "alpha" || tasks[1].Title != "bravo" || tasks[2].Title != "charlie" {
		t.Errorf("stored body order = %v, want it untouched by the display sort", tasks)
	}

	// Toggling o again returns to stored order.
	runCmd(v.Update, v.Update(key("o")))
	if got := v.View(); strings.Index(got, "alpha") > strings.Index(got, "charlie") {
		t.Errorf("view after toggling back = %q, want stored body order again", got)
	}
}

func TestRenderProjectRowsShowsPriorityStar(t *testing.T) {
	got := renderProjectRows([]core.Project{
		{ID: 1, Name: "starred one", Lifecycle: core.Active, Priority: true},
		{ID: 2, Name: "plain one", Lifecycle: core.Active},
	}, 0, nil)

	if !strings.Contains(got, "★ starred one") {
		t.Errorf("rows = %q, want a star on the starred Project", got)
	}
	if strings.Contains(got, "★ plain one") {
		t.Errorf("rows = %q, want no star on the unstarred Project", got)
	}
}
