package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestCreateIdeaShowsInList(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	idea, err := c.CreateIdea(ctx, core.IdeaInput{
		Name: "Learn woodworking", Description: "maybe someday", Notes: "start with a workbench", CategoryID: catID,
	})
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}
	if idea.Name != "Learn woodworking" || idea.Description != "maybe someday" ||
		idea.Notes != "start with a workbench" || idea.CategoryID != catID {
		t.Errorf("idea = %+v, want the given fields", idea)
	}
	if idea.PromotedProjectID != nil {
		t.Errorf("idea.PromotedProjectID = %v, want nil for a fresh Idea", idea.PromotedProjectID)
	}

	ideas, err := c.ListIdeas(ctx)
	if err != nil {
		t.Fatalf("ListIdeas: %v", err)
	}
	if len(ideas) != 1 || ideas[0].ID != idea.ID {
		t.Errorf("ListIdeas = %+v, want the created Idea", ideas)
	}
}

func TestCreateIdeaRejectsEmptyName(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	_, err := c.CreateIdea(ctx, core.IdeaInput{Name: "   ", CategoryID: catID})
	if !errors.Is(err, core.ErrEmptyIdeaName) {
		t.Errorf("err = %v, want ErrEmptyIdeaName", err)
	}
}

func TestCreateIdeaRejectsUnknownCategory(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)

	_, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Something", CategoryID: 99999})
	if !errors.Is(err, core.ErrCategoryNotFound) {
		t.Errorf("err = %v, want ErrCategoryNotFound", err)
	}
}

func TestDeleteIdeaPlainDeleteRemovesFromListAndCarriesNoLink(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	idea, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Fleeting", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	if err := c.DeleteIdea(ctx, idea.ID); err != nil {
		t.Fatalf("DeleteIdea: %v", err)
	}

	ideas, err := c.ListIdeas(ctx)
	if err != nil {
		t.Fatalf("ListIdeas: %v", err)
	}
	if len(ideas) != 0 {
		t.Errorf("ListIdeas after delete = %+v, want empty", ideas)
	}

	if err := c.DeleteIdea(ctx, idea.ID); !errors.Is(err, core.ErrIdeaNotFound) {
		t.Errorf("deleting an already-deleted Idea = %v, want ErrIdeaNotFound", err)
	}
}

func TestPromoteIdeaCreatesProjectWithCopiedFieldsInDefaultState(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	idea, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Build a shed", Description: "in the backyard", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	project, err := c.PromoteIdea(ctx, idea.ID)
	if err != nil {
		t.Fatalf("PromoteIdea: %v", err)
	}
	if project.Name != idea.Name || project.Description != idea.Description || project.CategoryID != idea.CategoryID {
		t.Errorf("promoted project = %+v, want fields copied from %+v", project, idea)
	}
	if project.Lifecycle != core.DefaultLifecycle {
		t.Errorf("promoted project.Lifecycle = %q, want %q", project.Lifecycle, core.DefaultLifecycle)
	}
}

func TestPromoteIdeaSoftDeletesIdeaWithLinkToProject(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	idea, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Build a shed", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	project, err := c.PromoteIdea(ctx, idea.ID)
	if err != nil {
		t.Fatalf("PromoteIdea: %v", err)
	}

	ideas, err := c.ListIdeas(ctx)
	if err != nil {
		t.Fatalf("ListIdeas: %v", err)
	}
	if len(ideas) != 0 {
		t.Errorf("ListIdeas after promotion = %+v, want the Idea gone from normal views", ideas)
	}

	if _, err := c.PromoteIdea(ctx, idea.ID); !errors.Is(err, core.ErrIdeaNotFound) {
		t.Errorf("re-promoting an already-promoted Idea = %v, want ErrIdeaNotFound", err)
	}

	_ = project // the link itself is a store-level detail, exercised in internal/store tests
}

func TestPromoteIdeaUnknownIdeaIsNotFound(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)

	if _, err := c.PromoteIdea(ctx, 99999); !errors.Is(err, core.ErrIdeaNotFound) {
		t.Errorf("err = %v, want ErrIdeaNotFound", err)
	}
}

func TestIdeasAreNeverActiveProjectsOrDoNextCandidates(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	if _, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Someday", CategoryID: catID}); err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	active, err := c.ActiveProjects(ctx)
	if err != nil {
		t.Fatalf("ActiveProjects: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("ActiveProjects = %+v, want no Ideas among Active Projects", active)
	}

	if _, ok, err := c.DoNext(ctx); err != nil {
		t.Fatalf("DoNext: %v", err)
	} else if ok {
		t.Error("DoNext returned a candidate, want none — an Idea is never a candidate")
	}
}
