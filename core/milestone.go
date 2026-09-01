package core

import (
	"context"
	"errors"
	"strings"
)

// Milestone is an optional, ordered grouping of Tasks marking a meaningful chunk
// of progress. It occupies one slot in a Project's ordered body (per ADR 0001
// the body is one heterogeneous sequence, not a separate Milestone list) and
// carries its own ordered list of inner Tasks, which travel with it when it is
// reordered. A Milestone may be empty.
//
// Tasks is populated by ProjectBody (and left nil by GetMilestone, which reads
// the Milestone alone). Use MilestoneTasks for the list on its own.
type Milestone struct {
	ID        int64
	ProjectID int64
	Name      string
	Tasks     []Task
}

// Errors returned by the Milestone operations. Callers match them with
// errors.Is; the entrypoints turn them into user-facing messages.
var (
	ErrEmptyMilestoneName = errors.New("milestone name must not be empty")
	ErrMilestoneNotFound  = errors.New("milestone not found")
)

// AddMilestone appends a Milestone to the end of a Project's body. The name is
// trimmed and must be non-empty. ErrProjectNotFound if projectID does not name a
// live Project.
func (c *Core) AddMilestone(ctx context.Context, projectID int64, name string) (Milestone, error) {
	if _, err := c.store.GetProject(ctx, projectID); err != nil {
		return Milestone{}, err
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return Milestone{}, ErrEmptyMilestoneName
	}
	return c.store.CreateMilestone(ctx, projectID, trimmed)
}

// AddMilestoneAfter inserts a Milestone into a Project's body one place after the
// body slot `after` holds — just below the cursor — pushing the following
// entries one place later. A zero `after` inserts at the front. Same name
// validation as AddMilestone. ErrProjectNotFound if projectID names no live
// Project; ErrTaskNotFound / ErrMilestoneNotFound if a non-zero `after` names no
// live body slot of the Project.
func (c *Core) AddMilestoneAfter(ctx context.Context, projectID int64, after BodyRef, name string) (Milestone, error) {
	if _, err := c.store.GetProject(ctx, projectID); err != nil {
		return Milestone{}, err
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return Milestone{}, ErrEmptyMilestoneName
	}
	return c.store.CreateMilestoneAfter(ctx, projectID, after, trimmed)
}

// ProjectBody returns a Project's ordered body: its loose Tasks and Milestones
// interleaved in stored order. ErrProjectNotFound if projectID does not name a
// live Project.
func (c *Core) ProjectBody(ctx context.Context, projectID int64) ([]BodyEntry, error) {
	body, err := c.loadBody(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return body.Tree(), nil
}

// AddMilestoneTask appends a Task to a Milestone's own ordered list. The title
// is trimmed and must be non-empty (ErrEmptyTaskTitle); the due date and notes
// are optional. ErrMilestoneNotFound if milestoneID does not name a live
// Milestone.
func (c *Core) AddMilestoneTask(ctx context.Context, milestoneID int64, in TaskInput) (Task, error) {
	if _, err := c.store.GetMilestone(ctx, milestoneID); err != nil {
		return Task{}, err
	}
	title, err := validateTaskInput(in)
	if err != nil {
		return Task{}, err
	}
	return c.store.CreateMilestoneTask(ctx, milestoneID, title, in.DueDate, in.Notes)
}

// AddMilestoneTaskAfter inserts a Task into a Milestone's ordered list one place
// after the slot afterTaskID holds — just below the cursor — pushing the
// following Tasks one place later. afterTaskID 0 inserts at the front. Same title
// validation as AddMilestoneTask. ErrMilestoneNotFound if milestoneID names no
// live Milestone; ErrTaskNotFound if a non-zero afterTaskID is not one of its
// Tasks.
func (c *Core) AddMilestoneTaskAfter(ctx context.Context, milestoneID, afterTaskID int64, in TaskInput) (Task, error) {
	if _, err := c.store.GetMilestone(ctx, milestoneID); err != nil {
		return Task{}, err
	}
	title, err := validateTaskInput(in)
	if err != nil {
		return Task{}, err
	}
	return c.store.CreateMilestoneTaskAfter(ctx, milestoneID, afterTaskID, title, in.DueDate, in.Notes)
}

// MilestoneTasks returns a Milestone's Tasks in Milestone order.
// ErrMilestoneNotFound if milestoneID does not name a live Milestone.
func (c *Core) MilestoneTasks(ctx context.Context, milestoneID int64) ([]Task, error) {
	if _, err := c.store.GetMilestone(ctx, milestoneID); err != nil {
		return nil, err
	}
	return c.store.ListMilestoneTasks(ctx, milestoneID)
}

// MoveMilestoneTask reorders a Task one slot earlier (MoveUp) or later
// (MoveDown) within its Milestone, swapping positions with its neighbor there.
// Moving the first Task up or the last Task down is a no-op. It returns the
// Milestone's Tasks in the resulting order. ErrInvalidMove if dir is neither
// direction; ErrTaskNotFound if id does not name a live Task inside a Milestone.
func (c *Core) MoveMilestoneTask(ctx context.Context, id int64, dir MoveDir) ([]Task, error) {
	if dir != MoveUp && dir != MoveDown {
		return nil, ErrInvalidMove
	}
	task, err := c.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.MilestoneID == nil {
		return nil, ErrTaskNotFound // a loose Task has no Milestone scope to move in
	}
	tasks, err := c.store.ListMilestoneTasks(ctx, *task.MilestoneID)
	if err != nil {
		return nil, err
	}
	refs := make([]BodyRef, len(tasks))
	for i, t := range tasks {
		refs[i] = BodyRef{Kind: TaskEntry, ID: t.ID}
	}
	moved, err := c.swapWithNeighbor(ctx, refs, BodyRef{Kind: TaskEntry, ID: id}, dir)
	if err != nil {
		return nil, err
	}
	if !moved {
		return tasks, nil // already at the edge; nothing to reorder
	}
	return c.store.ListMilestoneTasks(ctx, *task.MilestoneID)
}

// MoveMilestone reorders a Milestone one slot earlier (MoveUp) or later
// (MoveDown) within its Project body, swapping with whichever entry — a loose
// Task or another Milestone — currently sits in that slot. Moving the first
// entry up or the last entry down is a no-op. It returns the Project's body in
// the resulting order. ErrInvalidMove if dir is neither direction;
// ErrMilestoneNotFound if id does not name a live Milestone.
func (c *Core) MoveMilestone(ctx context.Context, id int64, dir MoveDir) ([]BodyEntry, error) {
	if dir != MoveUp && dir != MoveDown {
		return nil, ErrInvalidMove
	}
	m, err := c.store.GetMilestone(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.moveBodyEntry(ctx, m.ProjectID, BodyRef{Kind: MilestoneEntry, ID: id}, dir)
}
