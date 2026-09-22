package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

// containsArchived reports whether entries holds ref.
func containsArchived(entries []core.ArchivedEntity, ref core.ArchiveRef) bool {
	for _, e := range entries {
		if e.Ref == ref {
			return true
		}
	}
	return false
}

func TestArchiveMilestoneCascadesToItsTasks(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "p", categoryID(t, c, "Other"))
	m := mustAddMilestone(t, c, p.ID, "m")
	task := mustAddMilestoneTask(t, c, m.ID, "mt")

	if err := c.ArchiveMilestone(ctx, m.ID); err != nil {
		t.Fatalf("ArchiveMilestone: %v", err)
	}

	if _, err := c.GetTask(ctx, task.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("GetTask(cascaded) error = %v, want ErrTaskNotFound", err)
	}
	tasks, err := c.MilestoneTasks(ctx, m.ID)
	if !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("MilestoneTasks(archived milestone) error = %v, want ErrMilestoneNotFound; tasks=%v", err, tasks)
	}
}

func TestArchiveMilestoneUnknownOrAlreadyArchivedErrors(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "p", categoryID(t, c, "Other"))
	m := mustAddMilestone(t, c, p.ID, "m")

	if err := c.ArchiveMilestone(ctx, 99999); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("ArchiveMilestone(unknown) = %v, want ErrMilestoneNotFound", err)
	}
	if err := c.ArchiveMilestone(ctx, m.ID); err != nil {
		t.Fatalf("first ArchiveMilestone: %v", err)
	}
	if err := c.ArchiveMilestone(ctx, m.ID); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("second ArchiveMilestone = %v, want ErrMilestoneNotFound", err)
	}
}

func TestArchiveTaskRemovesItAlone(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "p", categoryID(t, c, "Other"))
	keep := mustAddTask(t, c, p.ID, "keep")
	drop := mustAddTask(t, c, p.ID, "drop")

	if err := c.ArchiveTask(ctx, drop.ID); err != nil {
		t.Fatalf("ArchiveTask: %v", err)
	}
	if _, err := c.GetTask(ctx, drop.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("GetTask(archived) error = %v, want ErrTaskNotFound", err)
	}
	remaining, err := c.ProjectTasks(ctx, p.ID)
	if err != nil {
		t.Fatalf("ProjectTasks: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != keep.ID {
		t.Errorf("ProjectTasks after archive = %+v, want only %v", remaining, keep.ID)
	}
}

func TestArchiveProjectCascadesToMilestonesAndTasks(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "p", categoryID(t, c, "Other"))
	looseTask := mustAddTask(t, c, p.ID, "loose")
	m := mustAddMilestone(t, c, p.ID, "m")
	mTask := mustAddMilestoneTask(t, c, m.ID, "mt")

	if err := c.ArchiveProject(ctx, p.ID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}

	if _, err := c.GetTask(ctx, looseTask.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("GetTask(loose, cascaded) error = %v, want ErrTaskNotFound", err)
	}
	if _, err := c.GetTask(ctx, mTask.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("GetTask(milestone task, cascaded) error = %v, want ErrTaskNotFound", err)
	}
	if _, err := c.GetMilestone(ctx, m.ID); !errors.Is(err, core.ErrMilestoneNotFound) {
		t.Errorf("GetMilestone(cascaded) error = %v, want ErrMilestoneNotFound", err)
	}
}

func TestArchivedEntitiesExcludedFromDashboardAndDoNext(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	p := mustCreateProject(t, c, "p", categoryID(t, c, "Other"))
	mustAddTask(t, c, p.ID, "t1")

	if err := c.ArchiveProject(ctx, p.ID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}

	dash, err := c.Dashboard(ctx)
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if len(dash) != 0 {
		t.Errorf("Dashboard after archive = %+v, want empty", dash)
	}
	if _, ok, err := c.DoNext(ctx); err != nil || ok {
		t.Errorf("DoNext after archive = (ok=%v, err=%v), want (false, nil)", ok, err)
	}
}

