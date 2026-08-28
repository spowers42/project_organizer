package tui

import (
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestFilterFormDefaultsToTheCurrentConstraint(t *testing.T) {
	f := newFilterForm(testCategories(), core.ProjectFilter{Lifecycle: core.Active})
	got := f.filter()
	if got.Lifecycle != core.Active || got.CategoryID != 0 {
		t.Errorf("filter() = %+v, want {Active, 0}", got)
	}
}

func TestFilterFormBuildsACombinedConstraint(t *testing.T) {
	f := newFilterForm(testCategories(), core.ProjectFilter{})

	// lifecycle list focused first: All -> Active -> Paused
	f.update(key("down"))
	f.update(key("down"))
	// switch to the Category list: All -> Programming -> Course
	f.update(key("tab"))
	f.update(key("down"))
	f.update(key("down"))

	got := f.filter()
	if got.Lifecycle != core.Paused {
		t.Errorf("Lifecycle = %q, want Paused", got.Lifecycle)
	}
	if got.CategoryID != 20 {
		t.Errorf("CategoryID = %d, want 20 (Course)", got.CategoryID)
	}
}

func TestFilterFormAllSelectionsClearTheConstraint(t *testing.T) {
	f := newFilterForm(testCategories(), core.ProjectFilter{Lifecycle: core.Done, CategoryID: 30})
	// move both lists back to their first row ("All")
	for i := 0; i < 6; i++ {
		f.update(key("up"))
	}
	f.update(key("tab"))
	for i := 0; i < 6; i++ {
		f.update(key("up"))
	}

	got := f.filter()
	if got.Lifecycle != "" || got.CategoryID != 0 {
		t.Errorf("filter() = %+v, want the zero (unconstrained) filter", got)
	}
}

func TestFilterFormApplyAndCancelReport(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		wantDone    bool
		wantApplied bool
	}{
		{"enter applies", "enter", true, true},
		{"esc cancels", "esc", true, false},
		{"other key is inert", "x", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFilterForm(testCategories(), core.ProjectFilter{})
			done, applied := f.update(key(tt.key))
			if done != tt.wantDone || applied != tt.wantApplied {
				t.Errorf("%s => done=%v applied=%v, want %v/%v", tt.key, done, applied, tt.wantDone, tt.wantApplied)
			}
		})
	}
}

func TestFilterLabelSummarisesTheConstraint(t *testing.T) {
	if got := filterLabel(core.ProjectFilter{}, testCategories()); got != "All states · all Categories" {
		t.Errorf("label = %q", got)
	}
	got := filterLabel(core.ProjectFilter{Lifecycle: core.Active, CategoryID: 20}, testCategories())
	if !strings.Contains(got, "Active") || !strings.Contains(got, "Course") {
		t.Errorf("label = %q, want it to name the state and Category", got)
	}

	// An id absent from the list (e.g. Categories failed to load) must not leak
	// a raw number into the header.
	got = filterLabel(core.ProjectFilter{CategoryID: 999}, testCategories())
	if strings.Contains(got, "999") || strings.Contains(got, "#") {
		t.Errorf("label = %q, want a neutral placeholder, not a raw id", got)
	}
}
