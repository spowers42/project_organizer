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
//
// Completion is derived, never stored: a Milestone is complete when it has at
// least one Task and every Task is done (see milestoneComplete). CompletionAcked
// tracks only whether the user has been prompted for the *current* completion,
// so the "just completed" signal SetTaskDone returns fires once; it is cleared
// whenever the Milestone gains a Task or one of its Tasks is un-completed, so a
// later re-completion signals again.
type Milestone struct {
	ID              int64
	ProjectID       int64
	Name            string
	Tasks           []Task
	CompletionAcked bool
}

// milestoneComplete reports whether m is finished by its own Tasks: non-empty
// and every Task done. m.Tasks must be populated (as ProjectBody and
// MilestoneTasks do); an empty Milestone is never complete.
func milestoneComplete(m Milestone) bool {
	if len(m.Tasks) == 0 {
		return false
	}
	for _, t := range m.Tasks {
		if !t.Done {
			return false
		}
	}
	return true
}

// AckMilestoneComplete acknowledges a Milestone's current derived completion,
// suppressing the "just completed" signal until the acknowledgement is cleared
// by a Task added to it or one of its Tasks being un-completed. The TUI calls
// this for both confirm and decline — core tracks no separate "declined"
// state, only whether the user has already been asked. ErrMilestoneNotFound if
// milestoneID does not name a live Milestone.
func (c *Core) AckMilestoneComplete(ctx context.Context, milestoneID int64) (Milestone, error) {
	return c.store.SetMilestoneCompletionAck(ctx, milestoneID, true)
}

// clearMilestoneAck resets a Milestone's completion acknowledgement so a
// completion produced by a subsequent change signals again. Called wherever a
// Task is added into a Milestone.
func (c *Core) clearMilestoneAck(ctx context.Context, milestoneID int64) error {
	_, err := c.store.SetMilestoneCompletionAck(ctx, milestoneID, false)
	return err
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
	return c.store.InsertMilestone(ctx, projectID, trimmed)
}

// AddMilestoneAfter inserts a Milestone into a Project's body one place after the
// body slot `after` holds — just below the cursor — pushing the following
// entries one place later. A zero `after` inserts at the front. Same name
// validation as AddMilestone. ErrProjectNotFound if projectID names no live
// Project; ErrTaskNotFound / ErrMilestoneNotFound if a non-zero `after` names no
// live body slot of the Project.
func (c *Core) AddMilestoneAfter(ctx context.Context, projectID int64, after BodyRef, name string) (Milestone, error) {
	body, err := c.loadBody(ctx, projectID)
	if err != nil {
		return Milestone{}, err
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return Milestone{}, ErrEmptyMilestoneName
	}
	if after.ID != 0 && body.indexOfSlot(after) == -1 {
		return Milestone{}, notFoundFor(after.Kind) // stale cursor anchor; add nothing
	}
	m, err := c.store.InsertMilestone(ctx, projectID, trimmed)
	if err != nil {
		return Milestone{}, err
	}
	if err := c.placeAfterInsert(ctx, projectID, BodyRef{Kind: MilestoneEntry, ID: m.ID}, after); err != nil {
		return Milestone{}, err
	}
	return m, nil
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
	t, err := c.store.InsertMilestoneTask(ctx, milestoneID, title, in.DueDate, in.Notes)
	if err != nil {
		return Task{}, err
	}
	if err := c.clearMilestoneAck(ctx, milestoneID); err != nil {
		return Task{}, err
	}
	return t, nil
}

