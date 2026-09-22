package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestCreateCategoryAddsToList(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)

	cat, err := c.CreateCategory(ctx, "Woodworking")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if cat.ID == 0 || cat.Name != "Woodworking" {
		t.Errorf("CreateCategory = %+v, want a non-zero id and the given name", cat)
	}

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	var found bool
	for _, got := range cats {
		if got.ID == cat.ID && got.Name == "Woodworking" {
			found = true
		}
	}
	if !found {
		t.Errorf("ListCategories = %+v, want it to include the created Category", cats)
	}
}

func TestCreateCategoryRejectsEmptyName(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)

	if _, err := c.CreateCategory(ctx, "   "); !errors.Is(err, core.ErrEmptyCategoryName) {
		t.Errorf("err = %v, want ErrEmptyCategoryName", err)
	}
}

func TestRenameCategoryUpdatesNameWithoutTouchingReferences(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	project := mustCreateProject(t, c, "Some app", catID)

	renamed, err := c.RenameCategory(ctx, catID, "Software")
	if err != nil {
		t.Fatalf("RenameCategory: %v", err)
	}
	if renamed.ID != catID || renamed.Name != "Software" {
		t.Errorf("RenameCategory = %+v, want id %d and name %q", renamed, catID, "Software")
	}

	got, err := c.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.CategoryID != catID {
		t.Errorf("project.CategoryID = %d after rename, want unchanged %d", got.CategoryID, catID)
	}

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	var names []string
	for _, cat := range cats {
		names = append(names, cat.Name)
	}
	var sawRenamed, sawOld bool
	for _, n := range names {
		if n == "Software" {
			sawRenamed = true
		}
		if n == "Programming" {
			sawOld = true
		}
	}
	if !sawRenamed || sawOld {
		t.Errorf("category names = %v, want Software present and Programming gone", names)
	}
}

func TestRenameCategoryRejectsEmptyName(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	if _, err := c.RenameCategory(ctx, catID, "  "); !errors.Is(err, core.ErrEmptyCategoryName) {
		t.Errorf("err = %v, want ErrEmptyCategoryName", err)
	}
}

func TestRenameCategoryUnknownCategoryIsNotFound(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)

	if _, err := c.RenameCategory(ctx, 99999, "New name"); !errors.Is(err, core.ErrCategoryNotFound) {
		t.Errorf("err = %v, want ErrCategoryNotFound", err)
	}
}

func TestDeleteCategoryRejectsWhenReferencedByProject(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")
	mustCreateProject(t, c, "Some app", catID)

	if err := c.DeleteCategory(ctx, catID); !errors.Is(err, core.ErrCategoryInUse) {
		t.Errorf("err = %v, want ErrCategoryInUse", err)
	}

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	var stillThere bool
	for _, cat := range cats {
		if cat.ID == catID {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("category %d missing after rejected delete, want it to remain", catID)
	}
}

func TestDeleteCategoryRejectsWhenReferencedByArchivedProject(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")
	project := mustCreateProject(t, c, "Some app", catID)

	if err := c.ArchiveProject(ctx, project.ID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}

	if err := c.DeleteCategory(ctx, catID); !errors.Is(err, core.ErrCategoryInUse) {
		t.Errorf("err = %v, want ErrCategoryInUse for a Category referenced by an archived Project", err)
	}
}

func TestDeleteCategoryRejectsWhenReferencedByIdea(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	if _, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Learn woodworking", CategoryID: catID}); err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	if err := c.DeleteCategory(ctx, catID); !errors.Is(err, core.ErrCategoryInUse) {
		t.Errorf("err = %v, want ErrCategoryInUse", err)
	}
}

func TestDeleteCategoryRejectsWhenReferencedByArchivedIdea(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)
	catID := categoryID(t, c, "Programming")

	idea, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Learn woodworking", CategoryID: catID})
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}
	if err := c.DeleteIdea(ctx, idea.ID); err != nil {
		t.Fatalf("DeleteIdea: %v", err)
	}

	if err := c.DeleteCategory(ctx, catID); !errors.Is(err, core.ErrCategoryInUse) {
		t.Errorf("err = %v, want ErrCategoryInUse for a Category referenced by an archived Idea", err)
	}
}

func TestDeleteCategorySucceedsWhenUnreferenced(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)

	cat, err := c.CreateCategory(ctx, "Unused")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	if err := c.DeleteCategory(ctx, cat.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	for _, got := range cats {
		if got.ID == cat.ID {
			t.Errorf("ListCategories = %+v, want the deleted Category gone", cats)
		}
	}
}

func TestDeleteCategoryUnknownCategoryIsNotFound(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestCore(t)

	if err := c.DeleteCategory(ctx, 99999); !errors.Is(err, core.ErrCategoryNotFound) {
		t.Errorf("err = %v, want ErrCategoryNotFound", err)
	}
}
