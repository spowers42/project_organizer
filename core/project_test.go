package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// categoryID returns the id of the seeded Category named name.
func categoryID(t *testing.T, c *core.Core, name string) int64 {
	t.Helper()
	cats, err := c.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	for _, cat := range cats {
		if cat.Name == name {
			return cat.ID
		}
	}
	t.Fatalf("seeded category %q not found", name)
	return 0
}

// mustCreateProject creates a Project through core and fails the test on error.
func mustCreateProject(t *testing.T, c *core.Core, name string, catID int64) core.Project {
	t.Helper()
	p, err := c.CreateProject(context.Background(), core.ProjectInput{
		Name:        name,
		Description: "desc of " + name,
		CategoryID:  catID,
	})
	if err != nil {
		t.Fatalf("CreateProject(%q): %v", name, err)
	}
	return p
}

func TestCreateProjectReadBack(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	prog := categoryID(t, c, "Programming")

	created, err := c.CreateProject(ctx, core.ProjectInput{
		Name:        "Write the parser",
		Description: "hand-rolled recursive descent",
		CategoryID:  prog,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if created.ID == 0 {
		t.Error("created Project has zero ID")
	}

	got, err := c.GetProject(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Write the parser" {
		t.Errorf("Name = %q, want %q", got.Name, "Write the parser")
	}
	if got.Description != "hand-rolled recursive descent" {
		t.Errorf("Description = %q, want %q", got.Description, "hand-rolled recursive descent")
	}
	if got.CategoryID != prog {
		t.Errorf("CategoryID = %d, want %d", got.CategoryID, prog)
	}
}

func TestNewProjectDefaultsToActive(t *testing.T) {
	c, _ := newTestCore(t)
	p := mustCreateProject(t, c, "Fresh", categoryID(t, c, "Other"))

	if p.Lifecycle != core.Active {
		t.Errorf("new Project Lifecycle = %q, want %q (the documented default)", p.Lifecycle, core.Active)
	}
	if core.DefaultLifecycle != core.Active {
		t.Errorf("DefaultLifecycle = %q, want %q", core.DefaultLifecycle, core.Active)
	}
}

func TestCreateProjectTrimsAndRejectsEmptyName(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")

	for _, name := range []string{"", "   ", "\t\n"} {
		_, err := c.CreateProject(ctx, core.ProjectInput{Name: name, CategoryID: other})
		if !errors.Is(err, core.ErrEmptyProjectName) {
			t.Errorf("CreateProject(name=%q) error = %v, want ErrEmptyProjectName", name, err)
		}
	}

	p, err := c.CreateProject(ctx, core.ProjectInput{Name: "  padded  ", CategoryID: other})
	if err != nil {
		t.Fatalf("CreateProject(padded): %v", err)
	}
	if p.Name != "padded" {
		t.Errorf("Name = %q, want %q (trimmed)", p.Name, "padded")
	}
}

func TestCreateProjectRejectsUnknownCategory(t *testing.T) {
	c, _ := newTestCore(t)

	_, err := c.CreateProject(context.Background(), core.ProjectInput{
		Name:       "Orphan",
		CategoryID: 99999,
	})
	if !errors.Is(err, core.ErrCategoryNotFound) {
		t.Errorf("error = %v, want ErrCategoryNotFound", err)
	}
}

func TestEditProjectChangesNameDescriptionAndCategory(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	prog := categoryID(t, c, "Programming")
	course := categoryID(t, c, "Course")

	p := mustCreateProject(t, c, "Original", prog)

	edited, err := c.EditProject(ctx, p.ID, core.ProjectInput{
		Name:        "Renamed",
		Description: "new description",
		CategoryID:  course,
	})
	if err != nil {
		t.Fatalf("EditProject: %v", err)
	}
	if edited.Name != "Renamed" || edited.Description != "new description" || edited.CategoryID != course {
		t.Errorf("edited = %+v, want name=Renamed desc=%q category=%d", edited, "new description", course)
	}

	got, err := c.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Renamed" || got.Description != "new description" || got.CategoryID != course {
		t.Errorf("persisted = %+v, want the edited values", got)
	}
	// Lifecycle is untouched by an edit.
	if got.Lifecycle != core.Active {
		t.Errorf("Lifecycle = %q, want it unchanged (%q)", got.Lifecycle, core.Active)
	}
}

func TestEditProjectValidatesInput(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")
	p := mustCreateProject(t, c, "Editable", other)

	if _, err := c.EditProject(ctx, p.ID, core.ProjectInput{Name: "  ", CategoryID: other}); !errors.Is(err, core.ErrEmptyProjectName) {
		t.Errorf("empty name: error = %v, want ErrEmptyProjectName", err)
	}
	if _, err := c.EditProject(ctx, p.ID, core.ProjectInput{Name: "ok", CategoryID: 4242}); !errors.Is(err, core.ErrCategoryNotFound) {
		t.Errorf("bad category: error = %v, want ErrCategoryNotFound", err)
	}
}

func TestEditProjectUnknownIDErrors(t *testing.T) {
	c, _ := newTestCore(t)
	other := categoryID(t, c, "Other")

	_, err := c.EditProject(context.Background(), 777, core.ProjectInput{Name: "ghost", CategoryID: other})
	if !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("error = %v, want ErrProjectNotFound", err)
	}
}