func TestListArchivedCoversAllFourKinds(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")

	p := mustCreateProject(t, c, "p", other)
	task := mustAddTask(t, c, p.ID, "t")
	m := mustAddMilestone(t, c, p.ID, "m")
	idea, err := c.CreateIdea(ctx, core.IdeaInput{Name: "i", CategoryID: other})
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	if err := c.ArchiveTask(ctx, task.ID); err != nil {
		t.Fatalf("ArchiveTask: %v", err)
	}
	if err := c.ArchiveMilestone(ctx, m.ID); err != nil {
		t.Fatalf("ArchiveMilestone: %v", err)
	}
	if err := c.DeleteIdea(ctx, idea.ID); err != nil {
		t.Fatalf("DeleteIdea: %v", err)
	}
	if err := c.ArchiveProject(ctx, p.ID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}

	archived, err := c.ListArchived(ctx)
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	wantRefs := []core.ArchiveRef{
		{Kind: core.ArchivedProject, ID: p.ID},
		{Kind: core.ArchivedMilestone, ID: m.ID},
		{Kind: core.ArchivedTask, ID: task.ID},
		{Kind: core.ArchivedIdea, ID: idea.ID},
	}
	for _, ref := range wantRefs {
		if !containsArchived(archived, ref) {
			t.Errorf("ListArchived = %+v, missing %v", archived, ref)
		}
	}
}

func TestRestoreArchivedRestoresExactlyTheCascade(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")
	p := mustCreateProject(t, c, "p", other)
	independent := mustAddTask(t, c, p.ID, "independent")
	sibling := mustAddTask(t, c, p.ID, "sibling")
	m := mustAddMilestone(t, c, p.ID, "m")
	mTask := mustAddMilestoneTask(t, c, m.ID, "mt")

	// Archived on its own, before the Project's cascade.
	if err := c.ArchiveTask(ctx, independent.ID); err != nil {
		t.Fatalf("ArchiveTask(independent): %v", err)
	}
	if err := c.ArchiveProject(ctx, p.ID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}

	if err := c.RestoreArchived(ctx, core.ArchiveRef{Kind: core.ArchivedProject, ID: p.ID}); err != nil {
		t.Fatalf("RestoreArchived: %v", err)
	}

	if _, err := c.GetProject(ctx, p.ID); err != nil {
		t.Errorf("GetProject(restored) error = %v, want nil", err)
	}
	if _, err := c.GetMilestone(ctx, m.ID); err != nil {
		t.Errorf("GetMilestone(restored) error = %v, want nil", err)
	}
	if _, err := c.GetTask(ctx, sibling.ID); err != nil {
		t.Errorf("GetTask(restored sibling) error = %v, want nil", err)
	}
	if _, err := c.GetTask(ctx, mTask.ID); err != nil {
		t.Errorf("GetTask(restored milestone task) error = %v, want nil", err)
	}
	// The independently-archived Task must NOT have come back.
	if _, err := c.GetTask(ctx, independent.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("GetTask(independent) error = %v, want ErrTaskNotFound (must stay archived)", err)
	}
}

