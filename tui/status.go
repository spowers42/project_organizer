package tui

import (
	"errors"

	"github.com/spowers42/project_organizer/core"
)

// errorMessage turns a typed core error into a sentence for the status line.
// Unrecognised errors fall back to their own text so nothing is swallowed.
func errorMessage(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, core.ErrEmptyProjectName):
		return "Project name must not be empty."
	case errors.Is(err, core.ErrCategoryNotFound):
		return "That Category no longer exists."
	case errors.Is(err, core.ErrProjectNotFound):
		return "That Project no longer exists."
	case errors.Is(err, core.ErrInvalidLifecycle):
		return "That is not a valid lifecycle state."
	case errors.Is(err, core.ErrEmptyTaskTitle):
		return "Task title must not be empty."
	case errors.Is(err, core.ErrTaskNotFound):
		return "That Task no longer exists."
	case errors.Is(err, errTaskDueDateFormat):
		return "Due date must be written as YYYY-MM-DD."
	default:
		return err.Error()
	}
}
