-- Archive cascade tracking: archiving a Project cascades to its live
-- Milestones and Tasks, and archiving a Milestone cascades to its live Tasks
-- (see docs/workflows/archive-cascade.md). Every row swept into one archive
-- call shares an `archive_batch` identifier so restore and purge can act on
-- exactly that cascade rather than blindly on every child of the parent — a
-- Task archived on its own, before or after some unrelated cascade, must not
-- be pulled back in when that cascade is restored.
--
-- The batch identifier is the kind and id of whichever entity was the root of
-- the archive call, e.g. "project:12" or "milestone:5"; a Task or Idea
-- archived on its own carries its own kind:id as a batch of one. It is
-- meaningful only while archived_at is set.

ALTER TABLE projects   ADD COLUMN archive_batch TEXT;
ALTER TABLE milestones ADD COLUMN archive_batch TEXT;
ALTER TABLE tasks      ADD COLUMN archive_batch TEXT;
ALTER TABLE ideas      ADD COLUMN archive_batch TEXT;
