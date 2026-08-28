-- Initial schema for project_organizer.
--
-- Every entity that can be archived carries a nullable archived_at timestamp;
-- by convention every normal query filters `archived_at IS NULL`. Categories
-- are not archivable (they are protected by reference-count instead), so they
-- have no archived_at column.

CREATE TABLE categories (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- Seed the shared Category list. Extendable by the user later.
INSERT INTO categories (name) VALUES
    ('Programming'),
    ('Course'),
    ('Other');
