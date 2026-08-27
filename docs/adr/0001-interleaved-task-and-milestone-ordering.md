# Interleaved Task and Milestone ordering within a Project

A Project's body is a single user-ordered sequence whose entries are
*heterogeneous*: each entry is either a loose Task or a Milestone (which itself
holds ordered Tasks). We chose this over the more common shape of a separate
Task list and a separate Milestone list, because real projects have setup or
blocking Tasks that must sit *before* the first Milestone, and forcing every
Task into a Milestone (or into a bucket that always renders after Milestones)
misrepresents the actual order of work.

## Consequences

- Ordering, drag/move logic, and Next-step resolution all operate over a mixed
  entry type rather than a flat Task list. Moving an entry reorders within its
  level only; crossing the loose/Milestone boundary is an explicit action.
- Reversing this (going to bucketed lists) would touch the schema and every
  piece of ordering and Next-step code, so it is effectively a rewrite of the
  Project core.
