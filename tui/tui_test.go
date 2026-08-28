package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

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

// TestRootModelSmoke exercises the wiring the real program uses: build the root
// model, run its Init load, render, then push a few keys through Update. It
// stops short of tea.Program.Run, which needs a TTY.
func TestRootModelSmoke(t *testing.T) {
	m := newModel(newTestCore(t))

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Init() = nil, want a load command")
	}
	drainInit(func(msg tea.Msg) tea.Cmd { _, cmd := m.Update(msg); return cmd }, initCmd)

	if m.View() == "" {
		t.Fatal("View() returned empty string after init")
	}
	for _, k := range []string{"down", "up", "n", "esc", "f", "esc", "q"} {
		if _, cmd := m.Update(key(k)); cmd != nil {
			cmd() // run it; must not panic
		}
		if m.View() == "" {
			t.Fatalf("View() empty after key %q", k)
		}
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

	// `c` returns the dashboard to its default Active-only view.
	cmd = d.Update(key("c"))
	if cmd == nil {
		t.Fatal("c produced no reload command while a filter was active")
	}
	d.Update(cmd())
	if d.filter != defaultDashboardFilter {
		t.Errorf("filter after c = %+v, want the default (Active) filter", d.filter)
	}
	if len(d.projects) != 0 {
		t.Errorf("projects after c = %+v, want the Paused Project excluded again", d.projects)
	}
	if d.Update(key("c")) != nil {
		t.Error("c on the default view should be a no-op (no reload)")
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

// Declining the archive confirmation leaves the Project untouched.
func TestProjectViewArchiveDeclined(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p, err := c.CreateProject(ctx, core.ProjectInput{Name: "keep me", CategoryID: firstCategoryID(t, c)})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("d"))
	if v.confirmUI == nil {
		t.Fatal("pressing d did not open the archive confirmation")
	}
	if cmd := v.Update(key("n")); cmd != nil {
		t.Errorf("declining produced a command %v, want none", cmd())
	}
	if v.confirmUI != nil {
		t.Error("confirmation still open after declining")
	}
	if _, err := c.GetProject(ctx, p.ID); err != nil {
		t.Errorf("GetProject after decline = %v, want the Project still live", err)
	}
}

// Confirming the archive removes the Project and returns to the dashboard.
func TestRootModelArchiveProjectFromProjectView(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	p, err := c.CreateProject(ctx, core.ProjectInput{Name: "doomed", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	m := newModel(c)
	drainInit(func(msg tea.Msg) tea.Cmd { _, cmd := m.Update(msg); return cmd }, m.Init())

	_, cmd := m.Update(key("enter")) // open the Project
	m.Update(cmd())
	drainInit(func(msg tea.Msg) tea.Cmd { _, cmd := m.Update(msg); return cmd }, m.proj.Init())
	if m.screen != screenProject {
		t.Fatalf("screen = %d, want screenProject", m.screen)
	}

	m.Update(key("d")) // open the confirm
	if m.proj.confirmUI == nil {
		t.Fatal("d did not open the archive confirmation")
	}
	_, cmd = m.Update(key("y")) // confirm -> archiveProject cmd
	if cmd == nil {
		t.Fatal("confirming produced no command")
	}
	_, cmd = m.Update(cmd()) // projectArchivedMsg -> backToDashboardMsg cmd
	if cmd == nil {
		t.Fatal("archive result produced no navigation command")
	}
	_, cmd = m.Update(cmd()) // backToDashboardMsg -> dashboard reload cmd
	if cmd != nil {
		m.Update(cmd()) // projectsLoadedMsg
	}

	if m.screen != screenDashboard {
		t.Errorf("screen = %d, want screenDashboard after archive", m.screen)
	}
	if _, err := c.GetProject(ctx, p.ID); err == nil {
		t.Error("Project still live after confirmed archive")
	}
	if len(m.dash.projects) != 0 {
		t.Errorf("dashboard projects = %+v, want the archived Project gone", m.dash.projects)
	}
}
