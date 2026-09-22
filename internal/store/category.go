package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spowers42/project_organizer/core"
)

// CreateCategory inserts a Category and returns it as stored.
func (s *Store) CreateCategory(ctx context.Context, name string) (core.Category, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO categories (name) VALUES (?)", name)
	if err != nil {
		return core.Category{}, fmt.Errorf("creating category: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return core.Category{}, fmt.Errorf("creating category: %w", err)
	}
	return core.Category{ID: id, Name: name}, nil
}

// RenameCategory rewrites a Category's name. Reports core.ErrCategoryNotFound
// when no row matches.
func (s *Store) RenameCategory(ctx context.Context, id int64, name string) (core.Category, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE categories SET name = ? WHERE id = ?", name, id)
	if err != nil {
		return core.Category{}, fmt.Errorf("renaming category %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return core.Category{}, fmt.Errorf("renaming category %d: %w", id, err)
	}
	if n == 0 {
		return core.Category{}, core.ErrCategoryNotFound
	}
	return core.Category{ID: id, Name: name}, nil
}

// CategoryReferenced reports whether any Project or Idea — including archived
// ones — references the Category via category_id.
func (s *Store) CategoryReferenced(ctx context.Context, id int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 WHERE EXISTS (SELECT 1 FROM projects WHERE category_id = ?)
		    OR EXISTS (SELECT 1 FROM ideas WHERE category_id = ?)`,
		id, id,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking references to category %d: %w", id, err)
	}
	return true, nil
}

// DeleteCategory removes a Category row. Reports core.ErrCategoryNotFound
// when no row matches.
func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM categories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting category %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deleting category %d: %w", id, err)
	}
	if n == 0 {
		return core.ErrCategoryNotFound
	}
	return nil
}
