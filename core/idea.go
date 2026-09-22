package core

import (
	"context"
	"errors"
	"strings"
)

// Idea is a lightweight, non-Project capture of something the user might do
// later (CONTEXT.md). See docs/workflows/idea-promotion.md for how it moves
// through capture, browsing, deletion, and promotion.
type Idea struct {
	ID          int64
	Name        string
	Description string
	Notes       string
	CategoryID  int64
	// PromotedProjectID is nil until promotion links this (now archived) Idea
	// to the Project it became.
	PromotedProjectID *int64
}

// IdeaInput carries the user-supplied fields for capturing an Idea.
type IdeaInput struct {
	Name        string
	Description string
	Notes       string
	CategoryID  int64
}

// Errors returned by the Idea operations. Callers match them with errors.Is.
var (
	ErrEmptyIdeaName = errors.New("idea name must not be empty")
	ErrIdeaNotFound  = errors.New("idea not found")
)

// CreateIdea captures a new Idea with the given name, description, notes, and
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
	return c.store.CreateIdea(ctx, name, in.Description, in.Notes, in.CategoryID)
}

// ListIdeas returns every live Idea, in creation order.
func (c *Core) ListIdeas(ctx context.Context) ([]Idea, error) {
	return c.store.ListIdeas(ctx)
}

// DeleteIdea plain-deletes a live Idea into the Archive, carrying no link.
// ErrIdeaNotFound if id does not name a live Idea.
func (c *Core) DeleteIdea(ctx context.Context, id int64) error {
	return c.store.ArchiveIdea(ctx, id, c.clock.Now())
}

// PromoteIdea turns a live Idea into a Project. See
// docs/workflows/idea-promotion.md. ErrIdeaNotFound if id does not name a
// live Idea.
func (c *Core) PromoteIdea(ctx context.Context, id int64) (Project, error) {
	return c.store.PromoteIdea(ctx, id, DefaultLifecycle, c.clock.Now())
}
