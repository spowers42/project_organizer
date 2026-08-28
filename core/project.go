package core

import (
	"context"
	"errors"
	"strings"
)

// Lifecycle is a Project's lifecycle state. The five values are the CONTEXT.md
// terms verbatim; Done and Abandoned are kept distinct so the completed list
// stays honest.
type Lifecycle string

// The five Project lifecycle states.
const (
	Active    Lifecycle = "Active"
	Paused    Lifecycle = "Paused"
	Someday   Lifecycle = "Someday"
	Done      Lifecycle = "Done"
	Abandoned Lifecycle = "Abandoned"
)

// DefaultLifecycle is the state a newly created Project starts in, so the user
// does not have to set one every time.
const DefaultLifecycle = Active

// Valid reports whether l is one of the five known lifecycle states.
func (l Lifecycle) Valid() bool {
	switch l {
	case Active, Paused, Someday, Done, Abandoned:
		return true
	default:
		return false
	}
}

// Project is a larger, multi-step undertaking the user is tracking, classified
// by a Category and moving through the lifecycle states. Its ordered body of
// Tasks and Milestones arrives in a later ticket.
type Project struct {
	ID          int64
	Name        string
	Description string
	CategoryID  int64
	Lifecycle   Lifecycle
}

// ProjectInput carries the user-supplied fields for creating or editing a
// Project. Lifecycle is not part of it: creation uses DefaultLifecycle and
// later changes go through SetProjectLifecycle.
type ProjectInput struct {
	Name        string
	Description string
	CategoryID  int64
}

// ProjectFilter narrows ListProjects. A zero-value field means "no constraint":
// an empty Lifecycle matches every state, a zero CategoryID matches every
// Category.
type ProjectFilter struct {
	Lifecycle  Lifecycle
	CategoryID int64
}

// Errors returned by the Project operations. Callers match them with
// errors.Is; the entrypoints turn them into user-facing messages.
var (
	ErrEmptyProjectName = errors.New("project name must not be empty")
	ErrCategoryNotFound = errors.New("category not found")
	ErrProjectNotFound  = errors.New("project not found")
	ErrInvalidLifecycle = errors.New("invalid lifecycle state")
)

// CreateProject creates a Project with the given name, description, and
// Category. The name is trimmed and must be non-empty; the Category must exist.
// The new Project starts in DefaultLifecycle.
func (c *Core) CreateProject(ctx context.Context, in ProjectInput) (Project, error) {
	name, err := c.validateProjectInput(ctx, in)
	if err != nil {
		return Project{}, err
	}
	return c.store.CreateProject(ctx, name, in.Description, in.CategoryID, DefaultLifecycle)
}

// EditProject updates an existing Project's name, description, and Category. It
// does not touch the lifecycle state. Same validation as CreateProject;
// ErrProjectNotFound if id does not name a live Project — and a missing Project
// is reported as such even when the input is also invalid.
func (c *Core) EditProject(ctx context.Context, id int64, in ProjectInput) (Project, error) {
	if _, err := c.store.GetProject(ctx, id); err != nil {
		return Project{}, err
	}
	name, err := c.validateProjectInput(ctx, in)
	if err != nil {
		return Project{}, err
	}
	return c.store.UpdateProject(ctx, id, name, in.Description, in.CategoryID)
}

// validateProjectInput trims and checks the user-supplied Project fields shared
// by CreateProject and EditProject, returning the cleaned name.
func (c *Core) validateProjectInput(ctx context.Context, in ProjectInput) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", ErrEmptyProjectName
	}
	if err := c.requireCategory(ctx, in.CategoryID); err != nil {
		return "", err
	}
	return name, nil
}

// SetProjectLifecycle moves a Project to state. ErrInvalidLifecycle if state is
// not one of the five; ErrProjectNotFound if id does not name a live Project.
func (c *Core) SetProjectLifecycle(ctx context.Context, id int64, state Lifecycle) (Project, error) {
	if !state.Valid() {
		return Project{}, ErrInvalidLifecycle
	}
	return c.store.UpdateProjectLifecycle(ctx, id, state)
}

// GetProject reads a single Project back by id. ErrProjectNotFound if it does
// not exist or has been archived.
func (c *Core) GetProject(ctx context.Context, id int64) (Project, error) {
	return c.store.GetProject(ctx, id)
}

// ArchiveProject soft-deletes a Project into the Archive: it disappears from
// every normal view (dashboard, Project list, lookups) but is not destroyed and
// can be recovered through the archive CLI. ErrProjectNotFound if id does not
// name a live Project. Cascading to a Project's Milestones and Tasks arrives
// with those entities.
func (c *Core) ArchiveProject(ctx context.Context, id int64) error {
	return c.store.ArchiveProject(ctx, id, c.clock.Now())
}

// ListProjects returns the Projects matching filter, in creation order. An
// empty filter returns every live Project. A non-empty but unknown Lifecycle
// filter is ErrInvalidLifecycle.
func (c *Core) ListProjects(ctx context.Context, filter ProjectFilter) ([]Project, error) {
	if filter.Lifecycle != "" && !filter.Lifecycle.Valid() {
		return nil, ErrInvalidLifecycle
	}
	return c.store.ListProjects(ctx, filter.Lifecycle, filter.CategoryID)
}

// ActiveProjects returns exactly the Active Projects — the "in flight" work the
// dashboard shows. No Next step is resolved yet.
func (c *Core) ActiveProjects(ctx context.Context) ([]Project, error) {
	return c.ListProjects(ctx, ProjectFilter{Lifecycle: Active})
}

// requireCategory returns ErrCategoryNotFound unless id names an existing
// Category.
func (c *Core) requireCategory(ctx context.Context, id int64) error {
	ok, err := c.store.CategoryExists(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCategoryNotFound
	}
	return nil
}