// AddMilestoneTaskAfter inserts a Task into a Milestone's ordered list one place
// after the slot afterTaskID holds — just below the cursor — pushing the
// following Tasks one place later. afterTaskID 0 inserts at the front. Same title
// validation as AddMilestoneTask. ErrMilestoneNotFound if milestoneID names no
// live Milestone; ErrTaskNotFound if a non-zero afterTaskID is not one of its
// Tasks.
func (c *Core) AddMilestoneTaskAfter(ctx context.Context, milestoneID, afterTaskID int64, in TaskInput) (Task, error) {
	m, err := c.store.GetMilestone(ctx, milestoneID)
	if err != nil {
		return Task{}, err
	}
	title, err := validateTaskInput(in)
	if err != nil {
		return Task{}, err
	}
	body, err := c.loadBody(ctx, m.ProjectID)
	if err != nil {
		return Task{}, err
	}
	tasks, err := body.MilestoneTasks(milestoneID)
	if err != nil {
		return Task{}, err
	}
	if afterTaskID != 0 && !containsTaskID(tasks, afterTaskID) {
		return Task{}, ErrTaskNotFound // stale cursor anchor; add nothing
	}
	t, err := c.store.InsertMilestoneTask(ctx, milestoneID, title, in.DueDate, in.Notes)
	if err != nil {
		return Task{}, err
	}
	if err := c.clearMilestoneAck(ctx, milestoneID); err != nil {
		return Task{}, err
	}
	fresh, err := c.loadBody(ctx, m.ProjectID)
	if err != nil {
		return Task{}, err
	}
	if err := fresh.PlaceMilestoneTaskAfter(milestoneID, t.ID, afterTaskID); err != nil {
		return Task{}, err
	}
	if err := c.store.WriteBodyOrder(ctx, m.ProjectID, fresh.Order()); err != nil {
		return Task{}, err
	}
	return t, nil
}

// containsTaskID reports whether tasks holds a Task with id.
func containsTaskID(tasks []Task, id int64) bool {
	for _, t := range tasks {
		if t.ID == id {
			return true
		}
	}
	return false
}

// MilestoneTasks returns a Milestone's Tasks in Milestone order.
// ErrMilestoneNotFound if milestoneID does not name a live Milestone.
func (c *Core) MilestoneTasks(ctx context.Context, milestoneID int64) ([]Task, error) {
	m, err := c.store.GetMilestone(ctx, milestoneID)
	if err != nil {
		return nil, err
	}
	body, err := c.loadBody(ctx, m.ProjectID)
	if err != nil {
		return nil, err
	}
	return body.MilestoneTasks(milestoneID)
}

// MoveMilestoneTask reorders a Task one slot earlier (MoveUp) or later
// (MoveDown) within its Milestone, swapping positions with its neighbor there.
// Moving the first Task up or the last Task down is a no-op. It returns the
// Milestone's Tasks in the resulting order. ErrInvalidMove if dir is neither
// direction; ErrTaskNotFound if id does not name a live Task inside a Milestone.
func (c *Core) MoveMilestoneTask(ctx context.Context, id int64, dir MoveDir) ([]Task, error) {
	task, err := c.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.MilestoneID == nil {
		return nil, ErrTaskNotFound // a loose Task has no Milestone scope to move in
	}
	body, err := c.loadBody(ctx, task.ProjectID)
	if err != nil {
		return nil, err
	}
	moved, err := body.MoveMilestoneTask(id, dir)
	if err != nil {
		return nil, err
	}
	if moved {
		if err := c.store.WriteBodyOrder(ctx, task.ProjectID, body.Order()); err != nil {
			return nil, err
		}
	}
	return body.MilestoneTasks(*task.MilestoneID)
}

// MoveMilestone reorders a Milestone one slot earlier (MoveUp) or later
// (MoveDown) within its Project body, swapping with whichever entry — a loose
// Task or another Milestone — currently sits in that slot. Moving the first
// entry up or the last entry down is a no-op. It returns the Project's body in
// the resulting order. ErrInvalidMove if dir is neither direction;
// ErrMilestoneNotFound if id does not name a live Milestone.
func (c *Core) MoveMilestone(ctx context.Context, id int64, dir MoveDir) ([]BodyEntry, error) {
	m, err := c.store.GetMilestone(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.reorderBody(ctx, m.ProjectID, func(b *Body) (bool, error) {
		return b.MoveSlot(BodyRef{Kind: MilestoneEntry, ID: id}, dir)
	})
}
