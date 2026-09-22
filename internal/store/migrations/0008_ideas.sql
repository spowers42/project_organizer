-- Ideas: a lightweight, non-Project capture (CONTEXT.md). See
-- docs/workflows/idea-promotion.md for the capture/browse/delete/promote flow.
-- `archived_at` follows the soft-delete convention from 0001.

CREATE TABLE ideas (
    id                   INTEGER PRIMARY KEY,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    notes                TEXT NOT NULL DEFAULT '',
    category_id          INTEGER NOT NULL REFERENCES categories(id),
    promoted_project_id  INTEGER REFERENCES projects(id),
    archived_at          TEXT
);

CREATE INDEX idx_ideas_category ON ideas (category_id);
