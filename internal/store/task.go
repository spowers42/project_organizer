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
const taskColumns = "id, project_id, title, due_date, notes, done"

// scanTask reads one core.Task from a row-like source.
func scanTask(sc interface{ Scan(...any) error }) (core.Task, error) {
	var (
		t   core.Task
		due sql.NullString
	)
	if err := sc.Scan(&t.ID, &t.ProjectID, &t.Title, &due, &t.Notes, &t.Done); err != nil {
		return core.Task{}, err
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

// CreateTask appends a loose Task to a Project's body: its position is one past
// the current maximum among that Project's live Tasks.
func (s *Store) CreateTask(ctx context.Context, projectID int64, title string, dueDate *time.Time, notes string) (core.Task, error) {
	var nextPos int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(position), -1) + 1 FROM tasks WHERE project_id = ? AND archived_at IS NULL",
		projectID,
	).Scan(&nextPos)
	if err != nil {
		return core.Task{}, fmt.Errorf("finding next task position for project %d: %w", projectID, err)
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

// SwapTaskPositions exchanges the body positions of two live loose Tasks in a
// single transaction, so a reorder never leaves the body with a duplicated or
// skipped position. Either id matching no live row is core.ErrTaskNotFound and
// rolls the swap back.
func (s *Store) SwapTaskPositions(ctx context.Context, firstID, secondID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("swapping task positions: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	firstPos, err := livePosition(ctx, tx, firstID)
	if err != nil {
		return err
	}
	secondPos, err := livePosition(ctx, tx, secondID)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE tasks SET position = ? WHERE id = ?", secondPos, firstID,
	); err != nil {
		return fmt.Errorf("moving task %d: %w", firstID, err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE tasks SET position = ? WHERE id = ?", firstPos, secondID,
	); err != nil {
		return fmt.Errorf("moving task %d: %w", secondID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("swapping task positions: %w", err)
	}
	return nil
}

// livePosition reads one live Task's body position within a transaction,
// mapping a missing row to core.ErrTaskNotFound.
func livePosition(ctx context.Context, tx *sql.Tx, id int64) (int64, error) {
	var pos int64
	err := tx.QueryRowContext(ctx,
		"SELECT position FROM tasks WHERE id = ? AND archived_at IS NULL", id,
	).Scan(&pos)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, core.ErrTaskNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("reading task %d position: %w", id, err)
	}
	return pos, nil
}

// ListProjectTasks returns a Project's live loose Tasks in body order (by
// ascending position, then id to break ties deterministically).
func (s *Store) ListProjectTasks(ctx context.Context, projectID int64) ([]core.Task, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+taskColumns+" FROM tasks WHERE project_id = ? AND archived_at IS NULL ORDER BY position, id",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing tasks for project %d: %w", projectID, err)
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
		return nil, fmt.Errorf("listing tasks for project %d: %w", projectID, err)
	}
	return tasks, nil
}
