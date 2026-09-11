package core

import (
	"context"
	"time"
)

// DoNextCandidate pairs a Next step with the Active Project it belongs to —
// what Do Next hands the user to act on.
type DoNextCandidate struct {
	Project  Project
	NextStep Task
}

// DoNext picks one Task for the user to work on in a moment of indecision. The
// candidate pool is the Next step of each Active Project — at most one per
// Project; Projects that are not Active, or that have no incomplete Task, are
// excluded. The pick is weighted random over that pool (see doNextWeight),
// drawn from the injected Rand, so it is reproducible under a fixed seed and
// clock. ok is false when the pool is empty — a graceful empty state, not an
// error. Calling it again (a "reroll") draws again from the same pool.
func (c *Core) DoNext(ctx context.Context) (DoNextCandidate, bool, error) {
	rows, err := c.Dashboard(ctx)
	if err != nil {
		return DoNextCandidate{}, false, err
	}
	candidates := make([]DoNextCandidate, 0, len(rows))
	for _, row := range rows {
		if row.NextStep == nil {
			continue
		}
		candidates = append(candidates, DoNextCandidate{Project: row.Project, NextStep: *row.NextStep})
	}
	if len(candidates) == 0 {
		return DoNextCandidate{}, false, nil
	}
	return weightedPick(candidates, c.clock.Now(), c.rand), true, nil
}

// doNextWeight computes a candidate's weight for the Do Next pick: the v1
// formula from the issue, pinned by TestDoNextWeighting*. A starred Task or
// Project doubles it; an approaching or overdue due date scales it up further,
// topping out at 4x when overdue. Staleness weighting is out of scope for v1.
func doNextWeight(candidate DoNextCandidate, now time.Time) float64 {
	weight := 1.0
	task, project := candidate.NextStep, candidate.Project
	if task.Priority || project.Priority {
		weight *= 2.0
	}
	switch {
	case task.DueDate == nil:
		// no change
	case now.After(*task.DueDate):
		weight *= 4.0
	case task.DueDate.Sub(now) <= 3*24*time.Hour:
		weight *= 3.0
	case task.DueDate.Sub(now) <= 7*24*time.Hour:
		weight *= 2.0
	case task.DueDate.Sub(now) <= 14*24*time.Hour:
		weight *= 1.5
	}
	return weight
}

// weightedPick draws one candidate from candidates with probability
// proportional to doNextWeight, using r for the single random draw it needs.
// candidates is never empty — callers check that first. The last candidate is
// always reachable, even if summing the weights loses a hair of precision:
// the loop's final iteration returns unconditionally rather than compare
// against a possibly-rounded total.
func weightedPick(candidates []DoNextCandidate, now time.Time, r Rand) DoNextCandidate {
	weights := make([]float64, len(candidates))
	total := 0.0
	for i, cand := range candidates {
		weights[i] = doNextWeight(cand, now)
		total += weights[i]
	}
	target := r.Float64() * total
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if target < cumulative || i == len(candidates)-1 {
			return candidates[i]
		}
	}
	panic("unreachable: the last iteration above always returns")
}
