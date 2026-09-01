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
	// ListProjectTasks returns a Project's loose Tasks — those not inside a
	// Milestone — in body order.
	ListProjectTasks(ctx context.Context, projectID int64) ([]Task, error)
	// CreateMilestoneTask appends a Task to a Milestone's own ordered list
	// (position after that Milestone's existing Tasks) and returns it as stored.
	// Reports ErrMilestoneNotFound when milestoneID names no live Milestone.
	CreateMilestoneTask(ctx context.Context, milestoneID int64, title string, dueDate *time.Time, notes string) (Task, error)
	// CreateTaskAfter inserts a loose Task one place after the body slot
	// afterTaskID holds (afterTaskID 0 inserts at the front), shifting the
	// following slots later. Reports ErrTaskNotFound when a non-zero afterTaskID
	// is not a live loose Task of the Project.
	CreateTaskAfter(ctx context.Context, projectID, afterTaskID int64, title string, dueDate *time.Time, notes string) (Task, error)
	// CreateMilestoneTaskAfter inserts a Task one place after the slot afterTaskID
	// holds in a Milestone's ordered list (afterTaskID 0 inserts at the front),
	// shifting the following Tasks later. Reports ErrTaskNotFound when a non-zero
	// afterTaskID is not a live Task of the Milestone, ErrMilestoneNotFound when
	// milestoneID names no live Milestone.
	CreateMilestoneTaskAfter(ctx context.Context, milestoneID, afterTaskID int64, title string, dueDate *time.Time, notes string) (Task, error)
	// ListMilestoneTasks returns a Milestone's Tasks in Milestone order.
	ListMilestoneTasks(ctx context.Context, milestoneID int64) ([]Task, error)

	// CreateMilestone appends a Milestone to a Project's body (position after
	// the existing entries, in the shared Task/Milestone position space) and
	// returns it as stored.
	CreateMilestone(ctx context.Context, projectID int64, name string) (Milestone, error)
	// CreateMilestoneAfter inserts a Milestone one place after the body slot
	// `after` holds (a zero after inserts at the front), shifting the following
	// slots later. Reports the anchor kind's not-found sentinel when a non-zero
	// `after` names no live body slot of the Project.
	CreateMilestoneAfter(ctx context.Context, projectID int64, after BodyRef, name string) (Milestone, error)
	// GetMilestone reads one live Milestone by id, reporting
	// ErrMilestoneNotFound when it is missing or archived.
	GetMilestone(ctx context.Context, id int64) (Milestone, error)
	// ListProjectBody returns a Project's ordered body — its loose Tasks and
	// Milestones interleaved by stored position. Each Milestone entry carries
	// its own ordered Tasks.
	ListProjectBody(ctx context.Context, projectID int64) ([]BodyEntry, error)
	// ReadBody returns a Project's ordered body — its loose Tasks and Milestones
	// interleaved by stored position, each Milestone carrying its own ordered
	// Tasks. It is the read half of the Body seam.
	ReadBody(ctx context.Context, projectID int64) ([]BodyEntry, error)
	// WriteBodyOrder renumbers a Project's body to match an in-memory ordering:
	// every top-level slot and every Milestone's own Tasks are set to 0..N-1 in
	// one transaction. It is the write half of the Body seam — the single
	// persistence call for reorders and for placing a freshly inserted row.
	WriteBodyOrder(ctx context.Context, projectID int64, order BodyOrder) error
	// InsertLooseTask appends a loose Task to the end of a Project's body and
	// returns it as stored; placement is a follow-up WriteBodyOrder.
	InsertLooseTask(ctx context.Context, projectID int64, title string, dueDate *time.Time, notes string) (Task, error)
	// InsertMilestoneTask appends a Task to the end of a Milestone's own list and
	// returns it as stored; placement is a follow-up WriteBodyOrder. Reports
	// ErrMilestoneNotFound when milestoneID names no live Milestone.
	InsertMilestoneTask(ctx context.Context, milestoneID int64, title string, dueDate *time.Time, notes string) (Task, error)
	// InsertMilestone appends a Milestone to the end of a Project's body and
	// returns it as stored; placement is a follow-up WriteBodyOrder.
	InsertMilestone(ctx context.Context, projectID int64, name string) (Milestone, error)
	// SwapBodyPositions exchanges the stored position of two live rows in one
	// transaction, each named by a BodyRef (its kind picking the table). It is
	// the reorder primitive for both ordering scopes — the Project body (loose
	// Tasks and Milestones) and a Milestone's own Tasks — since a position swap
	// by id is scope-agnostic. A missing row is ErrTaskNotFound or
	// ErrMilestoneNotFound and rolls the swap back.
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
