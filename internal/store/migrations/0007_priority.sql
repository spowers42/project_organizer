-- Priority: a boolean "star" on a Task and on a Project marking it as wanting
-- attention (CONTEXT.md). It is a flag, not a numeric scale: 0 is unstarred, 1
-- is starred, and every existing row defaults to unstarred.
--
-- Priority offers a Priority-first sort order for the list views (and per
-- CONTEXT.md is meant to weight the Do Next pick once that exists). That sort is
-- a view concern computed in memory, so no query orders on this column and it is
-- left unindexed; a stored Project body stays hand-ordered (ADR 0001) regardless
-- of any star.

ALTER TABLE tasks ADD COLUMN priority INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN priority INTEGER NOT NULL DEFAULT 0;
