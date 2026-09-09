package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestSetTaskPriorityTogglesOnAndOff(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Programming"))
	task := mustAddTask(t, c, p.ID, "starrable")

	if task.Priority {
		t.Fatalf("a new Task starts Priority = true, want false")
	}

	starred, err := c.SetTaskPriority(ctx, task.ID, true)
	if err != nil {
		t.Fatalf("SetTaskPriority(true): %v", err)
	}
	if !starred.Priority {
		t.Errorf("Priority = false after starring, want true")
	}

	// Setting it to the value it already holds is not an error.
	if _, err := c.SetTaskPriority(ctx, task.ID, true); err != nil {
		t.Errorf("SetTaskPriority(true) again: %v, want nil", err)
	}

	// The star survives a round-trip through the body listing.
	tasks, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if len(tasks) != 1 || !tasks[0].Priority {
		t.Errorf("persisted Priority = %v, want the star set", tasks)
	}

	cleared, err := c.SetTaskPriority(ctx, task.ID, false)
	if err != nil {
		t.Fatalf("SetTaskPriority(false): %v", err)
	}
	if cleared.Priority {
		t.Errorf("Priority = true after un-starring, want false")
	}
}

func TestSetTaskPriorityWorksInsideAMilestone(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))
	m := mustAddMilestone(t, c, p.ID, "M")
	task := mustAddMilestoneTask(t, c, m.ID, "inner")

	if _, err := c.SetTaskPriority(ctx, task.ID, true); err != nil {
		t.Fatalf("SetTaskPriority(true): %v", err)
	}

	tasks, err := c.MilestoneTasks(ctx, m.ID)
	if err != nil {
		t.Fatalf("MilestoneTasks: %v", err)
	}
	if len(tasks) != 1 || !tasks[0].Priority {
		t.Errorf("persisted Priority = %v, want the star set on the Milestone Task", tasks)
	}
}

func TestSetTaskPriorityUnknownIDErrors(t *testing.T) {
	c, _ := newTestCore(t)

	if _, err := c.SetTaskPriority(context.Background(), 4242, true); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("error = %v, want ErrTaskNotFound", err)
	}
}

func TestSetProjectPriorityTogglesOnAndOff(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "starrable", categoryID(t, c, "Programming"))

	if p.Priority {
		t.Fatalf("a new Project starts Priority = true, want false")
	}

	starred, err := c.SetProjectPriority(ctx, p.ID, true)
	if err != nil {
		t.Fatalf("SetProjectPriority(true): %v", err)
	}
	if !starred.Priority {
		t.Errorf("Priority = false after starring, want true")
	}

	got, err := c.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if !got.Priority {
		t.Errorf("persisted Priority = false, want the star set")
	}

	cleared, err := c.SetProjectPriority(ctx, p.ID, false)
	if err != nil {
		t.Fatalf("SetProjectPriority(false): %v", err)
	}
	if cleared.Priority {
		t.Errorf("Priority = true after un-starring, want false")
	}
}

func TestSetProjectPriorityUnknownIDErrors(t *testing.T) {
	c, _ := newTestCore(t)

	if _, err := c.SetProjectPriority(context.Background(), 999, true); !errors.Is(err, core.ErrProjectNotFound) {
		t.Errorf("error = %v, want ErrProjectNotFound", err)
	}
}

func TestProjectsByPriorityIsStablePartition(t *testing.T) {
	in := []core.Project{
		{ID: 1, Name: "a"},
		{ID: 2, Name: "b", Priority: true},
		{ID: 3, Name: "c"},
		{ID: 4, Name: "d", Priority: true},
		{ID: 5, Name: "e"},
	}
	want := []string{"b", "d", "a", "c", "e"}

	got := core.ProjectsByPriority(in)
	if !equalStrings(projectSliceNames(got), want) {
		t.Errorf("ProjectsByPriority order = %v, want %v (starred first, stable)", projectSliceNames(got), want)
	}
	// The input slice is a view input, not a target: it must be left alone.
	if !equalStrings(projectSliceNames(in), []string{"a", "b", "c", "d", "e"}) {
		t.Errorf("input reordered to %v, want it untouched", projectSliceNames(in))
	}
}

func TestTasksByPriorityIsStablePartitionAndLeavesBodyOrderAlone(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "P", categoryID(t, c, "Other"))
	mustAddTask(t, c, p.ID, "one")
	two := mustAddTask(t, c, p.ID, "two")
	mustAddTask(t, c, p.ID, "three")
	four := mustAddTask(t, c, p.ID, "four")

	for _, id := range []int64{two.ID, four.ID} {
		if _, err := c.SetTaskPriority(ctx, id, true); err != nil {
			t.Fatalf("SetTaskPriority(%d): %v", id, err)
		}
	}

	tasks, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	// Stored body order is unchanged by the stars (ADR 0001).
	if !equalStrings(taskTitles(tasks), []string{"one", "two", "three", "four"}) {
		t.Fatalf("stored body order = %v, want it unchanged", taskTitles(tasks))
	}

	sorted := core.TasksByPriority(tasks)
	if !equalStrings(taskTitles(sorted), []string{"two", "four", "one", "three"}) {
		t.Errorf("TasksByPriority order = %v, want starred first, stable", taskTitles(sorted))
	}
	// The sort returns a new slice; the caller's list keeps body order.
	if !equalStrings(taskTitles(tasks), []string{"one", "two", "three", "four"}) {
		t.Errorf("TasksByPriority mutated its input to %v", taskTitles(tasks))
	}
}

// projectSliceNames is the ordered list of names from a Project slice.
func projectSliceNames(ps []core.Project) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}
