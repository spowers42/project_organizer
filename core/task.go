package core

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Task is a single actionable step sitting directly in a Project's ordered body
// (a "loose" Task). It carries an optional due date and a completion flag. Its
// position in the body is not exposed — it only feeds Next step resolution.
// Tasks inside a Milestone arrive in a later ticket.
type Task struct {
	ID        int64
	ProjectID int64
	Title     string
	DueDate   *time.Time
	Notes     string
	Done      bool
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

// NextStep resolves the single Task a Project is waiting on right now: walking
// the body in order, the first incomplete loose Task. ok is false when every
// loose Task is done, or the Project has none. ErrProjectNotFound if id does
// not name a live Project. Milestone bodies are not consulted yet.
func (c *Core) NextStep(ctx context.Context, projectID int64) (Task, bool, error) {
	tasks, err := c.ProjectTasks(ctx, projectID)
	if err != nil {
		return Task{}, false, err
	}
	for _, t := range tasks {
		if !t.Done {
			return t, true, nil
		}
	}
	return Task{}, false, nil
}

// ActiveProjectNextStep pairs an Active Project with its resolved Next step.
// NextStep is nil when the Project has no incomplete loose Task.
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
