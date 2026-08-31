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
	moved, err := c.swapWithNeighbour(ctx, bodyRefs(body), ref, dir)
	if err != nil {
		return nil, err
	}
	if !moved {
		return body, nil // already at the edge; nothing to reorder
	}
	return c.store.ListProjectBody(ctx, projectID)
}

// bodyRefs is the ordered slot refs of a body, the shape swapWithNeighbour walks.
func bodyRefs(body []BodyEntry) []BodyRef {
	refs := make([]BodyRef, len(body))
	for i, e := range body {
		refs[i] = e.Ref()
	}
	return refs
}

// swapWithNeighbour exchanges target's stored position with that of the slot one
// step in dir within the ordered refs, and reports whether a swap happened — a
// move past either edge is a no-op. It is the shared reorder primitive for both
// ordering scopes: the Project body (moveBodyEntry) and a Milestone's own Tasks
// (MoveMilestoneTask). A target absent from refs is its kind's not-found
// sentinel.
func (c *Core) swapWithNeighbour(ctx context.Context, refs []BodyRef, target BodyRef, dir MoveDir) (bool, error) {
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
	neighbour := idx - 1
	if dir == MoveDown {
		neighbour = idx + 1
	}
	if neighbour < 0 || neighbour >= len(refs) {
		return false, nil
	}
	if err := c.store.SwapBodyPositions(ctx, target, refs[neighbour]); err != nil {
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
