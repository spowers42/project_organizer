package core_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/spowers42/project_organizer/core"
	"github.com/spowers42/project_organizer/internal/store"
)

// fixedNow is the wall-clock time every test starts at.
var fixedNow = time.Date(2024, time.January, 2, 15, 4, 5, 0, time.UTC)

// fakeClock is a fixed, mutable Clock for tests: newTestCore returns the
// *fakeClock so a test can move time by assigning now.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

// newTestCore returns a Core wired to a fresh temp-file SQLite database, a fake
// Clock fixed at fixedNow, and a Rand seeded to a constant. The returned
// *fakeClock is the same one inside the Core, so tests that care about time can
// move it. The database is closed automatically when the test ends.
func newTestCore(t *testing.T) (*core.Core, *fakeClock) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "organizer.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("closing test store: %v", err)
		}
	})

	clock := &fakeClock{now: fixedNow}
	return core.New(st, clock, core.NewRand(1)), clock
}