func TestRestoreArchivedTaskArchivedAfterAnEarlierCascadeStaysArchived(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")
	p := mustCreateProject(t, c, "p", other)
	t1 := mustAddTask(t, c, p.ID, "t1")
	t2 := mustAddTask(t, c, p.ID, "t2")

	// First cascade, then restore: both tasks come back live.
	if err := c.ArchiveProject(ctx, p.ID); err != nil {
		t.Fatalf("ArchiveProject (1st): %v", err)
	}
	if err := c.RestoreArchived(ctx, core.ArchiveRef{Kind: core.ArchivedProject, ID: p.ID}); err != nil {
		t.Fatalf("RestoreArchived (1st): %v", err)
	}

	// Now archive t1 on its own, then cascade-archive the Project again.
	if err := c.ArchiveTask(ctx, t1.ID); err != nil {
		t.Fatalf("ArchiveTask(t1): %v", err)
	}
	if err := c.ArchiveProject(ctx, p.ID); err != nil {
		t.Fatalf("ArchiveProject (2nd): %v", err)
	}

	if err := c.RestoreArchived(ctx, core.ArchiveRef{Kind: core.ArchivedProject, ID: p.ID}); err != nil {
		t.Fatalf("RestoreArchived (2nd): %v", err)
	}

	if _, err := c.GetTask(ctx, t2.ID); err != nil {
		t.Errorf("GetTask(t2, restored) error = %v, want nil", err)
	}
	if _, err := c.GetTask(ctx, t1.ID); !errors.Is(err, core.ErrTaskNotFound) {
		t.Errorf("GetTask(t1) error = %v, want ErrTaskNotFound (archived independently, must stay archived)", err)
	}
}

func TestRestoreArchivedUnknownErrors(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	if err := c.RestoreArchived(ctx, core.ArchiveRef{Kind: core.ArchivedProject, ID: 99999}); !errors.Is(err, core.ErrArchivedNotFound) {
		t.Errorf("RestoreArchived(unknown) = %v, want ErrArchivedNotFound", err)
	}
}

func TestPurgeArchivedRemovesRowsPermanently(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	other := categoryID(t, c, "Other")
	p := mustCreateProject(t, c, "p", other)
	task := mustAddTask(t, c, p.ID, "t")
	m := mustAddMilestone(t, c, p.ID, "m")
	mTask := mustAddMilestoneTask(t, c, m.ID, "mt")

	if err := c.ArchiveProject(ctx, p.ID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	ref := core.ArchiveRef{Kind: core.ArchivedProject, ID: p.ID}
	if err := c.PurgeArchived(ctx, ref); err != nil {
		t.Fatalf("PurgeArchived: %v", err)
	}

	// Purged rows are gone for good: even restore no longer finds them.
	if err := c.RestoreArchived(ctx, ref); !errors.Is(err, core.ErrArchivedNotFound) {
		t.Errorf("RestoreArchived(purged) = %v, want ErrArchivedNotFound", err)
	}
	archived, err := c.ListArchived(ctx)
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	for _, gone := range []core.ArchiveRef{
		ref,
		{Kind: core.ArchivedMilestone, ID: m.ID},
		{Kind: core.ArchivedTask, ID: task.ID},
		{Kind: core.ArchivedTask, ID: mTask.ID},
	} {
		if containsArchived(archived, gone) {
			t.Errorf("ListArchived after purge = %+v, still contains %v", archived, gone)
		}
	}
}

func TestPurgeArchivedUnknownErrors(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	if err := c.PurgeArchived(ctx, core.ArchiveRef{Kind: core.ArchivedTask, ID: 99999}); !errors.Is(err, core.ErrArchivedNotFound) {
		t.Errorf("PurgeArchived(unknown) = %v, want ErrArchivedNotFound", err)
	}
}

func TestPurgeArchivedIdeaAlone(t *testing.T) {
	c, _ := newTestCore(t)
	ctx := context.Background()
	idea, err := c.CreateIdea(ctx, core.IdeaInput{Name: "i", CategoryID: categoryID(t, c, "Other")})
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}
	if err := c.DeleteIdea(ctx, idea.ID); err != nil {
		t.Fatalf("DeleteIdea: %v", err)
	}
	ref := core.ArchiveRef{Kind: core.ArchivedIdea, ID: idea.ID}
	if err := c.PurgeArchived(ctx, ref); err != nil {
		t.Fatalf("PurgeArchived: %v", err)
	}
	archived, err := c.ListArchived(ctx)
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if containsArchived(archived, ref) {
		t.Errorf("ListArchived after purge = %+v, still contains %v", archived, ref)
	}
}
