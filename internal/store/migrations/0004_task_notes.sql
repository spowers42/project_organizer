-- Tasks gain an optional freeform notes field: somewhere to jot context that
-- does not belong in the one-line title. It is plain text, may span multiple
-- lines, and defaults to empty (no notes). NOT NULL keeps the Go side a plain
-- string rather than a nullable one.

ALTER TABLE tasks ADD COLUMN notes TEXT NOT NULL DEFAULT '';
