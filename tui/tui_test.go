package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
	"github.com/spowers42/project_organizer/internal/store"
)

func TestDashboardViewRendersEmptyStateWithoutPanic(t *testing.T) {
	got := renderDashboard(nil)

	if !strings.Contains(got, "Project Organizer") {
		t.Errorf("dashboard view = %q, want it to contain the title", got)
	}
	if !strings.Contains(got, "No Active Projects yet.") {
		t.Errorf("dashboard view = %q, want it to show the empty state", got)
	}
}

func TestDashboardViewListsActiveProjects(t *testing.T) {
	got := renderDashboard([]core.Project{
		{Name: "Write the parser"},
		{Name: "Refactor the store"},
	})

	for _, want := range []string{"Write the parser", "Refactor the store"} {
		if !strings.Contains(got, want) {
			t.Errorf("dashboard view = %q, want it to list %q", got, want)
		}
	}
	if strings.Contains(got, "No Active Projects yet.") {
		t.Errorf("dashboard view = %q, should not show the empty state with Projects present", got)
	}
}

func TestDashboardModelViewDoesNotPanicWithZeroProjects(t *testing.T) {
	d := newDashboard(nil) // View must not touch core.
	if out := d.View(); out == "" {
		t.Error("dashboard View() returned empty string")
	}
}

// The dashboard's load path must ask core for the Active Projects specifically,
// not every Project.
func TestDashboardLoadsOnlyActiveProjects(t *testing.T) {
	ctx := context.Background()

	st, err := store.Open(filepath.Join(t.TempDir(), "organizer.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := core.New(st, core.SystemClock{}, core.NewRand(1))

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	catID := cats[0].ID

	active, err := c.CreateProject(ctx, core.ProjectInput{Name: "in flight", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateProject active: %v", err)
	}
	paused, err := c.CreateProject(ctx, core.ProjectInput{Name: "on hold", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateProject paused: %v", err)
	}
	if _, err := c.SetProjectLifecycle(ctx, paused.ID, core.Paused); err != nil {
		t.Fatalf("SetProjectLifecycle: %v", err)
	}

	msg, ok := newDashboard(c).loadProjects().(projectsLoadedMsg)
	if !ok {
		t.Fatalf("loadProjects returned %T, want projectsLoadedMsg", newDashboard(c).loadProjects())
	}
	if msg.err != nil {
		t.Fatalf("loadProjects msg carried error: %v", msg.err)
	}
	if len(msg.projects) != 1 || msg.projects[0].ID != active.ID {
		t.Errorf("loaded projects = %+v, want only the Active one (%q)", msg.projects, active.Name)
	}

	if got := renderDashboard(msg.projects); !strings.Contains(got, "in flight") || strings.Contains(got, "on hold") {
		t.Errorf("dashboard view = %q, want it to list only the Active Project", got)
	}
}
