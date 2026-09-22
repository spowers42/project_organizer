-- Ideas: a lightweight, non-Project capture of something the user might do
-- later (CONTEXT.md) — name, description, Category. Never actionable and never
-- a Do Next candidate, so it lives in its own table rather than as a Project
-- lifecycle state.
--
-- `promoted_project_id` is NULL until the Idea is promoted; promotion sets it
-- alongside `archived_at` in the same soft-delete, linking the archived Idea to
-- the Project it became. A plain delete sets `archived_at` and leaves this NULL.
-- `archived_at` follows the soft-delete convention from 0001; every normal
-- query filters `archived_at IS NULL`.

CREATE TABLE ideas (
    id                   INTEGER PRIMARY KEY,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    category_id          INTEGER NOT NULL REFERENCES categories(id),
    promoted_project_id  INTEGER REFERENCES projects(id),
    archived_at          TEXT
);

CREATE INDEX idx_ideas_category ON ideas (category_id);
