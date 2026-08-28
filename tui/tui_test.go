package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
	"github.com/spowers42/project_organizer/internal/store"
)

// newTestCore wires a Core over a fresh temp-file database.
func newTestCore(t *testing.T) *core.Core {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "organizer.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return core.New(st, core.SystemClock{}, core.NewRand(1))
}

func firstCategoryID(t *testing.T, c *core.Core) int64 {
	t.Helper()
	cats, err := c.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats) == 0 {
		t.Fatal("no seeded Categories")
	}
	return cats[0].ID
}

// drainInit runs the commands from a screen's Init to completion and feeds the
// resulting messages back into Update, so the screen reaches its loaded state.
func drainInit(update func(tea.Msg) tea.Cmd, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			update(c())
		}
		return
	}
	update(msg)
}

func TestRenderProjectRowsEmptyState(t *testing.T) {
	got := renderProjectRows(nil, 0)
	if !strings.Contains(got, "No Projects match") {
		t.Errorf("rows = %q, want an empty-state message", got)
	}
}

func TestRenderProjectRowsMarksSelectionAndShowsLifecycle(t *testing.T) {
	got := renderProjectRows([]core.Project{
		{Name: "Write the parser", Lifecycle: core.Active},
		{Name: "Refactor the store", Lifecycle: core.Paused},
	}, 1)

	if !strings.Contains(got, "> Refactor the store") {
		t.Errorf("rows = %q, want a caret on the selected row", got)
	}
	if !strings.Contains(got, "[Active]") || !strings.Contains(got, "[Paused]") {
		t.Errorf("rows = %q, want each row to show its lifecycle state", got)
	}
}

func TestDashboardViewDoesNotPanicWithNoCore(t *testing.T) {
	if out := newDashboard(nil).View(); out == "" {
		t.Error("dashboard View() returned empty string")
	}
}

func TestRootModelSmoke(t *testing.T) {
	m := newModel(newTestCore(t))
	if m.Init() == nil {
		t.Error("Init() = nil, want a load command")
	}
	if m.View() == "" {
		t.Error("View() returned empty string")
	}
}

// The dashboard's default load asks core for the Active Projects only.
func TestDashboardLoadsOnlyActiveProjects(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)

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
}

// Driving the create overlay end to end persists a Project through core.
func TestDashboardCreateProjectFlow(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("n")) // open the New Project form
	if d.form == nil {
		t.Fatal("pressing n did not open the create form")
	}
	for _, m := range typeString("Learn Go") {
		d.Update(m)
	}
	cmd := d.Update(key("enter"))
	if cmd == nil {
		t.Fatal("submitting the form produced no command")
	}
	d.Update(cmd()) // apply projectSavedMsg

	if d.form != nil {
		t.Error("form still open after a successful create")
	}
	got, err := c.ListProjects(ctx, core.ProjectFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Learn Go" {
		t.Errorf("projects = %+v, want one named %q", got, "Learn Go")
	}
	if got[0].Lifecycle != core.Active {
		t.Errorf("new Project lifecycle = %q, want the default Active", got[0].Lifecycle)
	}
}

// An invalid submission keeps the form open and shows the mapped message.
func TestDashboardCreateProjectValidationKeepsFormOpen(t *testing.T) {
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("n"))
	cmd := d.Update(key("enter")) // empty name
	d.Update(cmd())

	if d.form == nil {
		t.Error("form closed on a validation error, want it kept open")
	}
	if !strings.Contains(d.View(), "name must not be empty") {
		t.Errorf("view = %q, want the empty-name message", d.View())
	}
}

// Filtering re-queries core with the chosen constraint.
func TestDashboardFilterOverlayWidensToPaused(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	p, err := c.CreateProject(ctx, core.ProjectInput{Name: "on hold", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := c.SetProjectLifecycle(ctx, p.ID, core.Paused); err != nil {
		t.Fatalf("SetProjectLifecycle: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())
	if len(d.projects) != 0 {
		t.Fatalf("default (Active) view = %+v, want empty", d.projects)
	}

	d.Update(key("f"))
	// The overlay opens on the current filter (Active); step once to Paused.
	d.Update(key("down"))
	cmd := d.Update(key("enter"))
	if cmd == nil {
		t.Fatal("applying the filter produced no reload command")
	}
	d.Update(cmd())

	if d.filter.Lifecycle != core.Paused {
		t.Errorf("filter lifecycle = %q, want Paused", d.filter.Lifecycle)
	}
	if len(d.projects) != 1 || d.projects[0].ID != p.ID {
		t.Errorf("projects after filter = %+v, want the Paused one", d.projects)
	}
}

// Opening a row navigates the root model to the Project view and back.
func TestRootModelNavigatesToProjectViewAndBack(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	p, err := c.CreateProject(ctx, core.ProjectInput{Name: "nav target", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	m := newModel(c)
	drainInit(func(msg tea.Msg) tea.Cmd { _, cmd := m.Update(msg); return cmd }, m.Init())

	_, cmd := m.Update(key("enter")) // open the selected row
	if cmd == nil {
		t.Fatal("enter on the dashboard produced no command")
	}
	m.Update(cmd()) // openProjectMsg
	if m.screen != screenProject {
		t.Fatalf("screen = %d, want screenProject", m.screen)
	}
	if m.proj == nil || m.proj.projectID != p.ID {
		t.Fatalf("project view = %+v, want it bound to Project %d", m.proj, p.ID)
	}

	_, cmd = m.Update(key("esc")) // back
	m.Update(cmd())
	if m.screen != screenDashboard {
		t.Errorf("screen = %d, want screenDashboard after esc", m.screen)
	}
}

// The Project view can edit fields and move the Project through its lifecycle.
func TestProjectViewEditAndLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	p, err := c.CreateProject(ctx, core.ProjectInput{Name: "old name", CategoryID: cats[0].ID})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())
	if !v.ready() {
		t.Fatalf("project view not ready after init (loadErr=%v)", v.loadErr)
	}

	// Edit: clear the name, type a new one, save.
	v.Update(key("e"))
	for i := 0; i < len("old name"); i++ {
		v.Update(key("backspace"))
	}
	for _, m := range typeString("new name") {
		v.Update(m)
	}
	cmd := v.Update(key("enter"))
	v.Update(cmd())
	if got, _ := c.GetProject(ctx, p.ID); got.Name != "new name" {
		t.Errorf("Project name = %q, want %q", got.Name, "new name")
	}

	// Lifecycle: open the picker, move to Done, set.
	v.Update(key("s"))
	if v.lifeUI == nil {
		t.Fatal("pressing s did not open the lifecycle picker")
	}
	for v.lifeUI.value() != string(core.Done) {
		v.Update(key("down"))
	}
	cmd = v.Update(key("enter"))
	v.Update(cmd())
	if got, _ := c.GetProject(ctx, p.ID); got.Lifecycle != core.Done {
		t.Errorf("Project lifecycle = %q, want Done", got.Lifecycle)
	}
}
