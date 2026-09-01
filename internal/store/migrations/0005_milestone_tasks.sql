-- Milestone Tasks: a Task may now sit inside a Milestone instead of directly in
-- the Project body. Per ADR 0001 the Project body stays one heterogeneous
-- ordered sequence of loose Tasks and Milestones; a Milestone's own Tasks are a
-- second, Milestone-scoped ordered list that travels with the Milestone.
--
-- `milestone_id` is NULL for a loose Task (its `position` orders it in the
-- Project-body space, shared with `milestones`) and set for a Task inside a
-- Milestone (its `position` then orders it within that Milestone only, starting
-- at 0 and independent of the Project-body positions). A Task belongs to
-- exactly one of the two scopes at a time. The soft-delete convention from 0001
-- is unchanged; every normal query filters `archived_at IS NULL`.

ALTER TABLE tasks ADD COLUMN milestone_id INTEGER REFERENCES milestones(id);

CREATE INDEX idx_tasks_milestone ON tasks (milestone_id, position);
