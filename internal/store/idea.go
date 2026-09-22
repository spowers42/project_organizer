package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/spowers42/project_organizer/core"
)

// ideaColumns is the SELECT list for reading a core.Idea.
const ideaColumns = "id, name, description, category_id, promoted_project_id"

// scanIdea reads one core.Idea from a row-like source.
func scanIdea(sc interface{ Scan(...any) error }) (core.Idea, error) {
	var (
		i        core.Idea
		promoted sql.NullInt64
	)
	if err := sc.Scan(&i.ID, &i.Name, &i.Description, &i.CategoryID, &promoted); err != nil {
		return core.Idea{}, err
	}
	if promoted.Valid {
		id := promoted.Int64
		i.PromotedProjectID = &id
	}
	return i, nil
}

// CreateIdea inserts an Idea and returns it as stored.
func (s *Store) CreateIdea(ctx context.Context, name, description string, categoryID int64) (core.Idea, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO ideas (name, description, category_id) VALUES (?, ?, ?)",
		name, description, categoryID,
	)
	if err != nil {
		return core.Idea{}, fmt.Errorf("creating idea: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return core.Idea{}, fmt.Errorf("creating idea: %w", err)
	}
	return s.GetIdea(ctx, id)
}

// GetIdea reads one live Idea by id.
func (s *Store) GetIdea(ctx context.Context, id int64) (core.Idea, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+ideaColumns+" FROM ideas WHERE id = ? AND archived_at IS NULL", id)
	i, err := scanIdea(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Idea{}, core.ErrIdeaNotFound
	}
	if err != nil {
		return core.Idea{}, fmt.Errorf("getting idea %d: %w", id, err)
	}
	return i, nil
}

// ListIdeas returns live Ideas in creation order.
func (s *Store) ListIdeas(ctx context.Context) ([]core.Idea, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+ideaColumns+" FROM ideas WHERE archived_at IS NULL ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("listing ideas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ideas []core.Idea
	for rows.Next() {
		i, err := scanIdea(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning idea: %w", err)
		}
		ideas = append(ideas, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing ideas: %w", err)
	}
	return ideas, nil
}

// ArchiveIdea soft-deletes a live Idea by stamping archived_at, carrying no
// link — the plain-delete path. An Idea that is missing or already archived
// yields core.ErrIdeaNotFound.
func (s *Store) ArchiveIdea(ctx context.Context, id int64, at time.Time) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE ideas SET archived_at = ? WHERE id = ? AND archived_at IS NULL",
		at.UTC().Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("archiving idea %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("archiving idea %d: %w", id, err)
	}
	if n == 0 {
		return core.ErrIdeaNotFound
	}
	return nil
}

// PromoteIdea turns a live Idea into a Project in one transaction: it copies
// the Idea's name, description, and Category into a new Project at lifecycle,
// then stamps the Idea's archived_at and promoted_project_id to link it to
// that Project. Reports core.ErrIdeaNotFound when id does not name a live
// Idea; the transaction is rolled back on any failure, so a partial promotion
// never persists.
func (s *Store) PromoteIdea(ctx context.Context, id int64, lifecycle core.Lifecycle, at time.Time) (core.Project, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Project{}, fmt.Errorf("promoting idea %d: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		"SELECT "+ideaColumns+" FROM ideas WHERE id = ? AND archived_at IS NULL", id)
	idea, err := scanIdea(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Project{}, core.ErrIdeaNotFound
	}
	if err != nil {
		return core.Project{}, fmt.Errorf("reading idea %d: %w", id, err)
	}

	res, err := tx.ExecContext(ctx,
		"INSERT INTO projects (name, description, category_id, lifecycle) VALUES (?, ?, ?, ?)",
		idea.Name, idea.Description, idea.CategoryID, string(lifecycle),
	)
	if err != nil {
		return core.Project{}, fmt.Errorf("creating project from idea %d: %w", id, err)
	}
	projectID, err := res.LastInsertId()
	if err != nil {
		return core.Project{}, fmt.Errorf("creating project from idea %d: %w", id, err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE ideas SET archived_at = ?, promoted_project_id = ? WHERE id = ?",
		at.UTC().Format(time.RFC3339Nano), projectID, id,
	); err != nil {
		return core.Project{}, fmt.Errorf("archiving promoted idea %d: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return core.Project{}, fmt.Errorf("promoting idea %d: %w", id, err)
	}

	return s.GetProject(ctx, projectID)
}
