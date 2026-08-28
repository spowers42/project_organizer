-- Milestones: an optional, ordered grouping of Tasks marking a meaningful chunk
-- of progress. Per ADR 0001 a Project body is one heterogeneous ordered
-- sequence, so a Milestone shares the `position` space with the Project's loose
-- Tasks rather than living in a separate list: a slot's position is unique
-- across `tasks` and `milestones` for a given project, and a body listing merges
-- the two by ascending position.
--
-- `position` is assigned on insert as the current body max + 1 (across both
-- tables); reordering swaps two slots' positions. Inner Tasks arrive in a later
-- ticket, so a Milestone may be empty here. `archived_at` follows the
-- soft-delete convention from 0001; every normal query filters
-- `archived_at IS NULL`.

CREATE TABLE milestones (
    id          INTEGER PRIMARY KEY,
    project_id  INTEGER NOT NULL REFERENCES projects(id),
    name        TEXT NOT NULL,
    position    INTEGER NOT NULL,
    archived_at TEXT
);

CREATE INDEX idx_milestones_project ON milestones (project_id, position);
