# Project Organizer

A local-only TUI for keeping track of larger personal projects across many
domains (programming, hobbies, online classes), so nothing in flight gets
silently lost. It shows what is currently being worked on and, in moments of
indecision, hands the user one concrete task to start on.

## Language

**Project**:
A larger, multi-step undertaking the user is tracking. Its body is a single
user-ordered sequence whose entries are either Tasks (sitting directly on the
Project) or Milestones. The user places every entry by hand, so a loose Task can
come before, between, or after Milestones.
_Avoid_: initiative, effort

**Task**:
A single actionable step. Sits either directly in a Project's ordered body or
inside a Milestone. May carry an optional due date and an optional Priority.
Can be completed in any order; its position only feeds the Next step hint.
_Avoid_: todo, item, action item

**Milestone**:
An optional, ordered grouping of Tasks, marking a meaningful chunk of progress.
A Milestone occupies one slot in the Project's ordered body and its Tasks travel
with it. Any Project can add Milestones; some Projects have none. A Milestone is
considered complete once it has at least one Task and all its Tasks are done
(the user is asked to confirm, and may decline to keep it open for more Tasks).
_Avoid_: phase, epic, stage

**Category**:
The label that classifies a Project or an Idea (e.g. Programming, Course). One
shared list across both, seeded with `Programming`, `Course`, `Other` and
extendable by the user. Used for grouping and filtering. A Category cannot be
deleted while any Project or Idea still references it.
_Avoid_: area, type, tag, project type

**Archive**:
The holding state for anything soft-deleted — Projects, Milestones, Tasks,
Ideas. Archived entities are hidden from every normal view (both TUI screens) and
reachable only through the `archive` CLI subcommand, which can restore them or
purge them for good. There is no other undo.
_Avoid_: trash, recycle bin, deleted

**Idea**:
A lightweight, non-Project capture of something the user might do later: name,
description, category. Not actionable and never appears in Do Next. Can be
promoted into a Project (carrying its fields over), which soft-deletes the Idea
with a link to the resulting Project. Plain deletion also soft-deletes it.
_Avoid_: note, backlog item, someday

**Next step**:
The single Task a given Project is waiting on right now. Walk the Project's
ordered body to the first incomplete entry: if it is a loose Task, that is the
Next step; if it is a Milestone, the Next step is the first incomplete Task
inside it.
_Avoid_: current task

**Do Next**:
The dashboard's suggestion for what to work on in a moment of indecision. The
candidate pool is the Next step of each Active Project (one per Project); one is
chosen by weighted random, biased toward approaching due dates, Priority, and
(later) categories that are going stale.
_Avoid_: recommended task, suggested action

**Priority**:
A boolean "star" on a Task or Project marking it as wanting attention. Not a
numeric scale. Weights the Do Next pick and offers a sort order; carries no
other behaviour.
_Avoid_: importance, severity, rank

## Project lifecycle states

**Active**:
Currently being worked on. This is what "in flight" on the dashboard means.

**Paused**:
Started, but deliberately on hold.

**Someday**:
Backlog the user has not committed to starting.

**Done**:
Completed.

**Abandoned**:
Deliberately dropped without completing. Kept distinct from Done so the
completed list stays honest.
