package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/spowers42/project_organizer/core"
)

func TestPromoteIdeaRecordsPromotedProjectLink(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "organizer.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cats, err := st.ListCategories(ctx)
	if err != nil || len(cats) == 0 {
		t.Fatalf("ListCategories: cats=%v err=%v", cats, err)
	}
	catID := cats[0].ID

	idea, err := st.CreateIdea(ctx, "Build a shed", "in the backyard", "needs pressure-treated lumber", catID)
	if err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	project, err := st.PromoteIdea(ctx, idea.ID, core.DefaultLifecycle, time.Now())
	if err != nil {
		t.Fatalf("PromoteIdea: %v", err)
	}
	if project.Name != idea.Name || project.Description != idea.Description || project.CategoryID != idea.CategoryID {
		t.Errorf("promoted project = %+v, want fields copied from %+v", project, idea)
	}

	var (
		archivedAt sql.NullString
		linked     sql.NullInt64
	)
	row := st.db.QueryRowContext(ctx, "SELECT archived_at, promoted_project_id FROM ideas WHERE id = ?", idea.ID)
	if err := row.Scan(&archivedAt, &linked); err != nil {
		t.Fatalf("scanning archived idea row: %v", err)
	}
	if !archivedAt.Valid {
		t.Error("archived_at is NULL, want it stamped")
	}
	if !linked.Valid || linked.Int64 != project.ID {
		t.Errorf("promoted_project_id = %+v, want %d", linked, project.ID)
	}

	if _, err := st.GetIdea(ctx, idea.ID); err != core.ErrIdeaNotFound {
		t.Errorf("GetIdea after promotion = %v, want ErrIdeaNotFound", err)
	}
}

func TestPromoteIdeaUnknownIdeaIsNotFound(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "organizer.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := st.PromoteIdea(ctx, 99999, core.DefaultLifecycle, time.Now()); err != core.ErrIdeaNotFound {
		t.Errorf("err = %v, want ErrIdeaNotFound", err)
	}
}
