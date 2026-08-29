// Package core is the single application-service seam for project_organizer.
// Every workflow described in the spec is exposed here; entrypoints (the TUI and
// the archive CLI) are thin and hold no domain logic. Callers pass value inputs
// and receive value results and typed errors — no database/sql types cross this
// boundary.
package core

import (
	"context"
	"time"
)

// Category classifies a Project or an Idea. The list is shared across both,
// seeded with Programming, Course, Other, and extendable by the user.
type Category struct {
	ID   int64
	Name string
}

// Store is the persistence dependency injected into Core. It is backed by
// SQLite in production and by a temp-file database in tests; Core never sees the
// concrete type. Core does the domain validation; Store persists what it is
// given and reports ErrProjectNotFound when an update or read matches no live
// row.
type Store interface {
	// ListCategories returns every Category in seed order.
	ListCategories(ctx context.Context) ([]Category, error)
	// CategoryExists reports whether a Category with the given id exists.
	CategoryExists(ctx context.Context, id int64) (bool, error)

	// CreateProject inserts a Project and returns it as stored.
	CreateProject(ctx context.Context, name, description string, categoryID int64, lifecycle Lifecycle) (Project, error)
	// UpdateProject rewrites a live Project's name, description, and Category.
	UpdateProject(ctx context.Context, id int64, name, description string, categoryID int64) (Project, error)
	// UpdateProjectLifecycle moves a live Project to lifecycle.
	UpdateProjectLifecycle(ctx context.Context, id int64, lifecycle Lifecycle) (Project, error)
	// GetProject reads one live Project by id.
	GetProject(ctx context.Context, id int64) (Project, error)
	// ListProjects returns live Projects in creation order. An empty lifecycle
	// or a zero categoryID is not filtered on.
	ListProjects(ctx context.Context, lifecycle Lifecycle, categoryID int64) ([]Project, error)
	// ArchiveProject stamps archived_at on a live Project, reporting
	// ErrProjectNotFound when no live row matches.
	ArchiveProject(ctx context.Context, id int64, at time.Time) error

	// CreateTask appends a loose Task to a Project's body (position after the
	// existing entries) and returns it as stored.
	CreateTask(ctx context.Context, projectID int64, title string, dueDate *time.Time, notes string) (Task, error)
	// UpdateTask rewrites a live Task's title, due date, and notes; a nil
	// dueDate clears the due date and an empty notes clears the notes. Reports
	// ErrTaskNotFound when no live row matches.
	UpdateTask(ctx context.Context, id int64, title string, dueDate *time.Time, notes string) (Task, error)
	// SetTaskDone sets a live Task's completion flag. Reports ErrTaskNotFound
	// when no live row matches.
	SetTaskDone(ctx context.Context, id int64, done bool) (Task, error)
	// GetTask reads one live Task by id, reporting ErrTaskNotFound when it is
	// missing or archived.
	GetTask(ctx context.Context, id int64) (Task, error)
	// ListProjectTasks returns a Project's loose Tasks in body order.
	ListProjectTasks(ctx context.Context, projectID int64) ([]Task, error)

	// CreateMilestone appends a Milestone to a Project's body (position after
	// the existing entries, in the shared Task/Milestone position space) and
	// returns it as stored.
	CreateMilestone(ctx context.Context, projectID int64, name string) (Milestone, error)
	// GetMilestone reads one live Milestone by id, reporting
	// ErrMilestoneNotFound when it is missing or archived.
	GetMilestone(ctx context.Context, id int64) (Milestone, error)
	// ListProjectBody returns a Project's ordered body — its loose Tasks and
	// Milestones interleaved by stored position.
	ListProjectBody(ctx context.Context, projectID int64) ([]BodyEntry, error)
	// SwapBodyPositions exchanges the body positions of two live slots in one
	// transaction. Each slot is a loose Task or a Milestone, named by a BodyRef;
	// a missing row is ErrTaskNotFound or ErrMilestoneNotFound and rolls the
	// swap back.
	SwapBodyPositions(ctx context.Context, a, b BodyRef) error
}

// Core holds the injected dependencies and exposes the application operations.
type Core struct {
	store Store
	clock Clock
	rand  Rand
}

// New wires a Core from its three dependencies: a Store (persistence), a Clock
// (the only source of wall-clock time), and a Rand (the only source of
// non-determinism, used by the Do Next pick).
func New(store Store, clock Clock, rand Rand) *Core {
	return &Core{store: store, clock: clock, rand: rand}
}

// ListCategories returns the shared Category list in seed order.
func (c *Core) ListCategories(ctx context.Context) ([]Category, error) {
	return c.store.ListCategories(ctx)
}
