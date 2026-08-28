-- Tasks: a single actionable step. This ticket only covers loose Tasks — ones
-- sitting directly in a Project's ordered body, not inside a Milestone (those
-- arrive with Milestones in a later ticket).
--
-- `position` orders a Task within its Project body (per ADR 0001 the body is one
-- ordered heterogeneous sequence; for now it holds loose Tasks only). It is
-- assigned on insert as the current max + 1; reordering lands in a later ticket.
-- `due_date` is an optional RFC3339 timestamp. `done` is the completion flag,
-- unconstrained in order relative to other Tasks. `archived_at` follows the
-- soft-delete convention from 0001 even though archiving a single Task lands
-- later; every normal query already filters `archived_at IS NULL`.

CREATE TABLE tasks (
    id          INTEGER PRIMARY KEY,
    project_id  INTEGER NOT NULL REFERENCES projects(id),
    title       TEXT NOT NULL,
    due_date    TEXT,
    done        INTEGER NOT NULL DEFAULT 0,
    position    INTEGER NOT NULL,
    archived_at TEXT
);

CREATE INDEX idx_tasks_project ON tasks (project_id, position);
