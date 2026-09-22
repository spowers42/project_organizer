package core

import (
	"context"
	"errors"
	"strings"
)

// Idea is a lightweight, non-Project capture of something the user might do
// later (CONTEXT.md): a name, a description, and a Category. It is never
// actionable and never a Do Next candidate. PromotedProjectID is nil until the
// Idea is promoted, at which point it is soft-deleted and this carries the id
// of the Project it became.
type Idea struct {
	ID                int64
	Name              string
	Description       string
	CategoryID        int64
	PromotedProjectID *int64
}

// IdeaInput carries the user-supplied fields for capturing an Idea.
type IdeaInput struct {
	Name        string
	Description string
	CategoryID  int64
}

// Errors returned by the Idea operations. Callers match them with errors.Is.
var (
	ErrEmptyIdeaName = errors.New("idea name must not be empty")
	ErrIdeaNotFound  = errors.New("idea not found")
)

// CreateIdea captures a new Idea with the given name, description, and
// Category. The name is trimmed and must be non-empty; the Category must
// exist.
func (c *Core) CreateIdea(ctx context.Context, in IdeaInput) (Idea, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Idea{}, ErrEmptyIdeaName
	}
	if err := c.requireCategory(ctx, in.CategoryID); err != nil {
		return Idea{}, err
	}
	return c.store.CreateIdea(ctx, name, in.Description, in.CategoryID)
}

// ListIdeas returns every live Idea, in creation order. Ideas never appear
// among Active Projects and are never Do Next candidates — they are a
// separate entity entirely, not a Project lifecycle state.
func (c *Core) ListIdeas(ctx context.Context) ([]Idea, error) {
	return c.store.ListIdeas(ctx)
}

// DeleteIdea plain-deletes a live Idea into the Archive, carrying no link.
// ErrIdeaNotFound if id does not name a live Idea.
func (c *Core) DeleteIdea(ctx context.Context, id int64) error {
	return c.store.ArchiveIdea(ctx, id, c.clock.Now())
}

// PromoteIdea turns a live Idea into a Project: the new Project copies the
// Idea's name, description, and Category and starts in DefaultLifecycle. The
// Idea is then soft-deleted with a link to the resulting Project, in one
// transaction (see Store.PromoteIdea) so a failure partway through never
// leaves an orphan Project or an unlinked Idea. ErrIdeaNotFound if id does not
// name a live Idea.
func (c *Core) PromoteIdea(ctx context.Context, id int64) (Project, error) {
	return c.store.PromoteIdea(ctx, id, DefaultLifecycle, c.clock.Now())
}