// A missing Project is reported as ErrProjectNotFound even when the supplied
// input would also fail validation — the not-found check wins.
func TestEditProjectUnknownIDBeatsBadInput(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()

	for _, in := range []core.ProjectInput{
		{Name: "   ", CategoryID: categoryID(t, c, "Other")},
		{Name: "ok", CategoryID: 9999},
	} {
		_, err := c.EditProject(ctx, 888, in)
		if !errors.Is(err, core.ErrProjectNotFound) {
			t.Errorf("EditProject(unknown, %+v) error = %v, want ErrProjectNotFound", in, err)
		}
	}
}

func TestGetProjectUnknownIDErrors(t *testing.T) {
	c, _ := newTestCore(t)

	_, err := c.GetProject(context.Background(), 12345)
	if !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("error = %v, want ErrProjectNotFound", err)
	}
}

func TestSetProjectLifecycleEachState(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Lifecycle", categoryID(t, c, "Other"))

	for _, state := range []core.Lifecycle{
		core.Paused, core.Someday, core.Done, core.Abandoned, core.Active,
	} {
		updated, err := c.SetProjectLifecycle(ctx, p.ID, state)
		if err != nil {
			t.Fatalf("SetProjectLifecycle(%q): %v", state, err)
		}
		if updated.Lifecycle != state {
			t.Errorf("returned Lifecycle = %q, want %q", updated.Lifecycle, state)
		}
		got, err := c.GetProject(ctx, p.ID)
		if err != nil {
			t.Fatalf("GetProject: %v", err)
		}
		if got.Lifecycle != state {
			t.Errorf("persisted Lifecycle = %q, want %q", got.Lifecycle, state)
		}
	}
}

func TestDoneAndAbandonedAreDistinct(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")

	done := mustCreateProject(t, c, "Finished", other)
	if _, err := c.SetProjectLifecycle(ctx, done.ID, core.Done); err != nil {
		t.Fatalf("set Done: %v", err)
	}
	dropped := mustCreateProject(t, c, "Dropped", other)
	if _, err := c.SetProjectLifecycle(ctx, dropped.ID, core.Abandoned); err != nil {
		t.Fatalf("set Abandoned: %v", err)
	}

	if core.Done == core.Abandoned {
		t.Fatal("Done and Abandoned must not be the same value")
	}

	doneList, err := c.ListProjects(ctx, core.ProjectFilter{Lifecycle: core.Done})
	if err != nil {
		t.Fatalf("ListProjects(Done): %v", err)
	}
	if len(doneList) != 1 || doneList[0].ID != done.ID {
		t.Errorf("Done filter = %+v, want exactly the Finished Project", doneList)
	}
	abandonedList, err := c.ListProjects(ctx, core.ProjectFilter{Lifecycle: core.Abandoned})
	if err != nil {
		t.Fatalf("ListProjects(Abandoned): %v", err)
	}
	if len(abandonedList) != 1 || abandonedList[0].ID != dropped.ID {
		t.Errorf("Abandoned filter = %+v, want exactly the Dropped Project", abandonedList)
	}
}

