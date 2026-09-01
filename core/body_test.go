package core

import "testing"

// looseTask builds a loose-Task body slot for the Body tests.
func looseTask(title string, done bool) BodyEntry {
	return BodyEntry{Kind: TaskEntry, Task: &Task{Title: title, Done: done}}
}

// milestone builds a Milestone body slot carrying the given inner Tasks.
func milestone(name string, tasks ...Task) BodyEntry {
	return BodyEntry{Kind: MilestoneEntry, Milestone: &Milestone{Name: name, Tasks: tasks}}
}

// task is a terse inner-Task literal for milestone().
func task(title string, done bool) Task {
	return Task{Title: title, Done: done}
}

func TestBodyNextStep(t *testing.T) {
	cases := []struct {
		name    string
		entries []BodyEntry
		want    string // "" means ok=false
	}{
		{
			name:    "first incomplete loose Task",
			entries: []BodyEntry{looseTask("one", false), looseTask("two", false)},
			want:    "one",
		},
		{
			name:    "skips a completed loose Task",
			entries: []BodyEntry{looseTask("one", true), looseTask("two", false)},
			want:    "two",
		},
		{
			name:    "descends into a Milestone for its first incomplete Task",
			entries: []BodyEntry{milestone("M", task("a", true), task("b", false))},
			want:    "b",
		},
		{
			name:    "skips a Milestone whose Tasks are all done",
			entries: []BodyEntry{milestone("M", task("a", true), task("b", true)), looseTask("after", false)},
			want:    "after",
		},
		{
			name:    "skips an empty Milestone",
			entries: []BodyEntry{milestone("M"), looseTask("after", false)},
			want:    "after",
		},
		{
			name:    "loose Task after a Milestone is reachable",
			entries: []BodyEntry{milestone("M", task("a", true)), looseTask("loose", false)},
			want:    "loose",
		},
		{
			name:    "nothing incomplete returns ok=false",
			entries: []BodyEntry{looseTask("one", true), milestone("M", task("a", true))},
			want:    "",
		},
		{
			name:    "empty body returns ok=false",
			entries: nil,
			want:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := newBody(tc.entries).NextStep()
			if tc.want == "" {
				if ok {
					t.Fatalf("NextStep = (%q, true), want ok=false", got.Title)
				}
				return
			}
			if !ok {
				t.Fatalf("NextStep = (_, false), want %q", tc.want)
			}
			if got.Title != tc.want {
				t.Errorf("NextStep = %q, want %q", got.Title, tc.want)
			}
		})
	}
}

func TestNewBodyGuaranteesNonNilMilestoneTasks(t *testing.T) {
	b := newBody([]BodyEntry{{Kind: MilestoneEntry, Milestone: &Milestone{Name: "M"}}})
	if got := b.Tree()[0].Milestone.Tasks; got == nil {
		t.Fatal("newBody left Milestone.Tasks nil; want a non-nil empty slice")
	}
}

func TestBodyTreeReturnsEntriesInOrder(t *testing.T) {
	entries := []BodyEntry{looseTask("one", false), milestone("M"), looseTask("two", false)}
	tree := newBody(entries).Tree()
	if len(tree) != 3 || tree[0].Task.Title != "one" || tree[1].Milestone.Name != "M" || tree[2].Task.Title != "two" {
		t.Fatalf("Tree returned %+v, want the entries unchanged and in order", tree)
	}
}
