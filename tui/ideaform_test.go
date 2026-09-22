package tui

import (
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestIdeaFormStartsBlankOnTheFirstCategory(t *testing.T) {
	f := newIdeaForm("New Idea", testCategories(), nil)

	in := f.input()
	if in.Name != "" || in.Description != "" || in.Notes != "" {
		t.Errorf("blank form input = %+v, want every field empty", in)
	}
	if in.CategoryID != 10 {
		t.Errorf("CategoryID = %d, want the first Category (10)", in.CategoryID)
	}
}

func TestIdeaFormEditModePreFillsEveryField(t *testing.T) {
	f := newIdeaForm("Edit Idea", testCategories(), &core.Idea{
		Name:        "Learn pottery",
		Description: "wheel throwing",
		Notes:       "check local studios",
		CategoryID:  20,
	})

	in := f.input()
	if in.Name != "Learn pottery" || in.Description != "wheel throwing" || in.Notes != "check local studios" {
		t.Errorf("edit form input = %+v, want the Idea's fields", in)
	}
	if in.CategoryID != 20 {
		t.Errorf("CategoryID = %d, want the Idea's Category (20)", in.CategoryID)
	}
	if !strings.Contains(f.render(), "Edit Idea") {
		t.Errorf("render = %q, want the edit title", f.render())
	}
}

func TestIdeaFormTypingLandsInTheFocusedFieldIncludingNotes(t *testing.T) {
	f := newIdeaForm("New Idea", testCategories(), nil)

	for _, m := range typeString("Learn pottery") {
		f.update(m)
	}
	f.update(key("tab")) // -> description
	for _, m := range typeString("wheel throwing") {
		f.update(m)
	}
	f.update(key("tab")) // -> category
	f.update(key("right"))
	f.update(key("tab")) // -> notes
	for _, m := range typeString("check local studios") {
		f.update(m)
	}

	in := f.input()
	if in.Name != "Learn pottery" {
		t.Errorf("Name = %q, want %q", in.Name, "Learn pottery")
	}
	if in.Description != "wheel throwing" {
		t.Errorf("Description = %q, want %q", in.Description, "wheel throwing")
	}
	if in.Notes != "check local studios" {
		t.Errorf("Notes = %q, want %q", in.Notes, "check local studios")
	}
	if in.CategoryID != 20 {
		t.Errorf("CategoryID = %d, want the second Category (20)", in.CategoryID)
	}
}

func TestIdeaFormEscCancelsAndEnterSubmits(t *testing.T) {
	f := newIdeaForm("New Idea", testCategories(), nil)

	if done, submitted := f.update(key("esc")); !done || submitted {
		t.Errorf("esc: done=%v submitted=%v, want done=true submitted=false", done, submitted)
	}

	f = newIdeaForm("New Idea", testCategories(), nil)
	if done, submitted := f.update(key("enter")); !done || !submitted {
		t.Errorf("enter: done=%v submitted=%v, want done=true submitted=true", done, submitted)
	}
}
