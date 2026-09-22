package core

import (
	"context"
	"errors"
	"time"
)

// ArchiveKind identifies which entity kind an archived row belongs to, using
// the CONTEXT.md glossary terms verbatim.
type ArchiveKind string

// The four archivable entity kinds.
const (
	ArchivedProject   ArchiveKind = "Project"
	ArchivedMilestone ArchiveKind = "Milestone"
	ArchivedTask      ArchiveKind = "Task"
	ArchivedIdea      ArchiveKind = "Idea"
)

// ArchiveRef identifies one archived entity by kind and id — the identifier
// ListArchived rows carry and RestoreArchived / PurgeArchived take.
type ArchiveRef struct {
	Kind ArchiveKind
	ID   int64
}

// ArchivedEntity is one row currently sitting in the Archive. Label is its
// most descriptive field as stored — a Project's or Milestone's name, a
// Task's title, or an Idea's name.
type ArchivedEntity struct {
	Ref        ArchiveRef
	Label      string
	ArchivedAt time.Time
}

// ErrArchivedNotFound is returned by RestoreArchived and PurgeArchived when
// ref does not name a currently archived entity.
var ErrArchivedNotFound = errors.New("archived entity not found")

// ArchiveMilestone soft-deletes a Milestone into the Archive, cascading to its
// live Tasks — they leave every normal view together. See
// docs/workflows/archive-cascade.md. ErrMilestoneNotFound if id does not name
// a live Milestone.
func (c *Core) ArchiveMilestone(ctx context.Context, id int64) error {
	return c.store.ArchiveMilestone(ctx, id, c.clock.Now())
}

// ArchiveTask soft-deletes a single Task into the Archive. A Task has no
// children, so this never cascades. ErrTaskNotFound if id does not name a
// live Task.
func (c *Core) ArchiveTask(ctx context.Context, id int64) error {
	return c.store.ArchiveTask(ctx, id, c.clock.Now())
}

// ListArchived returns every entity currently in the Archive, across all four
// kinds — the data an `archive list` view shows.
func (c *Core) ListArchived(ctx context.Context) ([]ArchivedEntity, error) {
	return c.store.ListArchived(ctx)
}

// RestoreArchived reverses exactly the rows archived together with ref in one
// archive call — not every child the parent has acquired since. See
// docs/workflows/archive-cascade.md. ErrArchivedNotFound if ref does not name
// a currently archived entity.
func (c *Core) RestoreArchived(ctx context.Context, ref ArchiveRef) error {
	return c.store.RestoreArchived(ctx, ref)
}

// PurgeArchived permanently deletes ref's row, along with every row archived
// together with it in the same cascade — symmetric with RestoreArchived, so
// what comes back together also goes for good together. See
// docs/workflows/archive-cascade.md. ErrArchivedNotFound if ref does not name
// a currently archived entity.
func (c *Core) PurgeArchived(ctx context.Context, ref ArchiveRef) error {
	return c.store.PurgeArchived(ctx, ref)
}
