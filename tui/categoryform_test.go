package tui

import (
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestCategoryFormCreateModeStartsBlank(t *testing.T) {
	f := newCategoryForm("New Category", nil)

	if f.value() != "" {
		t.Errorf("value() = %q, want empty", f.value())
	}
	if !strings.Contains(f.render(), "New Category") {
		t.Errorf("render = %q, want the create title", f.render())
	}
}

func TestCategoryFormRenameModePreFillsName(t *testing.T) {
	f := newCategoryForm("Rename Category", &core.Category{ID: 20, Name: "Course"})

	if f.value() != "Course" {
		t.Errorf("value() = %q, want %q", f.value(), "Course")
	}
	if !strings.Contains(f.render(), "Rename Category") {
		t.Errorf("render = %q, want the rename title", f.render())
	}
	if !strings.Contains(f.render(), "Course") {
		t.Errorf("render = %q, want the pre-filled name", f.render())
	}
}

func TestCategoryFormTyping(t *testing.T) {
	f := newCategoryForm("New Category", nil)

	for _, m := range typeString("Woodworking") {
		f.update(m)
	}
	if f.value() != "Woodworking" {
		t.Errorf("value() = %q, want %q", f.value(), "Woodworking")
	}

	f.update(key("backspace"))
	if f.value() != "Woodworkin" {
		t.Errorf("value() after backspace = %q, want %q", f.value(), "Woodworkin")
	}
}

func TestCategoryFormEnterSubmitsAndEscCancels(t *testing.T) {
	f := newCategoryForm("New Category", nil)

	if done, submitted := f.update(key("esc")); !done || submitted {
		t.Errorf("esc = (%v, %v), want (true, false)", done, submitted)
	}

	f2 := newCategoryForm("New Category", nil)
	if done, submitted := f2.update(key("enter")); !done || !submitted {
		t.Errorf("enter = (%v, %v), want (true, true)", done, submitted)
	}
}
