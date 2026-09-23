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
	// CreateCategory inserts a Category and returns it as stored.
	CreateCategory(ctx context.Context, name string) (Category, error)
	// RenameCategory rewrites a Category's name. Reports ErrCategoryNotFound
	// when no row matches.
	RenameCategory(ctx context.Context, id int64, name string) (Category, error)
	// CategoryReferenced reports whether any Project or Idea — including
	// archived ones — references the Category via category_id.
	CategoryReferenced(ctx context.Context, id int64) (bool, error)
	// DeleteCategory removes a Category row. Reports ErrCategoryNotFound when
	// no row matches.
	DeleteCategory(ctx context.Context, id int64) error

	// CreateProject inserts a Project and returns it as stored.
	CreateProject(ctx context.Context, name, description string, categoryID int64, lifecycle Lifecycle) (Project, error)
	// UpdateProject rewrites a live Project's name, description, and Category.
	UpdateProject(ctx context.Context, id int64, name, description string, categoryID int64) (Project, error)
	// UpdateProjectLifecycle moves a live Project to lifecycle.
	UpdateProjectLifecycle(ctx context.Context, id int64, lifecycle Lifecycle) (Project, error)
	// SetProjectPriority sets a live Project's Priority star. Reports
	// ErrProjectNotFound when no live row matches.
	SetProjectPriority(ctx context.Context, id int64, priority bool) (Project, error)
	// GetProject reads one live Project by id.
	GetProject(ctx context.Context, id int64) (Project, error)
	// ListProjects returns live Projects in creation order. An empty lifecycle
	// or a zero categoryID is not filtered on.
	ListProjects(ctx context.Context, lifecycle Lifecycle, categoryID int64) ([]Project, error)
	// ArchiveProject stamps archived_at on a live Project and cascades to its
	// live Milestones and Tasks, all sharing one cascade batch, in one
	// transaction. Reports ErrProjectNotFound when no live row matches. See
	// docs/workflows/archive-cascade.md.
	ArchiveProject(ctx context.Context, id int64, at time.Time) error

	// UpdateTask rewrites a live Task's title, due date, and notes; a nil
	// dueDate clears the due date and an empty notes clears the notes. Reports
	// ErrTaskNotFound when no live row matches.
	UpdateTask(ctx context.Context, id int64, title string, dueDate *time.Time, notes string) (Task, error)
	// SetTaskDone sets a live Task's completion flag. Reports ErrTaskNotFound
	// when no live row matches.
	SetTaskDone(ctx context.Context, id int64, done bool) (Task, error)
	// SetTaskPriority sets a live Task's Priority star. Reports ErrTaskNotFound
	// when no live row matches.
	SetTaskPriority(ctx context.Context, id int64, priority bool) (Task, error)
	// SetTaskMilestone rewrites which scope a live Task belongs to: nil for
	// loose, or a Milestone id to move it there. It only touches that column —
	// the caller repositions the Task with WriteBodyOrder afterward. Reports
	// ErrTaskNotFound when no live row matches.
	SetTaskMilestone(ctx context.Context, id int64, milestoneID *int64) (Task, error)
	// GetTask reads one live Task by id, reporting ErrTaskNotFound when it is
	// missing or archived. Core uses it to resolve a Task's owning Project
	// before loading that Project's Body.
	GetTask(ctx context.Context, id int64) (Task, error)
	// GetMilestone reads one live Milestone by id, reporting
	// ErrMilestoneNotFound when it is missing or archived.
	GetMilestone(ctx context.Context, id int64) (Milestone, error)
	// SetMilestoneCompletionAck rewrites a live Milestone's completion
	// acknowledgement flag. Reports ErrMilestoneNotFound when no live row
	// matches.
	SetMilestoneCompletionAck(ctx context.Context, id int64, acked bool) (Milestone, error)
	// ArchiveMilestone stamps archived_at on a live Milestone and cascades to
	// its live Tasks, sharing one cascade batch, in one transaction. Reports
	// ErrMilestoneNotFound when no live row matches. See
	// docs/workflows/archive-cascade.md.
	ArchiveMilestone(ctx context.Context, id int64, at time.Time) error
	// ArchiveTask stamps archived_at on a live Task. Reports ErrTaskNotFound
	// when no live row matches.
	ArchiveTask(ctx context.Context, id int64, at time.Time) error

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

	// CreateIdea inserts an Idea and returns it as stored.
	CreateIdea(ctx context.Context, name, description, notes string, categoryID int64) (Idea, error)
	// GetIdea reads one live Idea by id. Reports ErrIdeaNotFound when it is
	// missing or archived.
	GetIdea(ctx context.Context, id int64) (Idea, error)
	// ListIdeas returns live Ideas in creation order.
	ListIdeas(ctx context.Context) ([]Idea, error)
	// UpdateIdea rewrites a live Idea's name, description, notes, and
	// Category. Reports ErrIdeaNotFound when no live row matches.
	UpdateIdea(ctx context.Context, id int64, name, description, notes string, categoryID int64) (Idea, error)
	// ArchiveIdea stamps archived_at on a live Idea (plain delete; see
	// PromoteIdea for promotion). Reports ErrIdeaNotFound when no live row
	// matches.
	ArchiveIdea(ctx context.Context, id int64, at time.Time) error
	// PromoteIdea creates a Project from a live Idea and archives the Idea
	// with a link to it, in one transaction. See
	// docs/workflows/idea-promotion.md. Reports ErrIdeaNotFound when id does
	// not name a live Idea.
	PromoteIdea(ctx context.Context, id int64, lifecycle Lifecycle, at time.Time) (Project, error)

	// ListArchived returns every archived row across Projects, Milestones,
	// Tasks, and Ideas.
	ListArchived(ctx context.Context) ([]ArchivedEntity, error)
	// RestoreArchived reverses the archive call that archived ref's row,
	// restoring every row sharing its cascade batch. Reports
	// ErrArchivedNotFound when ref does not name a currently archived row.
	RestoreArchived(ctx context.Context, ref ArchiveRef) error
	// PurgeArchived permanently deletes ref's row along with every row sharing
	// its cascade batch. Reports ErrArchivedNotFound when ref does not name a
	// currently archived row.
	PurgeArchived(ctx context.Context, ref ArchiveRef) error
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
