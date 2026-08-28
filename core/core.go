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
// concrete type.
type Store interface {
	// ListCategories returns every Category in seed order.
	ListCategories(ctx context.Context) ([]Category, error)
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
