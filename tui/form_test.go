package tui

import (
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func testCategories() []core.Category {
	return []core.Category{
		{ID: 10, Name: "Programming"},
		{ID: 20, Name: "Course"},
		{ID: 30, Name: "Other"},
	}
}

func TestProjectFormCreateModeStartsBlankOnTheFirstCategory(t *testing.T) {
	f := newProjectForm("New Project", testCategories(), nil)

	in := f.input()
	if in.Name != "" || in.Description != "" {
		t.Errorf("blank form input = %+v, want empty name and description", in)
	}
	if in.CategoryID != 10 {
		t.Errorf("CategoryID = %d, want the first Category (10)", in.CategoryID)
	}
}

func TestProjectFormEditModePreFillsEveryField(t *testing.T) {
	f := newProjectForm("Edit Project", testCategories(), &core.Project{
		Name:        "Rewrite the parser",
		Description: "switch to a Pratt parser",
		CategoryID:  20,
	})

	in := f.input()
	if in.Name != "Rewrite the parser" || in.Description != "switch to a Pratt parser" {
		t.Errorf("edit form input = %+v, want the Project's fields", in)
	}
	if in.CategoryID != 20 {
		t.Errorf("CategoryID = %d, want the Project's Category (20)", in.CategoryID)
	}
	if !strings.Contains(f.render(), "Edit Project") {
		t.Errorf("render = %q, want the edit title", f.render())
	}
}

func TestProjectFormTypingLandsInTheFocusedField(t *testing.T) {
	f := newProjectForm("New Project", testCategories(), nil)

	for _, m := range typeString("Parser") {
		f.update(m)
	}
	f.update(key("tab")) // -> description
	for _, m := range typeString("notes") {
		f.update(m)
	}

	in := f.input()
	if in.Name != "Parser" {
		t.Errorf("Name = %q, want %q", in.Name, "Parser")
	}
	if in.Description != "notes" {
		t.Errorf("Description = %q, want %q", in.Description, "notes")
	}
}

func TestProjectFormFocusCyclesAndWraps(t *testing.T) {
	f := newProjectForm("New Project", testCategories(), nil)
	if f.focus != fieldName {
		t.Fatalf("initial focus = %d, want fieldName", f.focus)
	}
	f.update(key("tab"))
	f.update(key("tab"))
	if f.focus != fieldCategory {
		t.Errorf("after two tabs focus = %d, want fieldCategory", f.focus)
	}
	f.update(key("tab"))
	if f.focus != fieldName {
		t.Errorf("focus after wrap = %d, want fieldName", f.focus)
	}
	f.update(key("shift+tab"))
	if f.focus != fieldCategory {
		t.Errorf("shift+tab from first field = %d, want fieldCategory (wrap back)", f.focus)
	}
}

func TestProjectFormCategoryChoiceChangesTheInput(t *testing.T) {
	f := newProjectForm("New Project", testCategories(), nil)
	f.update(key("tab"))
	f.update(key("tab")) // focus Category
	f.update(key("right"))
	f.update(key("right")) // Programming -> Course -> Other

	if got := f.input().CategoryID; got != 30 {
		t.Errorf("CategoryID after two right moves = %d, want 30 (Other)", got)
	}
}

func TestProjectFormSubmitAndCancelReport(t *testing.T) {
	f := newProjectForm("New Project", testCategories(), nil)
	done, submitted := f.update(key("enter"))
	if !done || !submitted {
		t.Errorf("enter => done=%v submitted=%v, want true/true", done, submitted)
	}

	f = newProjectForm("New Project", testCategories(), nil)
	done, submitted = f.update(key("esc"))
	if !done || submitted {
		t.Errorf("esc => done=%v submitted=%v, want true/false", done, submitted)
	}
}

func TestProjectFormBackspaceEditsTheFocusedField(t *testing.T) {
	f := newProjectForm("Edit Project", testCategories(), &core.Project{Name: "abcd", CategoryID: 10})
	f.update(key("backspace"))
	f.update(key("backspace"))
	if got := f.input().Name; got != "ab" {
		t.Errorf("Name after two backspaces = %q, want %q", got, "ab")
	}
}
