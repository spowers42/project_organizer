package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/spowers42/project_organizer/core"
)

// projectColumns is the SELECT list for reading a core.Project.
const projectColumns = "id, name, description, category_id, lifecycle"

// scanProject reads one core.Project from a row-like source.
func scanProject(sc interface{ Scan(...any) error }) (core.Project, error) {
	var (
		p         core.Project
		lifecycle string
	)
	if err := sc.Scan(&p.ID, &p.Name, &p.Description, &p.CategoryID, &lifecycle); err != nil {
		return core.Project{}, err
	}
	p.Lifecycle = core.Lifecycle(lifecycle)
	return p, nil
}

// CategoryExists reports whether a Category row with id exists.
func (s *Store) CategoryExists(ctx context.Context, id int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM categories WHERE id = ?", id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking category %d: %w", id, err)
	}
	return true, nil
}

// CreateProject inserts a Project and returns it as stored.
func (s *Store) CreateProject(ctx context.Context, name, description string, categoryID int64, lifecycle core.Lifecycle) (core.Project, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO projects (name, description, category_id, lifecycle) VALUES (?, ?, ?, ?)",
		name, description, categoryID, string(lifecycle),
	)
	if err != nil {
		return core.Project{}, fmt.Errorf("creating project: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return core.Project{}, fmt.Errorf("creating project: %w", err)
	}
	return s.GetProject(ctx, id)
}

// UpdateProject rewrites a live Project's name, description, and Category.
func (s *Store) UpdateProject(ctx context.Context, id int64, name, description string, categoryID int64) (core.Project, error) {
	return s.updateProject(ctx, id,
		"UPDATE projects SET name = ?, description = ?, category_id = ? WHERE id = ? AND archived_at IS NULL",
		name, description, categoryID, id,
	)
}

// UpdateProjectLifecycle moves a live Project to lifecycle.
func (s *Store) UpdateProjectLifecycle(ctx context.Context, id int64, lifecycle core.Lifecycle) (core.Project, error) {
	return s.updateProject(ctx, id,
		"UPDATE projects SET lifecycle = ? WHERE id = ? AND archived_at IS NULL",
		string(lifecycle), id,
	)
}

// ArchiveProject soft-deletes a live Project by stamping archived_at. A Project
// that is missing or already archived yields core.ErrProjectNotFound.
func (s *Store) ArchiveProject(ctx context.Context, id int64, at time.Time) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE projects SET archived_at = ? WHERE id = ? AND archived_at IS NULL",
		at.UTC().Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("archiving project %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("archiving project %d: %w", id, err)
	}
	if n == 0 {
		return core.ErrProjectNotFound
	}
	return nil
}

// updateProject runs an UPDATE and returns the refreshed Project, mapping a
// zero-row result to core.ErrProjectNotFound.
func (s *Store) updateProject(ctx context.Context, id int64, query string, args ...any) (core.Project, error) {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return core.Project{}, fmt.Errorf("updating project %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return core.Project{}, fmt.Errorf("updating project %d: %w", id, err)
	}
	if n == 0 {
		return core.Project{}, core.ErrProjectNotFound
	}
	return s.GetProject(ctx, id)
}

// GetProject reads one live Project by id.
func (s *Store) GetProject(ctx context.Context, id int64) (core.Project, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+projectColumns+" FROM projects WHERE id = ? AND archived_at IS NULL", id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Project{}, core.ErrProjectNotFound
	}
	if err != nil {
		return core.Project{}, fmt.Errorf("getting project %d: %w", id, err)
	}
	return p, nil
}

// ListProjects returns live Projects in creation order, optionally narrowed by
// lifecycle (when non-empty) and category (when non-zero).
func (s *Store) ListProjects(ctx context.Context, lifecycle core.Lifecycle, categoryID int64) ([]core.Project, error) {
	query := "SELECT " + projectColumns + " FROM projects WHERE archived_at IS NULL"
	var args []any
	if lifecycle != "" {
		query += " AND lifecycle = ?"
		args = append(args, string(lifecycle))
	}
	if categoryID != 0 {
		query += " AND category_id = ?"
		args = append(args, categoryID)
	}
	query += " ORDER BY id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []core.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	return projects, nil
}
