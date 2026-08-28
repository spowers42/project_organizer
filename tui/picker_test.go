package tui

import (
	"strings"
	"testing"
)

func TestPickerMovementClampsAtBothEnds(t *testing.T) {
	p := newPicker([]string{"one", "two", "three"}, 0)

	p.up() // already at the top
	if got := p.selectedIndex(); got != 0 {
		t.Errorf("after up() at top, selectedIndex = %d, want 0", got)
	}

	p.down()
	p.down()
	p.down() // past the bottom
	if got := p.selectedIndex(); got != 2 {
		t.Errorf("after three down(), selectedIndex = %d, want 2", got)
	}
	if got := p.value(); got != "three" {
		t.Errorf("value = %q, want %q", got, "three")
	}
}

func TestPickerClampsAnOutOfRangeStart(t *testing.T) {
	p := newPicker([]string{"a", "b"}, 9)
	if got := p.selectedIndex(); got != 1 {
		t.Errorf("selectedIndex = %d, want it clamped to 1", got)
	}
}

func TestPickerEmptyListHasNoSelection(t *testing.T) {
	p := newPicker(nil, 0)
	if got := p.selectedIndex(); got != -1 {
		t.Errorf("selectedIndex = %d, want -1 for an empty list", got)
	}
	if got := p.value(); got != "" {
		t.Errorf("value = %q, want empty for an empty list", got)
	}
	if !strings.Contains(p.render(), "(none)") {
		t.Errorf("render = %q, want it to show an empty marker", p.render())
	}
}

func TestPickerRenderMarksTheSelectedRow(t *testing.T) {
	p := newPicker([]string{"one", "two"}, 1)
	got := p.render()
	if !strings.Contains(got, "> two") {
		t.Errorf("render = %q, want a caret against %q", got, "two")
	}
	if strings.Contains(got, "> one") {
		t.Errorf("render = %q, want no caret against the unselected row", got)
	}
}
