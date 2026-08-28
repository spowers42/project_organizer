package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spowers42/project_organizer/core"
)

// mustAddTask adds a loose Task through core and fails the test on error.
func mustAddTask(t *testing.T, c *core.Core, projectID int64, title string) core.Task {
	t.Helper()
	task, err := c.AddTask(context.Background(), projectID, core.TaskInput{Title: title})
	if err != nil {
		t.Fatalf("AddTask(%q): %v", title, err)
	}
	return task
}

// taskTitles is the ordered list of titles from a Task slice.
func taskTitles(ts []core.Task) []string {
	out := make([]string, len(ts))
	for i, task := range ts {
		out[i] = task.Title
	}
	return out
}

func TestAddTaskAppendsInBodyOrder(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "Ship it", categoryID(t, c, "Programming"))

	mustAddTask(t, c, p.ID, "draft the design")
	mustAddTask(t, c, p.ID, "write the code")
	mustAddTask(t, c, p.ID, "ship")

	tasks, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if !equalStrings(taskTitles(tasks), []string{"draft the design", "write the code", "ship"}) {
		t.Errorf("tasks = %v, want them in the order they were added", taskTitles(tasks))
	}
	for _, task := range tasks {
		if task.ProjectID != p.ID {
			t.Errorf("task %q ProjectID = %d, want %d", task.Title, task.ProjectID, p.ID)
		}
		if task.Done {
			t.Errorf("task %q starts done, want not done", task.Title)
		}
	}
}

func TestAddTaskTrimsAndRejectsEmptyTitle(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))

	for _, title := range []string{"", "   ", "\t\n"} {
		if _, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: title}); !errors.Is(err, core.ErrEmptyTaskTitle) {
			t.Errorf("AddTask(title=%q) error = %v, want ErrEmptyTaskTitle", title, err)
		}
	}

	task, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "  padded  "})
	if err != nil {
		t.Fatalf("AddTask(padded): %v", err)
	}
	if task.Title != "padded" {
		t.Errorf("Title = %q, want %q (trimmed)", task.Title, "padded")
	}
}

func TestAddTaskUnknownProjectErrors(t *testing.T) {
	c, _ := newTestCore(t)

	_, err := c.AddTask(context.Background(), 4242, core.TaskInput{Title: "orphan"})
	if !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("error = %v, want ErrProjectNotFound", err)
	}
}

func TestAddTaskDueDateIsOptionalAndStored(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))

	loose := mustAddTask(t, c, p.ID, "no due date")
	if loose.DueDate != nil {
		t.Errorf("DueDate = %v, want nil for a Task added without one", loose.DueDate)
	}

	due := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	dated, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "with due date", DueDate: &due})
	if err != nil {
		t.Fatalf("AddTask(dated): %v", err)
	}
	if dated.DueDate == nil || !dated.DueDate.Equal(due) {
		t.Errorf("DueDate = %v, want %v", dated.DueDate, due)
	}

	got, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if got[1].DueDate == nil || !got[1].DueDate.Equal(due) {
		t.Errorf("persisted DueDate = %v, want %v", got[1].DueDate, due)
	}
}

func TestEditTaskChangesTitleAndDueDate(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))
	due := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
	task, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "original", DueDate: &due})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	newDue := time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC)
	edited, err := c.EditTask(ctx, task.ID, core.TaskInput{Title: "renamed", DueDate: &newDue})
	if err != nil {
		t.Fatalf("EditTask: %v", err)
	}
	if edited.Title != "renamed" {
		t.Errorf("Title = %q, want %q", edited.Title, "renamed")
	}
	if edited.DueDate == nil || !edited.DueDate.Equal(newDue) {
		t.Errorf("DueDate = %v, want %v", edited.DueDate, newDue)
	}

	// A nil DueDate on an edit clears the existing one.
	cleared, err := c.EditTask(ctx, task.ID, core.TaskInput{Title: "renamed"})
	if err != nil {
		t.Fatalf("EditTask(clear due date): %v", err)
	}
	if cleared.DueDate != nil {
		t.Errorf("DueDate = %v, want nil after clearing", cleared.DueDate)
	}
}

func TestEditTaskValidatesTitleAndID(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))
	task := mustAddTask(t, c, p.ID, "editable")

	if _, err := c.EditTask(ctx, task.ID, core.TaskInput{Title: "   "}); !errors.Is(err, core.ErrEmptyTaskTitle) {
		t.Errorf("empty title: error = %v, want ErrEmptyTaskTitle", err)
	}
	if _, err := c.EditTask(ctx, 9999, core.TaskInput{Title: "ghost"}); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown id: error = %v, want ErrTaskNotFound", err)
	}
	// A missing Task wins over invalid input.
	if _, err := c.EditTask(ctx, 9999, core.TaskInput{Title: "  "}); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("unknown id + bad input: error = %v, want ErrTaskNotFound", err)
	}
}

