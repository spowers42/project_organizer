package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spowers42/project_organizer/core"
)

// milestoneColumns is the SELECT list for reading a core.Milestone.
const milestoneColumns = "id, project_id, name, completion_ack"

// scanMilestone reads one core.Milestone from a row-like source. Any lead
// targets are scanned before the milestoneColumns fields, so a body listing can
// prepend position.
func scanMilestone(sc interface{ Scan(...any) error }, lead ...any) (core.Milestone, error) {
	var m core.Milestone
	dest := append(append(make([]any, 0, len(lead)+4), lead...),
		&m.ID, &m.ProjectID, &m.Name, &m.CompletionAcked)
	if err := sc.Scan(dest...); err != nil {
		return core.Milestone{}, err
	}
	return m, nil
}

// InsertMilestone appends a Milestone to a Project's body: its position is one
// past the current maximum body slot, shared with the Project's loose Tasks.
// Placement at a chosen spot is a separate WriteBodyOrder call.
func (s *Store) InsertMilestone(ctx context.Context, projectID int64, name string) (core.Milestone, error) {
	nextPos, err := s.nextBodyPosition(ctx, projectID)
	if err != nil {
		return core.Milestone{}, err
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO milestones (project_id, name, position) VALUES (?, ?, ?)",
		projectID, name, nextPos,
	)
	if err != nil {
		return core.Milestone{}, fmt.Errorf("creating milestone: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return core.Milestone{}, fmt.Errorf("creating milestone: %w", err)
	}
	return s.GetMilestone(ctx, id)
}

// GetMilestone reads one live Milestone by id.
func (s *Store) GetMilestone(ctx context.Context, id int64) (core.Milestone, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+milestoneColumns+" FROM milestones WHERE id = ? AND archived_at IS NULL", id)
	m, err := scanMilestone(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Milestone{}, core.ErrMilestoneNotFound
	}
	if err != nil {
		return core.Milestone{}, fmt.Errorf("getting milestone %d: %w", id, err)
	}
	return m, nil
}

// SetMilestoneCompletionAck rewrites a live Milestone's completion_ack column.
func (s *Store) SetMilestoneCompletionAck(ctx context.Context, id int64, acked bool) (core.Milestone, error) {
	res, err := s.db.ExecContext(ctx,
		"UPDATE milestones SET completion_ack = ? WHERE id = ? AND archived_at IS NULL",
		acked, id,
	)
	if err != nil {
		return core.Milestone{}, fmt.Errorf("updating milestone %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return core.Milestone{}, fmt.Errorf("updating milestone %d: %w", id, err)
	}
	if n == 0 {
		return core.Milestone{}, core.ErrMilestoneNotFound
	}
	return s.GetMilestone(ctx, id)
}
