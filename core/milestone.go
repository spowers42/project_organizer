package core

import (
	"context"
	"errors"
	"strings"
)

// Milestone is an optional, ordered grouping of Tasks marking a meaningful chunk
// of progress. It occupies one slot in a Project's ordered body (per ADR 0001
// the body is one heterogeneous sequence, not a separate Milestone list). Its
// inner Tasks arrive in a later ticket, so a Milestone may be empty here.
type Milestone struct {
	ID        int64
	ProjectID int64
	Name      string
}

// Errors returned by the Milestone operations. Callers match them with
// errors.Is; the entrypoints turn them into user-facing messages.
var (
	ErrEmptyMilestoneName = errors.New("milestone name must not be empty")
	ErrMilestoneNotFound  = errors.New("milestone not found")
)

// AddMilestone appends a Milestone to the end of a Project's body. The name is
// trimmed and must be non-empty. ErrProjectNotFound if projectID does not name a
// live Project.
func (c *Core) AddMilestone(ctx context.Context, projectID int64, name string) (Milestone, error) {
	if _, err := c.store.GetProject(ctx, projectID); err != nil {
		return Milestone{}, err
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return Milestone{}, ErrEmptyMilestoneName
	}
	return c.store.CreateMilestone(ctx, projectID, trimmed)
}

// ProjectBody returns a Project's ordered body: its loose Tasks and Milestones
// interleaved in stored order. ErrProjectNotFound if projectID does not name a
// live Project.
func (c *Core) ProjectBody(ctx context.Context, projectID int64) ([]BodyEntry, error) {
	if _, err := c.store.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	return c.store.ListProjectBody(ctx, projectID)
}

// MoveMilestone reorders a Milestone one slot earlier (MoveUp) or later
// (MoveDown) within its Project body, swapping with whichever entry — a loose
// Task or another Milestone — currently sits in that slot. Moving the first
// entry up or the last entry down is a no-op. It returns the Project's body in
// the resulting order. ErrInvalidMove if dir is neither direction;
// ErrMilestoneNotFound if id does not name a live Milestone.
func (c *Core) MoveMilestone(ctx context.Context, id int64, dir MoveDir) ([]BodyEntry, error) {
	if dir != MoveUp && dir != MoveDown {
		return nil, ErrInvalidMove
	}
	m, err := c.store.GetMilestone(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.moveBodyEntry(ctx, m.ProjectID, BodyRef{Kind: MilestoneEntry, ID: id}, dir)
}
