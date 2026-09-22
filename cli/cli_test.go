package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
	"github.com/spowers42/project_organizer/internal/store"
)

// newTestCore returns a Core wired to a fresh temp-file SQLite database, for
// smoke-testing RunArchive against real core behavior.
func newTestCore(t *testing.T) *core.Core {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "organizer.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("closing test store: %v", err)
		}
	})
	return core.New(st, core.SystemClock{}, core.NewRand(1))
}

// categoryID returns the id of the seeded Category named name.
func categoryID(t *testing.T, c *core.Core, name string) int64 {
	t.Helper()
	cats, err := c.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	for _, cat := range cats {
		if cat.Name == name {
			return cat.ID
		}
	}
	t.Fatalf("seeded category %q not found", name)
	return 0
}

// mustArchiveProject creates and immediately archives a Project, returning
// its ArchiveRef.
func mustArchiveProject(t *testing.T, c *core.Core, name string) core.ArchiveRef {
	t.Helper()
	ctx := context.Background()
	p, err := c.CreateProject(ctx, core.ProjectInput{
		Name:        name,
		Description: "desc",
		CategoryID:  categoryID(t, c, "Other"),
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := c.ArchiveProject(ctx, p.ID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	return core.ArchiveRef{Kind: core.ArchivedProject, ID: p.ID}
}

func TestRunArchiveListPrintsEntities(t *testing.T) {
	c := newTestCore(t)
	ref := mustArchiveProject(t, c, "shelved")

	var stdout, stderr bytes.Buffer
	if err := RunArchive(c, []string{"list"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunArchive(list): %v", err)
	}
	if !strings.Contains(stdout.String(), formatArchiveRef(ref)) {
		t.Errorf("stdout = %q, want it to contain %q", stdout.String(), formatArchiveRef(ref))
	}
	if !strings.Contains(stdout.String(), "shelved") {
		t.Errorf("stdout = %q, want it to contain the Project's label", stdout.String())
	}
}

func TestRunArchiveListReportsEmptyArchive(t *testing.T) {
	c := newTestCore(t)

	var stdout, stderr bytes.Buffer
	if err := RunArchive(c, []string{"list"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunArchive(list): %v", err)
	}
	if !strings.Contains(stdout.String(), "empty") {
		t.Errorf("stdout = %q, want it to say the Archive is empty", stdout.String())
	}
}

func TestRunArchiveRestoreCallsCoreWithParsedRef(t *testing.T) {
	c := newTestCore(t)
	ref := mustArchiveProject(t, c, "comeback")

	var stdout, stderr bytes.Buffer
	if err := RunArchive(c, []string{"restore", formatArchiveRef(ref)}, &stdout, &stderr); err != nil {
		t.Fatalf("RunArchive(restore): %v", err)
	}
	if !strings.Contains(stdout.String(), formatArchiveRef(ref)) {
		t.Errorf("stdout = %q, want it to mention %q", stdout.String(), formatArchiveRef(ref))
	}

	// The restored Project should be back among live Projects, and gone from
	// the Archive.
	p, err := c.GetProject(context.Background(), ref.ID)
	if err != nil {
		t.Fatalf("GetProject after restore: %v", err)
	}
	if p.Name != "comeback" {
		t.Errorf("GetProject after restore = %+v, want Name = comeback", p)
	}
	entries, err := c.ListArchived(context.Background())
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	for _, e := range entries {
		if e.Ref == ref {
			t.Errorf("ListArchived still contains %v after restore", ref)
		}
	}
}

func TestRunArchivePurgeCallsCoreWithParsedRef(t *testing.T) {
	c := newTestCore(t)
	ref := mustArchiveProject(t, c, "gone-for-good")

	var stdout, stderr bytes.Buffer
	if err := RunArchive(c, []string{"purge", formatArchiveRef(ref)}, &stdout, &stderr); err != nil {
		t.Fatalf("RunArchive(purge): %v", err)
	}
	if !strings.Contains(stdout.String(), formatArchiveRef(ref)) {
		t.Errorf("stdout = %q, want it to mention %q", stdout.String(), formatArchiveRef(ref))
	}

	entries, err := c.ListArchived(context.Background())
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	for _, e := range entries {
		if e.Ref == ref {
			t.Errorf("ListArchived still contains %v after purge", ref)
		}
	}
}

func TestRunArchiveRestoreRejectsMalformedID(t *testing.T) {
	c := newTestCore(t)

	var stdout, stderr bytes.Buffer
	if err := RunArchive(c, []string{"restore", "not-an-id"}, &stdout, &stderr); err == nil {
		t.Fatal("RunArchive(restore, malformed id) = nil, want error")
	}
}

func TestRunArchivePurgeRejectsUnknownEntity(t *testing.T) {
	c := newTestCore(t)

	var stdout, stderr bytes.Buffer
	err := RunArchive(c, []string{"purge", "project:99999"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("RunArchive(purge, unknown entity) = nil, want error")
	}
}

func TestRunArchiveRejectsMissingAndUnknownCommands(t *testing.T) {
	tests := map[string][]string{
		"no command":      {},
		"unknown command": {"bogus"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := RunArchive(nil, args, &stdout, &stderr); err == nil {
				t.Fatalf("RunArchive(%q) = nil, want error", args)
			}
			if !strings.Contains(stderr.String(), "usage:") {
				t.Errorf("stderr = %q, want usage text", stderr.String())
			}
		})
	}
}

func TestRunArchiveRestoreAndPurgeRejectWrongArgCount(t *testing.T) {
	c := newTestCore(t)
	for _, cmd := range []string{"restore", "purge"} {
		t.Run(cmd, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := RunArchive(c, []string{cmd}, &stdout, &stderr); err == nil {
				t.Fatalf("RunArchive(%q, no id) = nil, want error", cmd)
			}
			if !strings.Contains(stderr.String(), "usage:") {
				t.Errorf("stderr = %q, want usage text", stderr.String())
			}
		})
	}
}
