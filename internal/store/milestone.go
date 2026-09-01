package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spowers42/project_organizer/core"
)

// milestoneColumns is the SELECT list for reading a core.Milestone.
const milestoneColumns = "id, project_id, name"

// scanMilestone reads one core.Milestone from a row-like source. Any lead
// targets are scanned before the milestoneColumns fields, so a body listing can
// prepend position.
func scanMilestone(sc interface{ Scan(...any) error }, lead ...any) (core.Milestone, error) {
	var m core.Milestone
	dest := append(append(make([]any, 0, len(lead)+3), lead...),
		&m.ID, &m.ProjectID, &m.Name)
	if err := sc.Scan(dest...); err != nil {
		return core.Milestone{}, err
	}
	return m, nil
}

// CreateMilestone is the pre-Body-module name for InsertMilestone, kept while
// callers migrate.
func (s *Store) CreateMilestone(ctx context.Context, projectID int64, name string) (core.Milestone, error) {
	return s.InsertMilestone(ctx, projectID, name)
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

// CreateMilestoneAfter inserts a Milestone into a Project's body one place after
// the slot `after` currently holds, shifting that following slot and every later
// one — loose Tasks and Milestones alike, since they share the position space
// (ADR 0001) — a place later. A zero after inserts at the front of the body. A
// non-zero after must name a live body slot of the Project; otherwise its kind's
// not-found sentinel.
func (s *Store) CreateMilestoneAfter(ctx context.Context, projectID int64, after core.BodyRef, name string) (core.Milestone, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Milestone{}, fmt.Errorf("inserting milestone: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var insertPos int64
	if after.ID != 0 {
		pos, err := anchorBodyPosition(ctx, tx, projectID, after)
		if err != nil {
			return core.Milestone{}, err
		}
		insertPos = pos + 1
	}

	if err := shiftBodyPositions(ctx, tx, projectID, insertPos); err != nil {
		return core.Milestone{}, err
	}
	res, err := tx.ExecContext(ctx,
		"INSERT INTO milestones (project_id, name, position) VALUES (?, ?, ?)",
		projectID, name, insertPos,
	)
	if err != nil {
		return core.Milestone{}, fmt.Errorf("inserting milestone: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return core.Milestone{}, fmt.Errorf("inserting milestone: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return core.Milestone{}, fmt.Errorf("inserting milestone: %w", err)
	}
	return s.GetMilestone(ctx, id)
}

// anchorBodyPosition reads the position of a live body slot of a Project — a
// loose Task or a Milestone, per ref.Kind — inside the caller's transaction,
// mapping a missing row to the kind's not-found sentinel.
func anchorBodyPosition(ctx context.Context, tx *sql.Tx, projectID int64, ref core.BodyRef) (int64, error) {
	query := "SELECT position FROM milestones WHERE id = ? AND project_id = ? AND archived_at IS NULL"
	if ref.Kind == core.TaskEntry {
		query = "SELECT position FROM tasks WHERE id = ? AND project_id = ? AND milestone_id IS NULL AND archived_at IS NULL"
	}
	var pos int64
	err := tx.QueryRowContext(ctx, query, ref.ID, projectID).Scan(&pos)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, notFoundErr(ref.Kind)
	}
	if err != nil {
		return 0, fmt.Errorf("reading %s %d position: %w", bodyTable(ref.Kind), ref.ID, err)
	}
	return pos, nil
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
