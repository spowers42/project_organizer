// Package store owns the SQLite database: connection, embedded schema
// migrations, and query methods. It holds no domain logic and is module-internal
// — application code reaches persistence through the core package; main only
// touches it to build the concrete Store it injects into core.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spowers42/project_organizer/core"

	_ "modernc.org/sqlite" // CGo-free SQLite driver, registered as "sqlite".
)

// Store is a handle to the opened database.
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) the SQLite database at path, creating any
// missing parent directories, applies all pending migrations, and returns a
// ready Store. The caller owns the returned Store and must Close it.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating data directory %s: %w", dir, err)
		}
	}

	// _pragma args apply per connection; foreign_keys must be on for every one.
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", path, err)
	}
	db.SetMaxOpenConns(1) // single local user; serialize to avoid SQLITE_BUSY.

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to database %s: %w", path, err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// ListCategories returns every Category, ordered by id (seed order first).
func (s *Store) ListCategories(ctx context.Context) ([]core.Category, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name FROM categories ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cats []core.Category
	for rows.Next() {
		var c core.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		cats = append(cats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	return cats, nil
}
