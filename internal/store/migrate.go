package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migration is a single versioned schema change loaded from migrations/.
type migration struct {
	version int
	name    string
	body    string
}

// label is the human-readable identity of a migration, e.g. "0001_init".
func (m migration) label() string {
	return fmt.Sprintf("%04d_%s", m.version, m.name)
}

// loadMigrations reads every migrations/NNNN_name.sql file, ordered by version.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations: %w", err)
	}

	migs := make([]migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix, rest, ok := strings.Cut(strings.TrimSuffix(e.Name(), ".sql"), "_")
		if !ok {
			return nil, fmt.Errorf("migration %q: name must be NNNN_description.sql", e.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q: %q is not a numeric version", e.Name(), prefix)
		}
		if version < 1 {
			return nil, fmt.Errorf("migration %q: version must be >= 1", e.Name())
		}
		body, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("reading migration %q: %w", e.Name(), err)
		}
		migs = append(migs, migration{version: version, name: rest, body: string(body)})
	}

	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	for i, m := range migs {
		want := i + 1
		if m.version != want {
			return nil, fmt.Errorf("migration versions must be contiguous from 1: expected %d, got %d", want, m.version)
		}
	}
	return migs, nil
}

// migrate brings db up to the latest embedded schema version. It is idempotent:
// on an already-current database it makes no changes and returns nil.
//
// The applied version is tracked in SQLite's `PRAGMA user_version`. A database
// whose version is ahead of this binary is a hard error: an older build must not
// touch a newer schema.
func migrate(db *sql.DB) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	if len(migs) == 0 {
		return fmt.Errorf("no embedded migrations found")
	}
	latest := migs[len(migs)-1].version

	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if current > latest {
		return fmt.Errorf(
			"database schema version %d is newer than this build of project_organizer supports (%d); upgrade to a newer release",
			current, latest,
		)
	}

	for _, m := range migs {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration %s: %w", m.label(), err)
		}
		if _, err := tx.Exec(m.body); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("applying migration %s: %w", m.label(), err)
		}
		// PRAGMA does not accept bind parameters; version is a validated int.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording schema version for migration %s: %w", m.label(), err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", m.label(), err)
		}
	}
	return nil
}