func TestSetProjectLifecycleRejectsInvalidState(t *testing.T) {
	c, _ := newTestCore(t)
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))

	_, err := c.SetProjectLifecycle(context.Background(), p.ID, core.Lifecycle("Frozen"))
	if !errors.Is(err, core.ErrInvalidLifecycle) {
		t.Errorf("error = %v, want ErrInvalidLifecycle", err)
	}
}

func TestSetProjectLifecycleUnknownIDErrors(t *testing.T) {
	c, _ := newTestCore(t)

	_, err := c.SetProjectLifecycle(context.Background(), 4711, core.Paused)
	if !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("error = %v, want ErrProjectNotFound", err)
	}
}

// listNames returns the Project names from a list, in order.
func listNames(ps []core.Project) []string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return names
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListProjectsFilters(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	prog := categoryID(t, c, "Programming")
	course := categoryID(t, c, "Course")

	// prog/Active, prog/Paused, course/Active, course/Done
	pa := mustCreateProject(t, c, "prog-active", prog)
	pp := mustCreateProject(t, c, "prog-paused", prog)
	ca := mustCreateProject(t, c, "course-active", course)
	cd := mustCreateProject(t, c, "course-done", course)

	if _, err := c.SetProjectLifecycle(ctx, pp.ID, core.Paused); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if _, err := c.SetProjectLifecycle(ctx, cd.ID, core.Done); err != nil {
		t.Fatalf("done: %v", err)
	}
	_ = pa
	_ = ca

	tests := []struct {
		name   string
		filter core.ProjectFilter
		want   []string
	}{
		{"no filter returns all", core.ProjectFilter{}, []string{"prog-active", "prog-paused", "course-active", "course-done"}},
		{"by state Active", core.ProjectFilter{Lifecycle: core.Active}, []string{"prog-active", "course-active"}},
		{"by state Paused", core.ProjectFilter{Lifecycle: core.Paused}, []string{"prog-paused"}},
		{"by category Programming", core.ProjectFilter{CategoryID: prog}, []string{"prog-active", "prog-paused"}},
		{"by category Course", core.ProjectFilter{CategoryID: course}, []string{"course-active", "course-done"}},
		{"by state and category", core.ProjectFilter{Lifecycle: core.Active, CategoryID: course}, []string{"course-active"}},
		{"state and category no match", core.ProjectFilter{Lifecycle: core.Done, CategoryID: prog}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.ListProjects(ctx, tt.filter)
			if err != nil {
				t.Fatalf("ListProjects: %v", err)
			}
			if !equalStrings(listNames(got), tt.want) {
				t.Errorf("ListProjects(%+v) = %v, want %v", tt.filter, listNames(got), tt.want)
			}
		})
	}
}

func TestListProjectsRejectsInvalidLifecycleFilter(t *testing.T) {
	c, _ := newTestCore(t)

	_, err := c.ListProjects(context.Background(), core.ProjectFilter{Lifecycle: core.Lifecycle("Nope")})
	if !errors.Is(err, core.ErrInvalidLifecycle) {
		t.Errorf("error = %v, want ErrInvalidLifecycle", err)
	}
}

func TestActiveProjectsReturnsExactlyActive(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")

	act := mustCreateProject(t, c, "still-going", other)
	paused := mustCreateProject(t, c, "on-hold", other)
	someday := mustCreateProject(t, c, "maybe", other)
	if _, err := c.SetProjectLifecycle(ctx, paused.ID, core.Paused); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if _, err := c.SetProjectLifecycle(ctx, someday.ID, core.Someday); err != nil {
		t.Fatalf("someday: %v", err)
	}

	got, err := c.ActiveProjects(ctx)
	if err != nil {
		t.Fatalf("ActiveProjects: %v", err)
	}
	if !equalStrings(listNames(got), []string{"still-going"}) {
		t.Errorf("ActiveProjects = %v, want [still-going]", listNames(got))
	}
	_ = act
}
