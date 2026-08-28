package tui

import (
	"strings"
	"testing"
)

func TestDashboardViewRendersEmptyStateWithoutPanic(t *testing.T) {
	got := renderDashboard()

	if !strings.Contains(got, "Project Organizer") {
		t.Errorf("dashboard view = %q, want it to contain the title", got)
	}
	if !strings.Contains(got, "No Active Projects yet.") {
		t.Errorf("dashboard view = %q, want it to show the empty state", got)
	}
}

func TestDashboardModelViewDoesNotPanicWithZeroProjects(t *testing.T) {
	d := newDashboard(nil) // View must not touch core.
	if out := d.View(); out == "" {
		t.Error("dashboard View() returned empty string")
	}
}
