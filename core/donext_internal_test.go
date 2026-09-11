package core

import (
	"testing"
	"time"
)

// stubRand returns a fixed value from Float64, so a test can force
// weightedPick's draw to land at a specific point in the cumulative range.
type stubRand struct{ v float64 }

func (r stubRand) Float64() float64 { return r.v }

func TestDoNextWeightPinsTheV1Formula(t *testing.T) {
	base := time.Date(2024, time.January, 10, 0, 0, 0, 0, time.UTC)
	due := func(d time.Duration) *time.Time {
		t := base.Add(d)
		return &t
	}
	tests := []struct {
		name string
		cand DoNextCandidate
		want float64
	}{
		{"no Priority, no due date", DoNextCandidate{}, 1.0},
		{"Task Priority only", DoNextCandidate{NextStep: Task{Priority: true}}, 2.0},
		{"Project Priority only", DoNextCandidate{Project: Project{Priority: true}}, 2.0},
		{"both Priority (not additive)", DoNextCandidate{Project: Project{Priority: true}, NextStep: Task{Priority: true}}, 2.0},
		{"overdue", DoNextCandidate{NextStep: Task{DueDate: due(-time.Hour)}}, 4.0},
		{"due within 3 days", DoNextCandidate{NextStep: Task{DueDate: due(3 * 24 * time.Hour)}}, 3.0},
		{"due within 7 days", DoNextCandidate{NextStep: Task{DueDate: due(7 * 24 * time.Hour)}}, 2.0},
		{"due within 14 days", DoNextCandidate{NextStep: Task{DueDate: due(14 * 24 * time.Hour)}}, 1.5},
		{"due beyond 14 days", DoNextCandidate{NextStep: Task{DueDate: due(15 * 24 * time.Hour)}}, 1.0},
		{
			"overdue and Priority stack multiplicatively",
			DoNextCandidate{NextStep: Task{Priority: true, DueDate: due(-time.Hour)}},
			8.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doNextWeight(tt.cand, base); got != tt.want {
				t.Errorf("doNextWeight = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWeightedPickReturnsLastCandidateAtTheTopOfTheRange exercises the
// boundary weightedPick must handle regardless of floating-point rounding:
// a draw that lands at (or a hair past) the summed total still must return
// the last candidate, not fall through with nothing picked.
func TestWeightedPickReturnsLastCandidateAtTheTopOfTheRange(t *testing.T) {
	candidates := []DoNextCandidate{
		{Project: Project{ID: 1}, NextStep: Task{ID: 1}},
		{Project: Project{ID: 2}, NextStep: Task{ID: 2}},
	}
	now := time.Now()

	// Float64() == 1.0 is out of math/rand's contract, but a hair below still
	// pushes target to (an epsilon under) the full total — the top of the
	// last candidate's slice, which floating summation error could round
	// past the naive "target < cumulative" comparison.
	got := weightedPick(candidates, now, stubRand{v: 0.9999999999999999})
	if got.Project.ID != 2 {
		t.Errorf("weightedPick at the top of the range returned Project %d, want the last candidate (2)", got.Project.ID)
	}
}
