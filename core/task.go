package core

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Task is a single actionable step. It sits either directly in a Project's
// ordered body (a "loose" Task, MilestoneID nil) or inside a Milestone
// (MilestoneID set). It carries an optional due date and a completion flag. Its
// position is not exposed — it only feeds Next step resolution: loose Tasks
// order within the Project body, Milestone Tasks within their Milestone.
type Task struct {
	ID          int64
	ProjectID   int64
	MilestoneID *int64
	Title       string
	DueDate     *time.Time
	Notes       string
	Done        bool
}

// TaskInput carries the user-supplied fields for adding or editing a Task. The
// title is trimmed and must be non-empty; DueDate is optional and a nil value
// clears any existing due date on an edit. Notes is optional freeform text
// (possibly multi-line); an empty string means no notes and clears any existing
// notes on an edit.
type TaskInput struct {
	Title   string
	DueDate *time.Time
	Notes   string
}

// Errors returned by the Task operations. Callers match them with errors.Is;
// the entrypoints turn them into user-facing messages.
var (
	ErrEmptyTaskTitle = errors.New("task title must not be empty")
	ErrTaskNotFound   = errors.New("task not found")
)

// AddTask appends a loose Task to the end of a Project's body. The title is
// trimmed and must be non-empty; the due date is optional. ErrProjectNotFound
// if id does not name a live Project.
func (c *Core) AddTask(ctx context.Context, projectID int64, in TaskInput) (Task, error) {
	if _, err := c.store.GetProject(ctx, projectID); err != nil {
		return Task{}, err
	}
	title, err := validateTaskInput(in)
	if err != nil {
		return Task{}, err
	}
	return c.store.CreateTask(ctx, projectID, title, in.DueDate, in.Notes)
}

// AddTaskAfter inserts a loose Task into a Project's body one place after the
// slot afterTaskID holds — just below the cursor — pushing the following entries
// one place later. afterTaskID 0 inserts at the front. Same title validation as
// AddTask. ErrProjectNotFound if projectID names no live Project; ErrTaskNotFound
// if a non-zero afterTaskID is not one of its loose Tasks.
func (c *Core) AddTaskAfter(ctx context.Context, projectID, afterTaskID int64, in TaskInput) (Task, error) {
	if _, err := c.store.GetProject(ctx, projectID); err != nil {
		return Task{}, err
	}
	title, err := validateTaskInput(in)
	if err != nil {
		return Task{}, err
	}
	return c.store.CreateTaskAfter(ctx, projectID, afterTaskID, title, in.DueDate, in.Notes)
}

// EditTask rewrites a Task's title and due date. Same title validation as
// AddTask; a nil DueDate clears the due date. ErrTaskNotFound if id does not
// name a live Task — reported as such even when the input is also invalid.
func (c *Core) EditTask(ctx context.Context, id int64, in TaskInput) (Task, error) {
	if _, err := c.store.GetTask(ctx, id); err != nil {
		return Task{}, err
	}
	title, err := validateTaskInput(in)
	if err != nil {
		return Task{}, err
	}
	return c.store.UpdateTask(ctx, id, title, in.DueDate, in.Notes)
}

// SetTaskDone marks a Task done or not done. Completion order is unconstrained:
// a Task can be completed or un-completed at any time, regardless of the other
// Tasks in the body. ErrTaskNotFound if id does not name a live Task.
func (c *Core) SetTaskDone(ctx context.Context, id int64, done bool) (Task, error) {
	return c.store.SetTaskDone(ctx, id, done)
}

// ProjectTasks returns a Project's loose Tasks in body order.
// ErrProjectNotFound if id does not name a live Project.
func (c *Core) ProjectTasks(ctx context.Context, projectID int64) ([]Task, error) {
	if _, err := c.store.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	return c.store.ListProjectTasks(ctx, projectID)
}

// MoveTask reorders a loose Task one slot earlier (MoveUp) or later (MoveDown)
// within its Project body, persisting the new order. The neighbouring slot may
// hold another loose Task or a Milestone; the move swaps with it either way, so
// a loose Task can be positioned before, between, or after Milestones. Moving
// the first entry up or the last entry down is a no-op. It returns the
// Project's body in the resulting order. ErrInvalidMove if dir is neither
// direction; ErrTaskNotFound if id does not name a live Task.
func (c *Core) MoveTask(ctx context.Context, id int64, dir MoveDir) ([]BodyEntry, error) {
	if dir != MoveUp && dir != MoveDown {
		return nil, ErrInvalidMove
	}
	task, err := c.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.moveBodyEntry(ctx, task.ProjectID, BodyRef{Kind: TaskEntry, ID: id}, dir)
}

// NextStep resolves the single Task a Project is waiting on right now. It walks
// the Project body in order to the first incomplete entry: a not-done loose Task
// is the Next step; a Milestone yields its first incomplete Task (in Milestone
// order); a Milestone with no incomplete Task — empty or all done — is skipped.
// ok is false when the body is exhausted with nothing incomplete.
// ErrProjectNotFound if projectID does not name a live Project.
func (c *Core) NextStep(ctx context.Context, projectID int64) (Task, bool, error) {
	body, err := c.loadBody(ctx, projectID)
	if err != nil {
		return Task{}, false, err
	}
	task, ok := body.NextStep()
	return task, ok, nil
}

// ActiveProjectNextStep pairs an Active Project with its resolved Next step.
// NextStep is nil when nothing in the Project body is incomplete — no open loose
// Task and no Milestone with an open Task.
type ActiveProjectNextStep struct {
	Project  Project
	NextStep *Task
}

// Dashboard returns every Active Project with its Next step, in creation order —
// the data the dashboard screen shows for the "what's in flight / where is it
// waiting" question.
func (c *Core) Dashboard(ctx context.Context) ([]ActiveProjectNextStep, error) {
	projects, err := c.ActiveProjects(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]ActiveProjectNextStep, 0, len(projects))
	for _, p := range projects {
		row := ActiveProjectNextStep{Project: p}
		next, ok, err := c.NextStep(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			step := next
			row.NextStep = &step
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// validateTaskInput trims and checks the user-supplied Task title shared by
// AddTask and EditTask, returning the cleaned title.
func validateTaskInput(in TaskInput) (string, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return "", ErrEmptyTaskTitle
	}
	return title, nil
}
