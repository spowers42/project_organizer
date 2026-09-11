package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestDashboardDoNextShowsTheDrawnTaskAndItsProject(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	p, err := c.CreateProject(ctx, core.ProjectInput{Name: "Learn Go", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "write the parser"}); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	cmd := d.Update(key("d"))
	if cmd == nil {
		t.Fatal("pressing d produced no command")
	}
	d.Update(cmd())

	view := d.View()
	if !strings.Contains(view, "write the parser") {
		t.Errorf("view = %q, want the drawn Task shown", view)
	}
	if !strings.Contains(view, "Learn Go") {
		t.Errorf("view = %q, want the Task's Project shown", view)
	}
}

func TestDashboardDoNextEmptyPoolIsAGracefulMessage(t *testing.T) {
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	cmd := d.Update(key("d"))
	if cmd == nil {
		t.Fatal("pressing d produced no command")
	}
	d.Update(cmd())

	view := d.View()
	if !strings.Contains(strings.ToLower(view), "nothing to do next") {
		t.Errorf("view = %q, want a graceful empty-pool message", view)
	}
}

func TestDashboardDoNextRerollDrawsAgain(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	for i := 0; i < 5; i++ {
		p, err := c.CreateProject(ctx, core.ProjectInput{Name: "P", CategoryID: catID})
		if err != nil {
			t.Fatalf("CreateProject: %v", err)
		}
		if _, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "task"}); err != nil {
			t.Fatalf("AddTask: %v", err)
		}
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	cmd := d.Update(key("d"))
	d.Update(cmd())
	if d.doNext == nil || !d.doNext.ok {
		t.Fatalf("Do Next did not draw a candidate: %+v", d.doNext)
	}

	cmd = d.Update(key("r"))
	if cmd == nil {
		t.Fatal("pressing r (reroll) produced no command")
	}
	d.Update(cmd())
	if d.doNext == nil || !d.doNext.ok {
		t.Fatalf("reroll did not draw a candidate: %+v", d.doNext)
	}
}

func TestDashboardDoNextDismissesOnEscOrQ(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		c := newTestCore(t)
		d := newDashboard(c)
		drainInit(d.Update, d.Init())

		cmd := d.Update(key("d"))
		d.Update(cmd())
		if d.doNext == nil {
			t.Fatalf("pressing d did not open the Do Next view")
		}

		d.Update(key(k))
		if d.doNext != nil {
			t.Errorf("%q did not dismiss the Do Next view", k)
		}
	}
}

// A key with no assigned action in the Do Next view is a no-op: it neither
// dismisses the view nor leaks through to the dashboard's own bindings (e.g.
// n would otherwise open the New Project form).
func TestDashboardDoNextUnassignedKeyIsANoop(t *testing.T) {
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	cmd := d.Update(key("d"))
	d.Update(cmd())
	if d.doNext == nil {
		t.Fatal("pressing d did not open the Do Next view")
	}

	if cmd := d.Update(key("n")); cmd != nil {
		t.Error("an unassigned key produced a command, want a no-op")
	}
	if d.doNext == nil {
		t.Error("an unassigned key dismissed the Do Next view, want a no-op")
	}
	if d.overlay.active() {
		t.Error("an unassigned key opened the New Project overlay, want it swallowed by the Do Next view")
	}
}
