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

// LooseTasks is the body's loose Tasks — those sitting directly in the body,
// not inside a Milestone — in body order.
func (b *Body) LooseTasks() []Task {
	var out []Task
	for _, e := range b.entries {
		if e.Kind == TaskEntry {
			out = append(out, *e.Task)
		}
	}
	return out
}

// MilestoneTasks is the ordered Tasks of the Milestone named by milestoneID.
// ErrMilestoneNotFound when the body holds no such Milestone.
func (b *Body) MilestoneTasks(milestoneID int64) ([]Task, error) {
	for _, e := range b.entries {
		if e.Kind == MilestoneEntry && e.Milestone.ID == milestoneID {
			return e.Milestone.Tasks, nil
		}
	}
	return nil, ErrMilestoneNotFound
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
	entries, err := c.store.ReadBody(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return newBody(entries), nil
}

// reorderBody loads the Project's Body, applies mutate, and persists the new
// order when mutate reports a change. It returns the resulting body. The shared
// spine of MoveTask, MoveMilestone, and MoveMilestoneTask.
func (c *Core) reorderBody(ctx context.Context, projectID int64, mutate func(*Body) (bool, error)) ([]BodyEntry, error) {
	body, err := c.loadBody(ctx, projectID)
	if err != nil {
		return nil, err
	}
	moved, err := mutate(body)
	if err != nil {
		return nil, err
	}
	if moved {
		if err := c.store.WriteBodyOrder(ctx, projectID, body.Order()); err != nil {
			return nil, err
		}
	}
	return body.Tree(), nil
}

// placeAfterInsert reloads the Project's Body (now holding a just-inserted
// slot), moves that slot to sit after anchor, and persists the order. The
// shared tail of the AddXAfter operations.
func (c *Core) placeAfterInsert(ctx context.Context, projectID int64, moved, anchor BodyRef) error {
	body, err := c.loadBody(ctx, projectID)
	if err != nil {
		return err
	}
	if err := body.PlaceSlotAfter(moved, anchor); err != nil {
		return err
	}
	return c.store.WriteBodyOrder(ctx, projectID, body.Order())
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

// BodyOrder is a Body's ordering as a plain value, the shape WriteBodyOrder
// persists: the top-level slots in order, plus each Milestone's own Task ids in
// order. Core never builds one by hand — it comes straight from Body.Order.
type BodyOrder struct {
	Slots          []BodyRef         // top-level, in order
	MilestoneTasks map[int64][]int64 // Milestone id -> ordered Task ids
}

// Order is the Body's current ordering as a BodyOrder, ready for
// Store.WriteBodyOrder.
func (b *Body) Order() BodyOrder {
	o := BodyOrder{
		Slots:          make([]BodyRef, len(b.entries)),
		MilestoneTasks: make(map[int64][]int64),
	}
	for i, e := range b.entries {
		o.Slots[i] = e.Ref()
		if e.Kind != MilestoneEntry {
			continue
		}
		ids := make([]int64, len(e.Milestone.Tasks))
		for j, t := range e.Milestone.Tasks {
			ids[j] = t.ID
		}
		o.MilestoneTasks[e.Milestone.ID] = ids
	}
	return o
}

// indexOfSlot is the position of the top-level slot named by ref, or -1.
func (b *Body) indexOfSlot(ref BodyRef) int {
	for i, e := range b.entries {
		if e.Ref() == ref {
			return i
		}
	}
	return -1
}

// MoveSlot swaps the top-level slot named by ref — a loose Task or a Milestone —
// with its neighbour in dir. A move past either edge is a no-op. moved reports
// whether anything changed, so the caller can skip persistence. ErrInvalidMove
// for a bad direction; notFoundFor(ref.Kind) when ref is not a top-level slot
// (a nested Task's id lands here and is ErrTaskNotFound).
func (b *Body) MoveSlot(ref BodyRef, dir MoveDir) (moved bool, err error) {
	if dir != MoveUp && dir != MoveDown {
		return false, ErrInvalidMove
	}
	idx := b.indexOfSlot(ref)
	if idx == -1 {
		return false, notFoundFor(ref.Kind)
	}
	neighbor := idx - 1
	if dir == MoveDown {
		neighbor = idx + 1
	}
	if neighbor < 0 || neighbor >= len(b.entries) {
		return false, nil
	}
	b.entries[idx], b.entries[neighbor] = b.entries[neighbor], b.entries[idx]
	return true, nil
}

// MoveMilestoneTask swaps the Task named by taskID with its neighbour in dir
// within its enclosing Milestone's own list. A move past either edge is a
// no-op. ErrInvalidMove for a bad direction; ErrTaskNotFound when no Milestone
// holds taskID (a loose Task's id lands here).
func (b *Body) MoveMilestoneTask(taskID int64, dir MoveDir) (moved bool, err error) {
	if dir != MoveUp && dir != MoveDown {
		return false, ErrInvalidMove
	}
	for _, e := range b.entries {
		if e.Kind != MilestoneEntry {
			continue
		}
		tasks := e.Milestone.Tasks
		for i := range tasks {
			if tasks[i].ID != taskID {
				continue
			}
			neighbor := i - 1
			if dir == MoveDown {
				neighbor = i + 1
			}
			if neighbor < 0 || neighbor >= len(tasks) {
				return false, nil
			}
			tasks[i], tasks[neighbor] = tasks[neighbor], tasks[i]
			return true, nil
		}
	}
	return false, ErrTaskNotFound
}

// PlaceSlotAfter moves the top-level slot named by ref to sit immediately after
// anchor. A zero-id anchor moves it to the front. notFoundFor(ref.Kind) when
// ref is not a top-level slot; notFoundFor(anchor.Kind) when a non-zero anchor
// is not one either.
func (b *Body) PlaceSlotAfter(ref, anchor BodyRef) error {
	from := b.indexOfSlot(ref)
	if from == -1 {
		return notFoundFor(ref.Kind)
	}
	moved := b.entries[from]
	rest := make([]BodyEntry, 0, len(b.entries)-1)
	rest = append(rest, b.entries[:from]...)
	rest = append(rest, b.entries[from+1:]...)

	insertAt := 0
	if anchor.ID != 0 {
		at := -1
		for i, e := range rest {
			if e.Ref() == anchor {
				at = i
				break
			}
		}
		if at == -1 {
			return notFoundFor(anchor.Kind)
		}
		insertAt = at + 1
	}

	out := make([]BodyEntry, 0, len(b.entries))
	out = append(out, rest[:insertAt]...)
	out = append(out, moved)
	out = append(out, rest[insertAt:]...)
	b.entries = out
	return nil
}

// PlaceMilestoneTaskAfter moves taskID within milestoneID's own list to sit
// immediately after afterTaskID. A zero afterTaskID moves it to the front.
// ErrMilestoneNotFound when milestoneID is not a Milestone in the body;
// ErrTaskNotFound when taskID, or a non-zero afterTaskID, is not one of its
// Tasks.
func (b *Body) PlaceMilestoneTaskAfter(milestoneID, taskID, afterTaskID int64) error {
	for _, e := range b.entries {
		if e.Kind != MilestoneEntry || e.Milestone.ID != milestoneID {
			continue
		}
		tasks := e.Milestone.Tasks
		from := -1
		for i := range tasks {
			if tasks[i].ID == taskID {
				from = i
				break
			}
		}
		if from == -1 {
			return ErrTaskNotFound
		}
		moved := tasks[from]
		rest := make([]Task, 0, len(tasks)-1)
		rest = append(rest, tasks[:from]...)
		rest = append(rest, tasks[from+1:]...)

		insertAt := 0
		if afterTaskID != 0 {
			at := -1
			for i := range rest {
				if rest[i].ID == afterTaskID {
					at = i
					break
				}
			}
			if at == -1 {
				return ErrTaskNotFound
			}
			insertAt = at + 1
		}

		out := make([]Task, 0, len(tasks))
		out = append(out, rest[:insertAt]...)
		out = append(out, moved)
		out = append(out, rest[insertAt:]...)
		e.Milestone.Tasks = out
		return nil
	}
	return ErrMilestoneNotFound
}

// notFoundFor is the sentinel for a body slot of the given kind that turned up
// missing.
func notFoundFor(kind BodyEntryKind) error {
	if kind == MilestoneEntry {
		return ErrMilestoneNotFound
	}
	return ErrTaskNotFound
}
