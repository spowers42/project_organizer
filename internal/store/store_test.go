package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenCreatesParentDirsAndReopens(t *testing.T) {
	// A path several levels below the temp dir: Open must create the chain.
	dbPath := filepath.Join(t.TempDir(), "a", "b", "project_organizer", "organizer.db")

	first, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("closing first: %v", err)
	}

	// Second run reuses the existing file without error.
	second, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	cats, err := second.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats) != 3 {
		t.Fatalf("categories after reopen = %d, want 3 (no re-seed)", len(cats))
	}
}

func TestListCategoriesSeededInOrder(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "organizer.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cats, err := st.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}

	want := []string{"Programming", "Course", "Other"}
	if len(cats) != len(want) {
		t.Fatalf("got %d categories, want %d", len(cats), len(want))
	}
	for i, w := range want {
		if cats[i].Name != w {
			t.Errorf("category %d = %q, want %q", i, cats[i].Name, w)
		}
	}
}

func TestDefaultDBPathUsesXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/xdg/data")

	got, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	want := filepath.Join("/xdg/data", "project_organizer", "organizer.db")
	if got != want {
		t.Errorf("DefaultDBPath() = %q, want %q", got, want)
	}
}

func TestDefaultDBPathFallsBackToLocalShare(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "") // unset / empty -> fall back
	t.Setenv("HOME", "/home/tester")

	got, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	want := filepath.Join("/home/tester", ".local", "share", "project_organizer", "organizer.db")
	if got != want {
		t.Errorf("DefaultDBPath() = %q, want %q", got, want)
	}
}

func TestDefaultDBPathIgnoresRelativeXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "relative/path") // spec: must be absolute
	t.Setenv("HOME", "/home/tester")

	got, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	want := filepath.Join("/home/tester", ".local", "share", "project_organizer", "organizer.db")
	if got != want {
		t.Errorf("DefaultDBPath() = %q, want %q", got, want)
	}
}
