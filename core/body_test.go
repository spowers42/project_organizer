package core

import (
	"errors"
	"strconv"
	"testing"
)

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

// lt / ms / mt build identified slots for the reorder tests, where ids are what
// the assertions key off.
func lt(id int64, title string) BodyEntry {
	return BodyEntry{Kind: TaskEntry, Task: &Task{ID: id, Title: title}}
}

func ms(id int64, name string, tasks ...Task) BodyEntry {
	return BodyEntry{Kind: MilestoneEntry, Milestone: &Milestone{ID: id, Name: name, Tasks: tasks}}
}

func mt(id int64, title string) Task {
	return Task{ID: id, Title: title}
}

// slotLabels renders a Body's top-level order as "t<id>" / "m<id>" tokens.
func slotLabels(b *Body) []string {
	out := make([]string, 0, len(b.entries))
	for _, e := range b.entries {
		if e.Kind == MilestoneEntry {
			out = append(out, "m"+strconv.FormatInt(e.Milestone.ID, 10))
			continue
		}
		out = append(out, "t"+strconv.FormatInt(e.Task.ID, 10))
	}
	return out
}

// milestoneTaskLabels renders one Milestone's Task order as "t<id>" tokens.
func milestoneTaskLabels(b *Body, milestoneID int64) []string {
	tasks, err := b.MilestoneTasks(milestoneID)
	if err != nil {
		return []string{"<" + err.Error() + ">"}
	}
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = "t" + strconv.FormatInt(t.ID, 10)
	}
	return out
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

func TestBodyMoveSlot(t *testing.T) {
	t.Run("swaps a loose Task down past a Milestone", func(t *testing.T) {
		b := newBody([]BodyEntry{lt(1, "a"), ms(2, "M"), lt(3, "c")})
		moved, err := b.MoveSlot(BodyRef{Kind: TaskEntry, ID: 1}, MoveDown)
		if err != nil || !moved {
			t.Fatalf("MoveSlot = (%v, %v), want (true, nil)", moved, err)
		}
		if got := slotLabels(b); !eqStrings(got, []string{"m2", "t1", "t3"}) {
			t.Errorf("order = %v, want [m2 t1 t3]", got)
		}
	})

	t.Run("a Milestone carries its Tasks when it moves", func(t *testing.T) {
		b := newBody([]BodyEntry{lt(1, "a"), ms(2, "M", mt(10, "x"), mt(11, "y"))})
		if _, err := b.MoveSlot(BodyRef{Kind: MilestoneEntry, ID: 2}, MoveUp); err != nil {
			t.Fatalf("MoveSlot: %v", err)
		}
		if got := slotLabels(b); !eqStrings(got, []string{"m2", "t1"}) {
			t.Errorf("order = %v, want [m2 t1]", got)
		}
		if got := milestoneTaskLabels(b, 2); !eqStrings(got, []string{"t10", "t11"}) {
			t.Errorf("milestone Tasks = %v, want [t10 t11] still attached", got)
		}
	})

	t.Run("past the edge is a no-op", func(t *testing.T) {
		b := newBody([]BodyEntry{lt(1, "a"), lt(2, "b")})
		moved, err := b.MoveSlot(BodyRef{Kind: TaskEntry, ID: 1}, MoveUp)
		if moved || err != nil {
			t.Fatalf("MoveSlot at top = (%v, %v), want (false, nil)", moved, err)
		}
	})

	t.Run("bad direction is ErrInvalidMove", func(t *testing.T) {
		b := newBody([]BodyEntry{lt(1, "a"), lt(2, "b")})
		if _, err := b.MoveSlot(BodyRef{Kind: TaskEntry, ID: 1}, MoveDir(9)); !errors.Is(err, ErrInvalidMove) {
			t.Errorf("err = %v, want ErrInvalidMove", err)
		}
	})

	t.Run("a nested Task id is not a top-level slot", func(t *testing.T) {
		b := newBody([]BodyEntry{ms(2, "M", mt(10, "x"))})
		if _, err := b.MoveSlot(BodyRef{Kind: TaskEntry, ID: 10}, MoveUp); !errors.Is(err, ErrTaskNotFound) {
			t.Errorf("err = %v, want ErrTaskNotFound", err)
		}
	})
}

func TestBodyMoveMilestoneTask(t *testing.T) {
	t.Run("swaps within the Milestone", func(t *testing.T) {
		b := newBody([]BodyEntry{ms(2, "M", mt(10, "x"), mt(11, "y"), mt(12, "z"))})
		moved, err := b.MoveMilestoneTask(11, MoveDown)
		if err != nil || !moved {
			t.Fatalf("MoveMilestoneTask = (%v, %v), want (true, nil)", moved, err)
		}
		if got := milestoneTaskLabels(b, 2); !eqStrings(got, []string{"t10", "t12", "t11"}) {
			t.Errorf("order = %v, want [t10 t12 t11]", got)
		}
	})

	t.Run("past the edge is a no-op", func(t *testing.T) {
		b := newBody([]BodyEntry{ms(2, "M", mt(10, "x"), mt(11, "y"))})
		moved, err := b.MoveMilestoneTask(11, MoveDown)
		if moved || err != nil {
			t.Fatalf("MoveMilestoneTask at bottom = (%v, %v), want (false, nil)", moved, err)
		}
	})

	t.Run("a loose Task id has no Milestone scope", func(t *testing.T) {
		b := newBody([]BodyEntry{lt(1, "a"), ms(2, "M", mt(10, "x"))})
		if _, err := b.MoveMilestoneTask(1, MoveUp); !errors.Is(err, ErrTaskNotFound) {
			t.Errorf("err = %v, want ErrTaskNotFound", err)
		}
	})
}

