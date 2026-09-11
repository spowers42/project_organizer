package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/spowers42/project_organizer/core"
)

func TestDoNextEmptyPoolIsNotAnError(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()

	if _, ok, err := c.DoNext(ctx); err != nil {
		t.Fatalf("DoNext: %v", err)
	} else if ok {
		t.Errorf("ok = true with no Active Project, want false")
	}

	// A Project with no incomplete Task (no loose Tasks, no Milestones) still
	// leaves the pool empty.
	catID := categoryID(t, c, "Other")
	mustCreateProject(t, c, "Empty", catID)
	if _, ok, err := c.DoNext(ctx); err != nil {
		t.Fatalf("DoNext: %v", err)
	} else if ok {
		t.Errorf("ok = true for a Project with no Next step, want false")
	}
}

func TestDoNextExcludesNonActiveAndNoNextStepProjects(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	catID := categoryID(t, c, "Other")

	active := mustCreateProject(t, c, "Active one", catID)
	want := mustAddTask(t, c, active.ID, "the only candidate")

	paused := mustCreateProject(t, c, "Paused one", catID)
	mustAddTask(t, c, paused.ID, "should never be picked")
	if _, err := c.SetProjectLifecycle(ctx, paused.ID, core.Paused); err != nil {
		t.Fatalf("SetProjectLifecycle: %v", err)
	}

	mustCreateProject(t, c, "No Next step", catID) // Active, but empty body

	for i := 0; i < 50; i++ {
		cand, ok, err := c.DoNext(ctx)
		if err != nil {
			t.Fatalf("DoNext: %v", err)
		}
		if !ok {
			t.Fatalf("ok = false, want a pick from the single candidate")
		}
		if cand.Project.ID != active.ID || cand.NextStep.ID != want.ID {
			t.Fatalf("DoNext picked %+v, want the sole candidate from %q", cand, active.Name)
		}
	}
}

func TestDoNextIsReproducibleUnderAFixedSeedAndClock(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	catID := categoryID(t, c, "Other")
	for i := 0; i < 5; i++ {
		p := mustCreateProject(t, c, "P", catID)
		mustAddTask(t, c, p.ID, "task")
	}

	first, ok, err := c.DoNext(ctx)
	if err != nil || !ok {
		t.Fatalf("DoNext: cand=%+v ok=%v err=%v", first, ok, err)
	}

	// A fresh Core wired to the same seed and the same clock reading, over the
	// same pool, reproduces the same pick.
	c2, clock2 := newTestCore(t)
	clock2.now = fixedNow
	for i := 0; i < 5; i++ {
		p := mustCreateProject(t, c2, "P", catID)
		mustAddTask(t, c2, p.ID, "task")
	}
	second, ok, err := c2.DoNext(ctx)
	if err != nil || !ok {
		t.Fatalf("DoNext (second Core): cand=%+v ok=%v err=%v", second, ok, err)
	}

	if first.Project.ID != second.Project.ID || first.NextStep.ID != second.NextStep.ID {
		t.Errorf("DoNext picked Project %d/Task %d then Project %d/Task %d under the same seed and clock, want the same pick",
			first.Project.ID, first.NextStep.ID, second.Project.ID, second.NextStep.ID)
	}
}

func TestDoNextRerollDrawsAgainFromTheSamePool(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	catID := categoryID(t, c, "Other")
	ids := map[int64]bool{}
	for i := 0; i < 6; i++ {
		p := mustCreateProject(t, c, "P", catID)
		task := mustAddTask(t, c, p.ID, "task")
		ids[task.ID] = true
	}

	seen := map[int64]bool{}
	for i := 0; i < 100; i++ {
		cand, ok, err := c.DoNext(ctx)
		if err != nil || !ok {
			t.Fatalf("DoNext: cand=%+v ok=%v err=%v", cand, ok, err)
		}
		if !ids[cand.NextStep.ID] {
			t.Fatalf("DoNext picked Task %d, not in the pool", cand.NextStep.ID)
		}
		seen[cand.NextStep.ID] = true
	}
	if len(seen) < 2 {
		t.Errorf("100 rerolls over %d equal-weight candidates only ever picked %d, want more variety", len(ids), len(seen))
	}
}

// TestDoNextWeightingBiasesTowardOverdueAndPriority draws many times from a
// pool with one heavily-weighted candidate (overdue and Priority, weight
// 4*2=8) among plain unweighted ones (weight 1) and checks it comes up
// disproportionately often — pinning the v1 formula from the issue.
func TestDoNextWeightingBiasesTowardOverdueAndPriority(t *testing.T) {
	c, clock := newTestCore(t)
	ctx := context.Background()
	catID := categoryID(t, c, "Other")

	heavy := mustCreateProject(t, c, "Heavy", catID)
	heavyTask := mustAddTask(t, c, heavy.ID, "overdue and starred")
	overdue := clock.now.Add(-24 * time.Hour)
	if _, err := c.EditTask(ctx, heavyTask.ID, core.TaskInput{Title: heavyTask.Title, DueDate: &overdue}); err != nil {
		t.Fatalf("EditTask: %v", err)
	}
	if _, err := c.SetTaskPriority(ctx, heavyTask.ID, true); err != nil {
		t.Fatalf("SetTaskPriority: %v", err)
	}

	const plainCount = 7
	for i := 0; i < plainCount; i++ {
		p := mustCreateProject(t, c, "Plain", catID)
		mustAddTask(t, c, p.ID, "no due date, no Priority")
	}

	const trials = 4000
	heavyPicks := 0
	for i := 0; i < trials; i++ {
		cand, ok, err := c.DoNext(ctx)
		if err != nil || !ok {
			t.Fatalf("DoNext: cand=%+v ok=%v err=%v", cand, ok, err)
		}
		if cand.Project.ID == heavy.ID {
			heavyPicks++
		}
	}

	// Expected share: weight 8 out of (8 + 7*1) = 15 -> ~53.3%. A plain
	// candidate's expected share is ~6.7%. Assert the heavy candidate clears
	// well past any single plain candidate's share, with slack for variance.
	got := float64(heavyPicks) / trials
	if got < 0.35 {
		t.Errorf("heavy candidate picked %.1f%% of %d trials, want well above the ~6.7%% a plain candidate gets (formula: overdue*Priority weight 8 vs weight 1)", got*100, trials)
	}
}
