package core

import (
	"context"
	"errors"
)

// A Project's body is one user-ordered sequence whose entries are
// heterogeneous: each is either a loose Task or a Milestone (ADR 0001). The
// types and the move machinery here operate over that mixed sequence; the
// Task- and Milestone-specific rules live in their own files.

// BodyEntryKind marks which kind of slot a BodyEntry is.
type BodyEntryKind int

// The two kinds of Project-body slot.
const (
	TaskEntry BodyEntryKind = iota
	MilestoneEntry
)

// BodyEntry is one slot in a Project's single user-ordered body: exactly one of
// Task or Milestone is set, per Kind. A loose Task can sit before, between, or
// after Milestones, so callers walk the slice in order rather than treating
// Tasks and Milestones as separate lists.
type BodyEntry struct {
	Kind      BodyEntryKind
	Task      *Task
	Milestone *Milestone
}

// BodyRef identifies one body slot by its kind and row id — the handle position
// swaps and selection-tracking key off, so callers pass it around instead of a
// loose (kind, id) pair.
type BodyRef struct {
	Kind BodyEntryKind
	ID   int64
}

// Body is a Project's ordered body held in memory: the interleaved loose Tasks
// and Milestones (each Milestone carrying its own ordered Tasks) as one value
// the domain rules — Next step, and later reordering — run over. It holds no
// Store and creates no rows, so it tests as a pure value; Core loads it, calls
// it, and persists the result.
type Body struct {
	entries []BodyEntry
}

// newBody wraps a freshly loaded slice as a Body, enforcing the invariant that
// every Milestone's Tasks slice is non-nil (possibly empty) so callers never
// have to nil-check it.
func newBody(entries []BodyEntry) *Body {
	for _, e := range entries {
		if e.Kind == MilestoneEntry && e.Milestone.Tasks == nil {
			e.Milestone.Tasks = []Task{}
		}
	}
	return &Body{entries: entries}
}

// Tree is the body's ordered slots, loose Tasks and Milestones interleaved,
// each Milestone carrying its own ordered Tasks.
func (b *Body) Tree() []BodyEntry {
	return b.entries
}

// NextStep resolves the single Task the Project is waiting on: walk the slots in
// order to the first incomplete one — a not-done loose Task is it; a Milestone
// yields its first not-done Task; a Milestone with none (empty or all done) is
// skipped. ok is false when nothing is incomplete.
func (b *Body) NextStep() (Task, bool) {
	for _, e := range b.entries {
		if e.Kind == TaskEntry {
			if !e.Task.Done {
				return *e.Task, true
			}
			continue
		}
		for _, mt := range e.Milestone.Tasks {
			if !mt.Done {
				return mt, true
			}
		}
	}
	return Task{}, false
}

// loadBody reads a Project's ordered body and wraps it as a Body.
// ErrProjectNotFound if projectID does not name a live Project.
func (c *Core) loadBody(ctx context.Context, projectID int64) (*Body, error) {
	if _, err := c.store.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	entries, err := c.store.ListProjectBody(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return newBody(entries), nil
}

// Ref is the slot's BodyRef, read without the caller re-checking Kind.
func (e BodyEntry) Ref() BodyRef {
	if e.Kind == MilestoneEntry {
		return BodyRef{Kind: MilestoneEntry, ID: e.Milestone.ID}
	}
	return BodyRef{Kind: TaskEntry, ID: e.Task.ID}
}

// MoveDir is the direction a body entry — a loose Task or a Milestone — moves
// within its Project body.
type MoveDir int

// The two move directions. Moving reorders within the Project-body scope only.
const (
	MoveUp MoveDir = iota
	MoveDown
)

// ErrInvalidMove is returned when a move direction is neither MoveUp nor
// MoveDown.
var ErrInvalidMove = errors.New("invalid move direction")

// moveBodyEntry swaps the slot named by ref with its neighbour in dir, then
// returns the reordered body. A move past either edge leaves the body
// untouched. Shared by MoveTask and MoveMilestone, which validate dir first:
// reordering stays within the Project-body scope, never crossing into a
// Milestone.
func (c *Core) moveBodyEntry(ctx context.Context, projectID int64, ref BodyRef, dir MoveDir) ([]BodyEntry, error) {
	body, err := c.store.ListProjectBody(ctx, projectID)
	if err != nil {
		return nil, err
	}
	moved, err := c.swapWithNeighbor(ctx, bodyRefs(body), ref, dir)
	if err != nil {
		return nil, err
	}
	if !moved {
		return body, nil // already at the edge; nothing to reorder
	}
	return c.store.ListProjectBody(ctx, projectID)
}

// bodyRefs is the ordered slot refs of a body, the shape swapWithNeighbor walks.
func bodyRefs(body []BodyEntry) []BodyRef {
	refs := make([]BodyRef, len(body))
	for i, e := range body {
		refs[i] = e.Ref()
	}
	return refs
}

// swapWithNeighbor exchanges target's stored position with that of the slot one
// step in dir within the ordered refs, and reports whether a swap happened — a
// move past either edge is a no-op. It is the shared reorder primitive for both
// ordering scopes: the Project body (moveBodyEntry) and a Milestone's own Tasks
// (MoveMilestoneTask). A target absent from refs is its kind's not-found
// sentinel.
func (c *Core) swapWithNeighbor(ctx context.Context, refs []BodyRef, target BodyRef, dir MoveDir) (bool, error) {
	idx := -1
	for i, r := range refs {
		if r == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false, notFoundFor(target.Kind)
	}
	neighbor := idx - 1
	if dir == MoveDown {
		neighbor = idx + 1
	}
	if neighbor < 0 || neighbor >= len(refs) {
		return false, nil
	}
	if err := c.store.SwapBodyPositions(ctx, target, refs[neighbor]); err != nil {
		return false, err
	}
	return true, nil
}

// notFoundFor is the sentinel for a body slot of the given kind that turned up
// missing.
func notFoundFor(kind BodyEntryKind) error {
	if kind == MilestoneEntry {
		return ErrMilestoneNotFound
	}
	return ErrTaskNotFound
}
