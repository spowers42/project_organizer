package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spowers42/project_organizer/core"
)

// archiveBatch is the cascade batch identifier for an archive call rooted at
// the given kind and id: the kind and id themselves, e.g. "project:12". Every
// row an archive call touches — the root plus whatever it cascades to — is
// stamped with the same value, so RestoreArchived and PurgeArchived can later
// act on exactly that group. See docs/workflows/archive-cascade.md.
func archiveBatch(kind core.ArchiveKind, id int64) string {
	return string(kind) + ":" + strconv.FormatInt(id, 10)
}

// archiveTable is the table backing an ArchiveKind.
func archiveTable(kind core.ArchiveKind) (table string, ok bool) {
	switch kind {
	case core.ArchivedProject:
		return "projects", true
	case core.ArchivedMilestone:
		return "milestones", true
	case core.ArchivedTask:
		return "tasks", true
	case core.ArchivedIdea:
		return "ideas", true
	default:
		return "", false
	}
}

// archiveLabelColumn is the column holding an ArchivedEntity's display Label
// for an ArchiveKind: Projects, Milestones, and Ideas use "name"; Tasks use
// "title".
func archiveLabelColumn(kind core.ArchiveKind) string {
	if kind == core.ArchivedTask {
		return "title"
	}
	return "name"
}

// ArchiveMilestone soft-deletes a live Milestone by stamping archived_at, and
// cascades to its live Tasks, stamping every row with the same cascade batch
// in one transaction. See docs/workflows/archive-cascade.md. A Milestone that
// is missing or already archived yields core.ErrMilestoneNotFound.
func (s *Store) ArchiveMilestone(ctx context.Context, id int64, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("archiving milestone %d: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	ts := at.UTC().Format(time.RFC3339Nano)
	batch := archiveBatch(core.ArchivedMilestone, id)

	res, err := tx.ExecContext(ctx,
		"UPDATE milestones SET archived_at = ?, archive_batch = ? WHERE id = ? AND archived_at IS NULL",
		ts, batch, id,
	)
	if err != nil {
		return fmt.Errorf("archiving milestone %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("archiving milestone %d: %w", id, err)
	}
	if n == 0 {
		return core.ErrMilestoneNotFound
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE tasks SET archived_at = ?, archive_batch = ? WHERE milestone_id = ? AND archived_at IS NULL",
		ts, batch, id,
	); err != nil {
		return fmt.Errorf("cascading archive to tasks of milestone %d: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("archiving milestone %d: %w", id, err)
	}
	return nil
}

// ArchiveTask soft-deletes a live Task by stamping archived_at. A Task has no
// children, so this never cascades. A Task that is missing or already
// archived yields core.ErrTaskNotFound.
func (s *Store) ArchiveTask(ctx context.Context, id int64, at time.Time) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE tasks SET archived_at = ?, archive_batch = ? WHERE id = ? AND archived_at IS NULL",
		at.UTC().Format(time.RFC3339Nano), archiveBatch(core.ArchivedTask, id), id,
	)
	if err != nil {
		return fmt.Errorf("archiving task %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("archiving task %d: %w", id, err)
	}
	if n == 0 {
		return core.ErrTaskNotFound
	}
	return nil
}

// ListArchived returns every archived row across projects, milestones, tasks,
// and ideas, newest archived first.
func (s *Store) ListArchived(ctx context.Context) ([]core.ArchivedEntity, error) {
	var out []core.ArchivedEntity
	for _, kind := range []core.ArchiveKind{core.ArchivedProject, core.ArchivedMilestone, core.ArchivedTask, core.ArchivedIdea} {
		table, _ := archiveTable(kind)
		rows, err := s.db.QueryContext(ctx,
			"SELECT id, "+archiveLabelColumn(kind)+", archived_at FROM "+table+" WHERE archived_at IS NOT NULL",
		)
		if err != nil {
			return nil, fmt.Errorf("listing archived %s: %w", table, err)
		}
		err = func() error {
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var (
					id    int64
					label string
					atStr string
				)
				if err := rows.Scan(&id, &label, &atStr); err != nil {
					return fmt.Errorf("scanning archived %s: %w", table, err)
				}
				at, err := time.Parse(time.RFC3339Nano, atStr)
				if err != nil {
					return fmt.Errorf("parsing archived_at %q: %w", atStr, err)
				}
				out = append(out, core.ArchivedEntity{
					Ref:        core.ArchiveRef{Kind: kind, ID: id},
					Label:      label,
					ArchivedAt: at,
				})
			}
			return rows.Err()
		}()
		if err != nil {
			return nil, err
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].ArchivedAt.Equal(out[j].ArchivedAt) {
			return out[i].ArchivedAt.After(out[j].ArchivedAt)
		}
		if out[i].Ref.Kind != out[j].Ref.Kind {
			return out[i].Ref.Kind < out[j].Ref.Kind
		}
		return out[i].Ref.ID < out[j].Ref.ID
	})
	return out, nil
}