func TestSetTaskDoneTogglesInAnyOrder(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))
	first := mustAddTask(t, c, p.ID, "first")
	second := mustAddTask(t, c, p.ID, "second")
	third := mustAddTask(t, c, p.ID, "third")

	// Complete out of body order: third, then first. Second stays open.
	if _, err := c.SetTaskDone(ctx, third.ID, true); err != nil {
		t.Fatalf("SetTaskDone(third, true): %v", err)
	}
	if _, err := c.SetTaskDone(ctx, first.ID, true); err != nil {
		t.Fatalf("SetTaskDone(first, true): %v", err)
	}

	tasks, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	got := map[string]bool{}
	for _, task := range tasks {
		got[task.Title] = task.Done
	}
	if !got["first"] || got["second"] || !got["third"] {
		t.Errorf("done flags = %v, want first+third done, second open", got)
	}

	// Un-mark first; completion is reversible.
	undone, err := c.SetTaskDone(ctx, first.ID, false)
	if err != nil {
		t.Fatalf("SetTaskDone(first, false): %v", err)
	}
	if undone.Done {
		t.Errorf("first still Done after un-marking, want not done")
	}
	_ = second
}

func TestSetTaskDoneUnknownIDErrors(t *testing.T) {
	c, _ := newTestCore(t)

	if _, err := c.SetTaskDone(context.Background(), 777, true); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("error = %v, want ErrTaskNotFound", err)
	}
}

func TestProjectTasksUnknownProjectErrors(t *testing.T) {
	c, _ := newTestCore(t)

	if _, err := c.ProjectTasks(context.Background(), 123); !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("error = %v, want ErrProjectNotFound", err)
	}
}

func TestNextStepIsFirstIncompleteLooseTask(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Programming"))
	one := mustAddTask(t, c, p.ID, "one")
	two := mustAddTask(t, c, p.ID, "two")
	mustAddTask(t, c, p.ID, "three")

	// All open: Next step is the first in body order.
	step, ok, err := c.NextStep(ctx, p.ID)
	if err != nil || !ok {
		t.Fatalf("NextStep = (%+v, %v, %v), want the first Task", step, ok, err)
	}
	if step.ID != one.ID {
		t.Errorf("NextStep = %q, want %q", step.Title, "one")
	}

	// Complete the first; Next step advances to the next incomplete entry,
	// even though a later Task is still open too.
	if _, err := c.SetTaskDone(ctx, one.ID, true); err != nil {
		t.Fatalf("SetTaskDone(one): %v", err)
	}
	step, ok, err = c.NextStep(ctx, p.ID)
	if err != nil || !ok {
		t.Fatalf("NextStep after completing one = (%+v, %v, %v)", step, ok, err)
	}
	if step.ID != two.ID {
		t.Errorf("NextStep = %q, want %q", step.Title, "two")
	}
}

func TestNextStepEmptyProjectHasNone(t *testing.T) {
	c, _ := newTestCore(t)
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))

	step, ok, err := c.NextStep(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("NextStep: %v", err)
	}
	if ok {
		t.Errorf("NextStep = %q, want none for a Project with no Tasks", step.Title)
	}
}

func TestNextStepAllDoneHasNone(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))
	a := mustAddTask(t, c, p.ID, "a")
	b := mustAddTask(t, c, p.ID, "b")
	for _, id := range []int64{a.ID, b.ID} {
		if _, err := c.SetTaskDone(ctx, id, true); err != nil {
			t.Fatalf("SetTaskDone(%d): %v", id, err)
		}
	}

	step, ok, err := c.NextStep(ctx, p.ID)
	if err != nil {
		t.Fatalf("NextStep: %v", err)
	}
	if ok {
		t.Errorf("NextStep = %q, want none once every Task is done", step.Title)
	}
}

func TestNextStepUnknownProjectErrors(t *testing.T) {
	c, _ := newTestCore(t)

	if _, _, err := c.NextStep(context.Background(), 55); !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("error = %v, want ErrProjectNotFound", err)
	}
}

func TestDashboardShowsNextStepForEachActiveProject(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")

	waiting := mustCreateProject(t, c, "waiting", other)
	mustAddTask(t, c, waiting.ID, "do this next")
	mustAddTask(t, c, waiting.ID, "then this")

	finished := mustCreateProject(t, c, "finished", other)
	fin := mustAddTask(t, c, finished.ID, "already done")
	if _, err := c.SetTaskDone(ctx, fin.ID, true); err != nil {
		t.Fatalf("SetTaskDone: %v", err)
	}

	// A Paused Project with an open Task must not appear on the dashboard.
	paused := mustCreateProject(t, c, "paused", other)
	mustAddTask(t, c, paused.ID, "not in flight")
	if _, err := c.SetProjectLifecycle(ctx, paused.ID, core.Paused); err != nil {
		t.Fatalf("SetProjectLifecycle: %v", err)
	}

	rows, err := c.Dashboard(ctx)
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if !equalStrings(projectNames(rows), []string{"waiting", "finished"}) {
		t.Fatalf("dashboard projects = %v, want only the Active ones", projectNames(rows))
	}

	if rows[0].NextStep == nil || rows[0].NextStep.Title != "do this next" {
		t.Errorf("waiting Next step = %v, want %q", rows[0].NextStep, "do this next")
	}
	if rows[1].NextStep != nil {
		t.Errorf("finished Next step = %q, want none (all Tasks done)", rows[1].NextStep.Title)
	}
}

// projectNames is the ordered list of Project names from dashboard rows.
func projectNames(rows []core.ActiveProjectNextStep) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Project.Name
	}
	return out
}
