package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/spowers42/project_organizer/core"
)

// taskColumns is the SELECT list for reading a core.Task.
const taskColumns = "id, project_id, milestone_id, title, due_date, notes, done"

// scanTask reads one core.Task from a row-like source. Any lead targets are
// scanned before the taskColumns fields, so callers that prepend extra columns
// (a body listing prepends position) can reuse this.
func scanTask(sc interface{ Scan(...any) error }, lead ...any) (core.Task, error) {
	var (
		t         core.Task
		milestone sql.NullInt64
		due       sql.NullString
	)
	dest := append(append(make([]any, 0, len(lead)+7), lead...),
		&t.ID, &t.ProjectID, &milestone, &t.Title, &due, &t.Notes, &t.Done)
	if err := sc.Scan(dest...); err != nil {
		return core.Task{}, err
	}
	if milestone.Valid {
		id := milestone.Int64
		t.MilestoneID = &id
	}
	if due.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, due.String)
		if err != nil {
			return core.Task{}, fmt.Errorf("parsing due date %q: %w", due.String, err)
		}
		t.DueDate = &parsed
	}
	return t, nil
}

// formatDue renders an optional due date for storage, or a NULL when unset.
func formatDue(due *time.Time) any {
	if due == nil {
		return nil
	}
	return due.UTC().Format(time.RFC3339Nano)
}

// InsertLooseTask appends a loose Task to a Project's body: its position is one
// past the current maximum body slot — across the Project's live loose Tasks and
// Milestones, which share one position space (ADR 0001). milestone_id stays NULL.
// Placement at a chosen spot is a separate WriteBodyOrder call.
func (s *Store) InsertLooseTask(ctx context.Context, projectID int64, title string, dueDate *time.Time, notes string) (core.Task, error) {
	nextPos, err := s.nextBodyPosition(ctx, projectID)
	if err != nil {
		return core.Task{}, err
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO tasks (project_id, title, due_date, notes, done, position) VALUES (?, ?, ?, ?, 0, ?)",
		projectID, title, formatDue(dueDate), notes, nextPos,
	)
	if err != nil {
		return core.Task{}, fmt.Errorf("creating task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return core.Task{}, fmt.Errorf("creating task: %w", err)
	}
	return s.GetTask(ctx, id)
}

// InsertMilestoneTask appends a Task to a Milestone's own ordered list: its
// position is one past the current maximum among that Milestone's live Tasks,
// independent of the Project-body positions. The Task inherits the Milestone's
// project_id. A milestoneID naming no live Milestone is core.ErrMilestoneNotFound.
// Placement at a chosen spot is a separate WriteBodyOrder call.
func (s *Store) InsertMilestoneTask(ctx context.Context, milestoneID int64, title string, dueDate *time.Time, notes string) (core.Task, error) {
	var (
		projectID int64
		nextPos   int64
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT project_id FROM milestones WHERE id = ? AND archived_at IS NULL", milestoneID,
	).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Task{}, core.ErrMilestoneNotFound
	}
	if err != nil {
		return core.Task{}, fmt.Errorf("reading milestone %d: %w", milestoneID, err)
	}

	err = s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(position), -1) + 1 FROM tasks WHERE milestone_id = ? AND archived_at IS NULL", milestoneID,
	).Scan(&nextPos)
	if err != nil {
		return core.Task{}, fmt.Errorf("finding next task position in milestone %d: %w", milestoneID, err)
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO tasks (project_id, milestone_id, title, due_date, notes, done, position) VALUES (?, ?, ?, ?, ?, 0, ?)",
		projectID, milestoneID, title, formatDue(dueDate), notes, nextPos,
	)
	if err != nil {
		return core.Task{}, fmt.Errorf("creating milestone task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return core.Task{}, fmt.Errorf("creating milestone task: %w", err)
	}
	return s.GetTask(ctx, id)
}

// UpdateTask rewrites a live Task's title, due date, and notes.
func (s *Store) UpdateTask(ctx context.Context, id int64, title string, dueDate *time.Time, notes string) (core.Task, error) {
	return s.updateTask(ctx, id,
		"UPDATE tasks SET title = ?, due_date = ?, notes = ? WHERE id = ? AND archived_at IS NULL",
		title, formatDue(dueDate), notes, id,
	)
}

// SetTaskDone sets a live Task's completion flag.
func (s *Store) SetTaskDone(ctx context.Context, id int64, done bool) (core.Task, error) {
	return s.updateTask(ctx, id,
		"UPDATE tasks SET done = ? WHERE id = ? AND archived_at IS NULL",
		done, id,
	)
}

// updateTask runs an UPDATE and returns the refreshed Task, mapping a zero-row
// result to core.ErrTaskNotFound.
func (s *Store) updateTask(ctx context.Context, id int64, query string, args ...any) (core.Task, error) {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return core.Task{}, fmt.Errorf("updating task %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return core.Task{}, fmt.Errorf("updating task %d: %w", id, err)
	}
	if n == 0 {
		return core.Task{}, core.ErrTaskNotFound
	}
	return s.GetTask(ctx, id)
}

// GetTask reads one live Task by id.
func (s *Store) GetTask(ctx context.Context, id int64) (core.Task, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+taskColumns+" FROM tasks WHERE id = ? AND archived_at IS NULL", id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Task{}, core.ErrTaskNotFound
	}
	if err != nil {
		return core.Task{}, fmt.Errorf("getting task %d: %w", id, err)
	}
	return t, nil
}

// ListMilestoneTasks returns a Milestone's live Tasks in Milestone order (by
// ascending position, then id). It is a store-internal helper for ReadBody,
// which attaches each Milestone's Tasks to its body entry.
func (s *Store) ListMilestoneTasks(ctx context.Context, milestoneID int64) ([]core.Task, error) {
	return s.queryTasks(ctx,
		"SELECT "+taskColumns+" FROM tasks WHERE milestone_id = ? AND archived_at IS NULL ORDER BY position, id",
		fmt.Sprintf("listing tasks for milestone %d", milestoneID),
		milestoneID,
	)
}

// queryTasks runs a task SELECT and scans every row into a core.Task slice.
// what labels the operation for wrapped errors.
func (s *Store) queryTasks(ctx context.Context, query, what string, args ...any) ([]core.Task, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	defer func() { _ = rows.Close() }()

	var tasks []core.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return tasks, nil
}
