package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// schemaVersion reads PRAGMA user_version from db.
func schemaVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	return v
}

func TestMigrateIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "organizer.db")

	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	first := schemaVersion(t, db)
	if first < 1 {
		t.Fatalf("schema version after migrate = %d, want >= 1", first)
	}

	if err := migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if second := schemaVersion(t, db); second != first {
		t.Fatalf("schema version changed on re-run: %d -> %d", first, second)
	}
}

func TestMigrateRejectsNewerDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "organizer.db")

	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Simulate a database written by a future build.
	if _, err := db.Exec("PRAGMA user_version = 9999"); err != nil {
		t.Fatalf("bumping user_version: %v", err)
	}

	err = migrate(db)
	if err == nil {
		t.Fatal("migrate against a newer database returned nil, want error")
	}
	// The message must be actionable: name the offending version and tell the
	// user what to do.
	for _, want := range []string{"9999", "newer", "upgrade"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestLoadMigrationsAreContiguousFromOne(t *testing.T) {
	migs, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(migs) == 0 {
		t.Fatal("no migrations loaded")
	}
	for i, m := range migs {
		if m.version != i+1 {
			t.Errorf("migration %d has version %d, want %d", i, m.version, i+1)
		}
	}
}