// batchFor reads the archive_batch of the currently-archived row named by
// ref. core.ErrArchivedNotFound if ref names no archived row.
func (s *Store) batchFor(ctx context.Context, ref core.ArchiveRef) (string, error) {
	table, ok := archiveTable(ref.Kind)
	if !ok {
		return "", core.ErrArchivedNotFound
	}
	var batch sql.NullString
	err := s.db.QueryRowContext(ctx,
		"SELECT archive_batch FROM "+table+" WHERE id = ? AND archived_at IS NOT NULL", ref.ID,
	).Scan(&batch)
	if errors.Is(err, sql.ErrNoRows) {
		return "", core.ErrArchivedNotFound
	}
	if err != nil {
		return "", fmt.Errorf("reading archive batch for %s %d: %w", table, ref.ID, err)
	}
	if batch.Valid && batch.String != "" {
		return batch.String, nil
	}
	// Every archive path stamps a batch; this is a defensive fallback for a
	// row archived with none, treating it as a batch of one.
	return archiveBatch(ref.Kind, ref.ID), nil
}

// archiveCascadeTables lists the tables a cascade batch can span, in the order
// safe for deletion: children before the parents that FOREIGN KEY references
// them (tasks before milestones before projects); ideas reference nothing
// that also archives, so their position does not matter.
var archiveCascadeTables = []string{"tasks", "milestones", "projects", "ideas"}

// RestoreArchived reverses the archive call that archived ref's row: every row
// across all four tables sharing its cascade batch is un-archived in one
// transaction. See docs/workflows/archive-cascade.md.
// core.ErrArchivedNotFound if ref does not name a currently archived row.
func (s *Store) RestoreArchived(ctx context.Context, ref core.ArchiveRef) error {
	batch, err := s.batchFor(ctx, ref)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("restoring %v: %w", ref, err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range archiveCascadeTables {
		if _, err := tx.ExecContext(ctx,
			"UPDATE "+table+" SET archived_at = NULL, archive_batch = NULL WHERE archive_batch = ? AND archived_at IS NOT NULL",
			batch,
		); err != nil {
			return fmt.Errorf("restoring %s in batch %s: %w", table, batch, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("restoring %v: %w", ref, err)
	}
	return nil
}

// PurgeArchived permanently deletes ref's row along with every row sharing its
// cascade batch — symmetric with RestoreArchived. Deletes run child tables
// before parents so FOREIGN KEY constraints (enforced per-connection, see
// Open) are never violated. See docs/workflows/archive-cascade.md.
// core.ErrArchivedNotFound if ref does not name a currently archived row.
func (s *Store) PurgeArchived(ctx context.Context, ref core.ArchiveRef) error {
	batch, err := s.batchFor(ctx, ref)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("purging %v: %w", ref, err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range archiveCascadeTables {
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM "+table+" WHERE archive_batch = ? AND archived_at IS NOT NULL",
			batch,
		); err != nil {
			return fmt.Errorf("purging %s in batch %s: %w", table, batch, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("purging %v: %w", ref, err)
	}
	return nil
}
