-- Projects: a larger, multi-step undertaking the user is tracking.
--
-- `lifecycle` holds one of the five states from CONTEXT.md (Active, Paused,
-- Someday, Done, Abandoned) verbatim; a newly created Project starts Active,
-- the documented default. `archived_at` follows the soft-delete convention
-- established in 0001 even though archiving Projects lands in a later ticket;
-- every normal query already filters `archived_at IS NULL`.

CREATE TABLE projects (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category_id INTEGER NOT NULL REFERENCES categories(id),
    lifecycle   TEXT NOT NULL,
    archived_at TEXT
);

CREATE INDEX idx_projects_category ON projects (category_id);
CREATE INDEX idx_projects_lifecycle ON projects (lifecycle);
