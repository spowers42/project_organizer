package core

import "testing"

// milestoneComplete is derived purely from a Milestone's own Tasks: non-empty
// and every Task done. An empty Milestone is never complete, matching the
// spec's explicit "no Tasks, never complete" rule.
func TestMilestoneCompleteRequiresANonEmptyAllDoneTaskList(t *testing.T) {
	tests := []struct {
		name  string
		tasks []Task
		want  bool
	}{
		{"empty", nil, false},
		{"one incomplete", []Task{task("a", false)}, false},
		{"one done", []Task{task("a", true)}, true},
		{"mixed", []Task{task("a", true), task("b", false)}, false},
		{"all done", []Task{task("a", true), task("b", true)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Milestone{Tasks: tt.tasks}
			if got := milestoneComplete(m); got != tt.want {
				t.Errorf("milestoneComplete(%+v) = %v, want %v", m, got, tt.want)
			}
		})
	}
}
