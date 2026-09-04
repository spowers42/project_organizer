-- Milestone completion acknowledgement: "complete" stays a derived property
-- (core.milestoneComplete — non-empty and every Task done), never stored.
-- `completion_ack` instead records whether the user has already been prompted
-- for the Milestone's *current* completion, so the "just completed" signal
-- fires once. It resets to 0 whenever the Milestone's Task set changes in a
-- way that could produce a fresh completion — a Task added to it, or one of
-- its Tasks un-completed — so a later re-completion prompts again.

ALTER TABLE milestones ADD COLUMN completion_ack INTEGER NOT NULL DEFAULT 0;
