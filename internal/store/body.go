package store

import (
	"context"
	"fmt"
	"sort"

	"github.com/spowers42/project_organizer/core"
)

// bodyTable is the table backing a body-slot kind. Both tables carry id,
// project_id, position, and archived_at, so WriteBodyOrder can pick the table
// from a BodyRef's kind.
func bodyTable(kind core.BodyEntryKind) string {
	if kind == core.MilestoneEntry {
		return "milestones"
	}
	return "tasks"
}

// nextBodyPosition is one past the highest position currently used by a
// Project's live body slots, across both loose Tasks and Milestones. A Project
// with an empty body starts at 0.
func (s *Store) nextBodyPosition(ctx context.Context, projectID int64) (int64, error) {
	var next int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position), -1) + 1 FROM (
			SELECT position FROM tasks      WHERE project_id = ? AND milestone_id IS NULL AND archived_at IS NULL
			UNION ALL
			SELECT position FROM milestones WHERE project_id = ? AND archived_at IS NULL
		)`,
		projectID, projectID,
	).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("finding next body position for project %d: %w", projectID, err)
	}
	return next, nil
}

// positionedEntry pairs a body entry with its stored position, the key
// ReadBody merges the two slot tables on.
type positionedEntry struct {
	pos   int64
	entry core.BodyEntry
}

// ReadBody returns a Project's ordered body: its live loose Tasks and
// Milestones merged by ascending position, each Milestone carrying its own
// ordered Tasks. A Project assigns each new slot a position past every existing
// one, so positions are distinct in practice; the comparator still breaks an
// equal-position tie by kind then id so the order is never left to chance.
func (s *Store) ReadBody(ctx context.Context, projectID int64) ([]core.BodyEntry, error) {
	tasks, err := s.listBodyTasks(ctx, projectID)
	if err != nil {
		return nil, err
	}
	milestones, err := s.listBodyMilestones(ctx, projectID)
	if err != nil {
		return nil, err
	}
	// Attach each Milestone's own ordered Tasks so the body carries the whole
	// nested structure. Done here, once the listing queries above have closed
	// their rows, so these follow-up reads get the single connection.
	for _, pe := range milestones {
		mTasks, err := s.ListMilestoneTasks(ctx, pe.entry.Milestone.ID)
		if err != nil {
			return nil, err
		}
		pe.entry.Milestone.Tasks = mTasks
	}

	merged := append(tasks, milestones...)
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].pos != merged[j].pos {
			return merged[i].pos < merged[j].pos
		}
		ri, rj := merged[i].entry.Ref(), merged[j].entry.Ref()
		if ri.Kind != rj.Kind {
			return ri.Kind < rj.Kind
		}
		return ri.ID < rj.ID
	})

	body := make([]core.BodyEntry, len(merged))
	for i, m := range merged {
		body[i] = m.entry
	}
	return body, nil
}

// WriteBodyOrder rewrites the stored order of a Project's body from an
// in-memory ordering: each top-level slot (loose Task or Milestone) and each
// Milestone's own Tasks are renumbered 0..N-1 to match the given sequence, in
// one transaction. The order is expected to name exactly the Project's live
// slots — it comes from a freshly loaded Body — so rows not mentioned are left
// alone. It is the single persistence call for every reorder and every
// insert-at-a-position.
func (s *Store) WriteBodyOrder(ctx context.Context, projectID int64, order core.BodyOrder) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("writing body order for project %d: %w", projectID, err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, ref := range order.Slots {
		if _, err := tx.ExecContext(ctx,
			"UPDATE "+bodyTable(ref.Kind)+" SET position = ? WHERE id = ? AND project_id = ? AND archived_at IS NULL",
			i, ref.ID, projectID,
		); err != nil {
			return fmt.Errorf("ordering %s %d: %w", bodyTable(ref.Kind), ref.ID, err)
		}
	}
	for milestoneID, taskIDs := range order.MilestoneTasks {
		for j, taskID := range taskIDs {
			if _, err := tx.ExecContext(ctx,
				"UPDATE tasks SET position = ? WHERE id = ? AND milestone_id = ? AND archived_at IS NULL",
				j, taskID, milestoneID,
			); err != nil {
				return fmt.Errorf("ordering task %d in milestone %d: %w", taskID, milestoneID, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("writing body order for project %d: %w", projectID, err)
	}
	return nil
}

// listBodyTasks reads a Project's live loose Tasks — those not inside a
// Milestone — as positioned body entries.
func (s *Store) listBodyTasks(ctx context.Context, projectID int64) ([]positionedEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT position, "+taskColumns+" FROM tasks WHERE project_id = ? AND milestone_id IS NULL AND archived_at IS NULL",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing body tasks for project %d: %w", projectID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []positionedEntry
	for rows.Next() {
		var pos int64
		task, err := scanTask(rows, &pos)
		if err != nil {
			return nil, fmt.Errorf("scanning body task: %w", err)
		}
		entry := task
		out = append(out, positionedEntry{
			pos:   pos,
			entry: core.BodyEntry{Kind: core.TaskEntry, Task: &entry},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing body tasks for project %d: %w", projectID, err)
	}
	return out, nil
}

// listBodyMilestones reads a Project's live Milestones as positioned body
// entries.
func (s *Store) listBodyMilestones(ctx context.Context, projectID int64) ([]positionedEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT position, "+milestoneColumns+" FROM milestones WHERE project_id = ? AND archived_at IS NULL",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing body milestones for project %d: %w", projectID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []positionedEntry
	for rows.Next() {
		var pos int64
		m, err := scanMilestone(rows, &pos)
		if err != nil {
			return nil, fmt.Errorf("scanning body milestone: %w", err)
		}
		entry := m
		out = append(out, positionedEntry{
			pos:   pos,
			entry: core.BodyEntry{Kind: core.MilestoneEntry, Milestone: &entry},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing body milestones for project %d: %w", projectID, err)
	}
	return out, nil
}
