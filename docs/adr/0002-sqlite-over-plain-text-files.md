# SQLite for storage, not hand-editable plain-text files

Persistence is a single SQLite database file under `~/.local/share/`, not a tree
of TOML/Markdown/JSON files. Plain-text storage is attractive for a personal
tool — git-diffable, editable in any editor without the app — but this app's
core operations are ordered heterogeneous lists, cross-entity scans (Do Next
walks every Active Project), soft-delete/restore, and staleness timestamps. Those
are cheap and correct in SQL and fiddly-and-bug-prone with a bespoke file parser
and serializer. The TUI is the intended editor, so hand-editability is not a
real loss.

## Consequences

- No git history of content changes and no external-editor workflow. The
  `archive` CLI subcommand and soft-delete provide the only recovery path.
- Schema changes require embedded migrations gated on a `schema_version`.