func TestBodyPlaceSlotAfter(t *testing.T) {
	t.Run("moves a slot to just after the anchor", func(t *testing.T) {
		b := newBody([]BodyEntry{lt(1, "a"), lt(2, "b"), lt(3, "c")})
		if err := b.PlaceSlotAfter(BodyRef{Kind: TaskEntry, ID: 3}, BodyRef{Kind: TaskEntry, ID: 1}); err != nil {
			t.Fatalf("PlaceSlotAfter: %v", err)
		}
		if got := slotLabels(b); !eqStrings(got, []string{"t1", "t3", "t2"}) {
			t.Errorf("order = %v, want [t1 t3 t2]", got)
		}
	})

	t.Run("a zero anchor moves the slot to the front", func(t *testing.T) {
		b := newBody([]BodyEntry{lt(1, "a"), lt(2, "b"), lt(3, "c")})
		if err := b.PlaceSlotAfter(BodyRef{Kind: TaskEntry, ID: 3}, BodyRef{}); err != nil {
			t.Fatalf("PlaceSlotAfter(front): %v", err)
		}
		if got := slotLabels(b); !eqStrings(got, []string{"t3", "t1", "t2"}) {
			t.Errorf("order = %v, want [t3 t1 t2]", got)
		}
	})

	t.Run("an unknown anchor is not-found for its kind", func(t *testing.T) {
		b := newBody([]BodyEntry{lt(1, "a"), lt(2, "b")})
		if err := b.PlaceSlotAfter(BodyRef{Kind: TaskEntry, ID: 2}, BodyRef{Kind: TaskEntry, ID: 99}); !errors.Is(err, ErrTaskNotFound) {
			t.Errorf("err = %v, want ErrTaskNotFound", err)
		}
		if err := b.PlaceSlotAfter(BodyRef{Kind: TaskEntry, ID: 2}, BodyRef{Kind: MilestoneEntry, ID: 99}); !errors.Is(err, ErrMilestoneNotFound) {
			t.Errorf("err = %v, want ErrMilestoneNotFound", err)
		}
	})
}

func TestBodyPlaceMilestoneTaskAfter(t *testing.T) {
	base := func() *Body {
		return newBody([]BodyEntry{ms(2, "M", mt(10, "x"), mt(11, "y"), mt(12, "z"))})
	}

	t.Run("reorders within the Milestone", func(t *testing.T) {
		b := base()
		if err := b.PlaceMilestoneTaskAfter(2, 12, 10); err != nil {
			t.Fatalf("PlaceMilestoneTaskAfter: %v", err)
		}
		if got := milestoneTaskLabels(b, 2); !eqStrings(got, []string{"t10", "t12", "t11"}) {
			t.Errorf("order = %v, want [t10 t12 t11]", got)
		}
	})

	t.Run("a zero afterTaskID moves to the front", func(t *testing.T) {
		b := base()
		if err := b.PlaceMilestoneTaskAfter(2, 12, 0); err != nil {
			t.Fatalf("PlaceMilestoneTaskAfter(front): %v", err)
		}
		if got := milestoneTaskLabels(b, 2); !eqStrings(got, []string{"t12", "t10", "t11"}) {
			t.Errorf("order = %v, want [t12 t10 t11]", got)
		}
	})

	t.Run("error cases", func(t *testing.T) {
		if err := base().PlaceMilestoneTaskAfter(99, 10, 0); !errors.Is(err, ErrMilestoneNotFound) {
			t.Errorf("unknown milestone: err = %v, want ErrMilestoneNotFound", err)
		}
		if err := base().PlaceMilestoneTaskAfter(2, 77, 0); !errors.Is(err, ErrTaskNotFound) {
			t.Errorf("unknown task: err = %v, want ErrTaskNotFound", err)
		}
		if err := base().PlaceMilestoneTaskAfter(2, 10, 77); !errors.Is(err, ErrTaskNotFound) {
			t.Errorf("unknown anchor: err = %v, want ErrTaskNotFound", err)
		}
	})
}

func TestBodyOrderRoundTrips(t *testing.T) {
	b := newBody([]BodyEntry{
		lt(1, "a"),
		ms(2, "M", mt(10, "x"), mt(11, "y")),
		lt(3, "c"),
	})
	o := b.Order()
	wantSlots := []BodyRef{
		{Kind: TaskEntry, ID: 1},
		{Kind: MilestoneEntry, ID: 2},
		{Kind: TaskEntry, ID: 3},
	}
	if len(o.Slots) != 3 || o.Slots[0] != wantSlots[0] || o.Slots[1] != wantSlots[1] || o.Slots[2] != wantSlots[2] {
		t.Errorf("Slots = %v, want %v", o.Slots, wantSlots)
	}
	if got := o.MilestoneTasks[2]; len(got) != 2 || got[0] != 10 || got[1] != 11 {
		t.Errorf("MilestoneTasks[2] = %v, want [10 11]", got)
	}
}

func TestBodyLooseTasksSkipsMilestoneTasks(t *testing.T) {
	b := newBody([]BodyEntry{lt(1, "a"), ms(2, "M", mt(10, "x")), lt(3, "c")})
	got := b.LooseTasks()
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Errorf("LooseTasks = %+v, want the two loose Tasks only", got)
	}
}
