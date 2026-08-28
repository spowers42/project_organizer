// Package core is the single application-service seam for project_organizer.
// Every workflow described in the spec is exposed here; entrypoints (the TUI and
// the archive CLI) are thin and hold no domain logic. Callers pass value inputs
// and receive value results and typed errors — no database/sql types cross this
// boundary.
package core

import "context"

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
