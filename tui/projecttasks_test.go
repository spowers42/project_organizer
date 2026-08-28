package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// runCmd runs a command to completion and feeds every resulting message back
// into update, chasing follow-up commands until the model settles.
func runCmd(update func(tea.Msg) tea.Cmd, cmd tea.Cmd) {
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return
		}
		cmd = update(msg)
	}
}

// mustProject creates an Active Project for a Project-view test.
func mustProject(t *testing.T, c *core.Core, name string) core.Project {
	t.Helper()
	p, err := c.CreateProject(context.Background(), core.ProjectInput{Name: name, CategoryID: firstCategoryID(t, c)})
	if err != nil {
		t.Fatalf("CreateProject(%q): %v", name, err)
	}
	return p
}

// Adding a loose Task through the Project-view overlay persists it and lists it
// in body order.
func TestProjectViewAddTaskFlow(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "Ship it")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("a"))
	if !v.overlay.active() {
		t.Fatal("pressing a did not open the add-Task form")
	}
	for _, m := range typeString("draft the design") {
		v.Update(m)
	}
	runCmd(v.Update, v.Update(key("enter")))

	if v.overlay.active() {
		t.Error("form still open after a successful add")
	}
	tasks, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "draft the design" {
		t.Fatalf("tasks = %+v, want one titled %q", tasks, "draft the design")
	}
	if !strings.Contains(v.View(), "draft the design") {
		t.Errorf("view = %q, want the new Task listed", v.View())
	}
}

// Space toggles the selected Task's completion flag, and toggles it back.
func TestProjectViewToggleTaskDone(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "P")
	task, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "do the thing"})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	runCmd(v.Update, v.Update(key(" ")))
	if got := findTask(t, c, p.ID, task.ID); !got.Done {
		t.Errorf("Task Done = false after toggling, want true")
	}
	if !strings.Contains(v.View(), "[x] do the thing") {
		t.Errorf("view = %q, want the Task shown complete", v.View())
	}

	runCmd(v.Update, v.Update(key(" ")))
	if got := findTask(t, c, p.ID, task.ID); got.Done {
		t.Errorf("Task Done = true after toggling back, want false")
	}
}

// Editing a Task changes its title and sets an optional due date.
func TestProjectViewEditTaskTitleAndDueDate(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "P")
	task, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "old"})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("t"))
	if !v.overlay.active() {
		t.Fatal("pressing t did not open the edit-Task form")
	}
	for i := 0; i < len("old"); i++ {
		v.Update(key("backspace"))
	}
	for _, m := range typeString("new title") {
		v.Update(m)
	}
	v.Update(key("tab")) // move to the due-date field
	for _, m := range typeString("2026-09-01") {
		v.Update(m)
	}
	runCmd(v.Update, v.Update(key("enter")))

	got := findTask(t, c, p.ID, task.ID)
	if got.Title != "new title" {
		t.Errorf("Title = %q, want %q", got.Title, "new title")
	}
	want := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	if got.DueDate == nil || !got.DueDate.Equal(want) {
		t.Errorf("DueDate = %v, want %v", got.DueDate, want)
	}
}

// Editing a Task with a blank due-date field clears an existing due date.
func TestProjectViewEditTaskClearsDueDate(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "P")
	due := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	task, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "dated", DueDate: &due})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("t"))
	v.Update(key("tab")) // move to the due-date field
	for i := 0; i < len(taskDueDateLayout); i++ {
		v.Update(key("backspace")) // clear the pre-filled date
	}
	runCmd(v.Update, v.Update(key("enter")))

	if got := findTask(t, c, p.ID, task.ID); got.DueDate != nil {
		t.Errorf("DueDate = %v, want nil after clearing the field", got.DueDate)
	}
}

// A malformed due date never reaches core: the form stays open with a message.
func TestProjectViewAddTaskRejectsBadDueDate(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "P")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("a"))
	for _, m := range typeString("has a bad date") {
		v.Update(m)
	}
	v.Update(key("tab"))
	for _, m := range typeString("next tuesday") {
		v.Update(m)
	}
	runCmd(v.Update, v.Update(key("enter")))

	if !v.overlay.active() {
		t.Error("form closed on a bad due date, want it kept open")
	}
	if !strings.Contains(v.View(), "YYYY-MM-DD") {
		t.Errorf("view = %q, want the due-date format message", v.View())
	}
	tasks, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("tasks = %+v, want none persisted from a rejected add", tasks)
	}
}

// An empty title is surfaced from core as a message, not a panic.
func TestProjectViewAddTaskEmptyTitleSurfaced(t *testing.T) {
	c := newTestCore(t)
	p := mustProject(t, c, "P")

	v := newProjectView(c, p.ID)
	drainInit(v.Update, v.Init())

	v.Update(key("a"))
	runCmd(v.Update, v.Update(key("enter"))) // submit with no title

	if !v.overlay.active() {
		t.Error("form closed on an empty title, want it kept open")
	}
	if !strings.Contains(v.View(), "title must not be empty") {
		t.Errorf("view = %q, want the empty-title message", v.View())
	}
}

// The dashboard shows each Active Project's Next step, and nothing once every
// Task is done.
func TestDashboardShowsNextStep(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	p := mustProject(t, c, "waiting")
	a, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "first step"})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if _, err := c.AddTask(ctx, p.ID, core.TaskInput{Title: "second step"}); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	if step, ok := d.nextSteps[p.ID]; !ok || step.Title != "first step" {
		t.Errorf("nextSteps[%d] = (%+v, %v), want the first step", p.ID, step, ok)
	}
	if !strings.Contains(d.View(), "Next step: first step") {
		t.Errorf("view = %q, want the Next step shown", d.View())
	}

	// Complete every Task; the dashboard then shows no Next step.
	for _, id := range []int64{a.ID} {
		if _, err := c.SetTaskDone(ctx, id, true); err != nil {
			t.Fatalf("SetTaskDone: %v", err)
		}
	}
	if _, err := c.SetTaskDone(ctx, secondTaskID(t, c, p.ID), true); err != nil {
		t.Fatalf("SetTaskDone(second): %v", err)
	}
	drainInit(d.Update, d.reload())
	if _, ok := d.nextSteps[p.ID]; ok {
		t.Errorf("nextSteps[%d] present, want none once every Task is done", p.ID)
	}
	if strings.Contains(d.View(), "Next step:") {
		t.Errorf("view = %q, want no Next-step line", d.View())
	}
}

// findTask returns a Project's loose Task by id.
func findTask(t *testing.T, c *core.Core, projectID, taskID int64) core.Task {
	t.Helper()
	tasks, err := c.ProjectTasks(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	for _, tk := range tasks {
		if tk.ID == taskID {
			return tk
		}
	}
	t.Fatalf("task %d not found in project %d", taskID, projectID)
	return core.Task{}
}

// secondTaskID returns the id of a Project's second loose Task.
func secondTaskID(t *testing.T, c *core.Core, projectID int64) int64 {
	t.Helper()
	tasks, err := c.ProjectTasks(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if len(tasks) < 2 {
		t.Fatalf("want at least 2 Tasks, got %d", len(tasks))
	}
	return tasks[1].ID
}
