package core

import "context"

// This file holds the deliberate, explicit actions that cross the
// loose-Task/Milestone boundary (ADR 0001). MoveTask and MoveMilestoneTask
// each reorder within one scope only; MoveTaskToMilestone and MoveTaskToBody
// (and their After variants) are the separate, named operations for moving a
// Task between scopes.

// MoveTaskToMilestone moves a loose Task out of the Project body and into a
// Milestone's own ordered list, appending it as that Milestone's last Task.
// The Project body re-sequences to close the gap the Task left; the
// Milestone's Tasks re-sequence to give it the last slot. ErrTaskNotFound if
// taskID does not name a live loose Task — including one already inside a
// Milestone, which has no body-level scope to leave. ErrMilestoneNotFound if
// milestoneID does not name a live Milestone in the same Project as the Task.
func (c *Core) MoveTaskToMilestone(ctx context.Context, taskID, milestoneID int64) (Task, error) {
	return c.moveTaskToMilestone(ctx, taskID, milestoneID, nil)
}

// MoveTaskToMilestoneAfter is MoveTaskToMilestone with an explicit landing
// spot: the moved Task sits immediately after afterTaskID within the
// Milestone's own order, or at the front when afterTaskID is 0. Same errors as
// MoveTaskToMilestone, plus ErrTaskNotFound when a non-zero afterTaskID is not
// one of the Milestone's Tasks.
func (c *Core) MoveTaskToMilestoneAfter(ctx context.Context, taskID, milestoneID, afterTaskID int64) (Task, error) {
	return c.moveTaskToMilestone(ctx, taskID, milestoneID, &afterTaskID)
}

// moveTaskToMilestone is the shared body of MoveTaskToMilestone and
// MoveTaskToMilestoneAfter: after is the explicit landing spot, or nil to
// append at the end of the Milestone's Tasks.
func (c *Core) moveTaskToMilestone(ctx context.Context, taskID, milestoneID int64, after *int64) (Task, error) {
	task, err := c.store.GetTask(ctx, taskID)
	if err != nil {
		return Task{}, err
	}
	if task.MilestoneID != nil {
		return Task{}, ErrTaskNotFound // already inside a Milestone; not a loose Task to cross
	}
	m, err := c.store.GetMilestone(ctx, milestoneID)
	if err != nil {
		return Task{}, err
	}
	if m.ProjectID != task.ProjectID {
		return Task{}, ErrMilestoneNotFound // not a Milestone in this Task's Project
	}

	if _, err := c.store.SetTaskMilestone(ctx, taskID, &milestoneID); err != nil {
		return Task{}, err
	}
	body, err := c.loadBody(ctx, task.ProjectID)
	if err != nil {
		return Task{}, err
	}
	afterTaskID, err := resolveMilestoneAnchor(body, milestoneID, taskID, after)
	if err != nil {
		return Task{}, err
	}
	if err := body.PlaceMilestoneTaskAfter(milestoneID, taskID, afterTaskID); err != nil {
		return Task{}, err
	}
	if err := c.store.WriteBodyOrder(ctx, task.ProjectID, body.Order()); err != nil {
		return Task{}, err
	}
	return c.store.GetTask(ctx, taskID)
}

// MoveTaskToBody moves a Task out of its Milestone and into the Project body as
// a loose Task, appended at the end of the body. The Milestone's Tasks
// re-sequence to close the gap the Task left; the Project body re-sequences to
// give it the last slot. ErrTaskNotFound if taskID does not name a live Task
// inside a Milestone — a loose Task has no Milestone scope to leave.
func (c *Core) MoveTaskToBody(ctx context.Context, taskID int64) (Task, error) {
	return c.moveTaskToBody(ctx, taskID, nil)
}

// MoveTaskToBodyAfter is MoveTaskToBody with an explicit landing spot: the
// moved Task sits immediately after the body slot `after` holds, or at the
// front when `after` is the zero BodyRef. Same error as MoveTaskToBody, plus
// ErrTaskNotFound / ErrMilestoneNotFound when a non-zero `after` names no live
// body slot of the Task's Project.
func (c *Core) MoveTaskToBodyAfter(ctx context.Context, taskID int64, after BodyRef) (Task, error) {
	return c.moveTaskToBody(ctx, taskID, &after)
}

// moveTaskToBody is the shared body of MoveTaskToBody and MoveTaskToBodyAfter:
// after is the explicit landing spot, or nil to append at the end of the body.
func (c *Core) moveTaskToBody(ctx context.Context, taskID int64, after *BodyRef) (Task, error) {
	task, err := c.store.GetTask(ctx, taskID)
	if err != nil {
		return Task{}, err
	}
	if task.MilestoneID == nil {
		return Task{}, ErrTaskNotFound // already loose; nothing to cross
	}

	if _, err := c.store.SetTaskMilestone(ctx, taskID, nil); err != nil {
		return Task{}, err
	}
	body, err := c.loadBody(ctx, task.ProjectID)
	if err != nil {
		return Task{}, err
	}
	taskRef := BodyRef{Kind: TaskEntry, ID: taskID}
	anchor := after
	if anchor == nil {
		last := lastOtherRef(body.Tree(), taskRef)
		anchor = &last
	}
	if err := body.PlaceSlotAfter(taskRef, *anchor); err != nil {
		return Task{}, err
	}
	if err := c.store.WriteBodyOrder(ctx, task.ProjectID, body.Order()); err != nil {
		return Task{}, err
	}
	return c.store.GetTask(ctx, taskID)
}

// resolveMilestoneAnchor is the afterTaskID to place a just-relocated Task at
// within milestoneID's Tasks: the explicit after when given, or the id of the
// Milestone's current last Task (excluding taskID itself) to land at the end.
func resolveMilestoneAnchor(body *Body, milestoneID, taskID int64, after *int64) (int64, error) {
	if after != nil {
		return *after, nil
	}
	tasks, err := body.MilestoneTasks(milestoneID)
	if err != nil {
		return 0, err
	}
	return lastOtherID(tasks, func(t Task) int64 { return t.ID }, taskID), nil
}

// lastOtherRef is the BodyRef of the last top-level slot in entries that is not
// exclude, or the zero BodyRef ("place at the front") when there is none.
func lastOtherRef(entries []BodyEntry, exclude BodyRef) BodyRef {
	return lastOtherID(entries, BodyEntry.Ref, exclude)
}

// lastOtherID is the key (from keyOf) of the last item in items whose key is
// not exclude, or the zero key ("place at the front") when there is none. The
// shared shape behind picking an "append at the end" anchor, over either a
// Milestone's Tasks (keyed by Task id) or a Project body (keyed by BodyRef).
func lastOtherID[T any, K comparable](items []T, keyOf func(T) K, exclude K) K {
	var last K
	for _, item := range items {
		if k := keyOf(item); k != exclude {
			last = k
		}
	}
	return last
}
