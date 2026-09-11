# Zone-convention prototype — issue #48

Throwaway mockups, not code. Answers: what exactly goes in header / body /
footer, for both screens, an open overlay, the help overlay, Do Next, and a
narrow terminal. `[R]` marks reverse-video (selection), `[B]` marks bold — both
schematic stand-ins; the real palette is ticket #51's job.

Convention settled by these mockups:
- **Header** = 1–3 lines: title/identity line, then a one-line context/summary
  line, then — only when non-empty — the status/error line, in that order.
  The header's line count is *dynamic* (2 lines normally, 3 when there's a
  status to show), not a fixed-height band with blank filler.
- **Body** = everything else down to the footer: the scrollable/interactive
  content. An open overlay renders *inside* the body, replacing the screen's
  own body content one-for-one; the header stays untouched (it's about "where
  am I", which doesn't change while the overlay does its work) but the header's
  status line clears when an overlay opens, since the overlay hasn't produced
  a new status yet.
- **Footer** = exactly 1 line while ownership is with the screen: the trimmed
  command set. When an overlay is open, the footer is fully replaced by that
  overlay's own 1-line command set — never both.
- A trailing blank line separates body from footer everywhere, for a
  consistent visual seam.

---

## 1. Dashboard, normal state

```
Project Organizer
Showing: Active Projects (in flight)

  ★ Ship the deck build          [Active]
      Next step: Sand the top rails
> Learn Rust                     [Active]
      Next step: Finish chapter 4 exercises
  Kitchen re-tile                [Active]
      Next step: Order tile samples

↑/↓: select   enter: open   n: new   d: Do Next   ?: help
```

## 1b. Dashboard, normal state, with a status line

```
Project Organizer
Showing: Active Projects (in flight)
Priority updated.

  ★ Ship the deck build          [Active]
...
↑/↓: select   enter: open   n: new   d: Do Next   ?: help
```

---

## 2. Dashboard, create-Project form overlay open

Header unchanged (still "where am I"), status line cleared, footer fully
swapped to the overlay's own commands.

```
Project Organizer
Showing: Active Projects (in flight)

  New Project
  ─────────────────────────────────────
  Name:        [                      ]
  Description: [                      ]
  Category:    Programming ▾

tab: next field   enter: save   esc: cancel
```

---

## 3. Project view, normal state — DECIDED (Variant A, star-only marker)

```
Learn Rust                                                    [Active]  ★

Description: Work through the book, one chapter a week
Category:    Course

Body:
> Read chapter 4                              (due 2026-09-15)
  Milestone: Finish the book
    Write the final exercise

↑/↓: select   space: toggle done   a: add Task   e: edit   ?: help
```

Header carries identity + one-line state summary (`[Active]  ★`) in place of
the four stacked `Description:/Category:/Lifecycle:/Priority:` lines the code
prints today; `Description`/`Category` move into the body as the top of its
content, since they're project *content*, not "where am I" framing. The
Priority marker in the header is the star alone — no "starred" word — matching
the compact `priorityStar` marker used everywhere else; when unstarred, it
renders as nothing (no marker, no placeholder glyph), same as `priorityStar`'s
existing on/off behavior elsewhere. This replaces `projectview.go`'s current
`priorityLabel` helper (`"★ starred"` / `"—"`), which no longer applies once
Priority moves into this header line.

Two variants considered and rejected: keeping all four fields always visible
in the header (grows the header to 4–5 lines, eating into body space), and a
compromise merging Description+Category onto one shared line (blurs which
field is which). Full comparison: see `project-view-header-variants.md` on
this ticket's prototype branch.

## 3b. Project view, with a status line

```
Learn Rust                                                    [Active]  ★
Saved.

Description: Work through the book, one chapter a week
...
```

---

## 4. `?` help overlay, open over the Project view

Same overlay-in-body rule as #2: header stays, status clears, footer swaps to
"esc: close".

```
Learn Rust                                              [Active]  ★ starred

  All commands
  ─────────────────────────────────────
  Navigate        ↑/↓ or j/k
  Reorder         shift+↑/↓
  Toggle done     space
  Add Task        a         Add Task to Milestone   A
  Add Milestone   m
  Edit Task       t         Star Task               p
  Move into MS    >         Move out to body         <
  Edit Project    e         Star Project             P
  Set lifecycle   s         Archive                  d
  Sort            o (Priority-first, display only)
  Back            esc / b   Quit                     q

esc: close
```

(The grouping above is illustrative — the actual command split into
footer-worthy vs. help-only is ticket #50's job, not this one.)

---

## 5. Do Next, restructured into the convention

Today `renderDoNext` is a bespoke full-screen replacement drawn over the
dashboard. Refit: it becomes a **body-zone overlay** on the dashboard, exactly
like the create-Project form — same rule as #2, so nothing new is invented.

```
Project Organizer
Showing: Active Projects (in flight)

  Do Next
  ─────────────────────────────────────
  Sand the top rails
    in ★ Ship the deck build

r: reroll   esc: back
```

The empty-pool and error cases stay body content, same footer:

```
Project Organizer
Showing: Active Projects (in flight)

  Do Next
  ─────────────────────────────────────
  Nothing to do next — no Active Project has a Next step.

r: reroll   esc: back
```

---

## 6. Narrow terminal (~50 cols)

Zones hold up unchanged — header/body/footer don't reflow into each other,
content inside the body just wraps or truncates:

```
Project Organizer
Showing: Active Projects (in flight)

  ★ Ship the deck build
      [Active]
      Next step: Sand the top rails
> Learn Rust
      [Active]
      Next step: Finish chapter 4
      exercises

↑/↓: select  enter: open  ?: help
```

The `[Lifecycle]` tag drops to its own line under the name rather than
sharing the row, and the footer keeps its command list but wraps to a second
line only if it must — it stays 1 *logical* line of hints, wrapped by the
terminal, never a footer the screen itself grows to two lines.

This also sets the baseline for the two-column dashboard (#49): at this width,
a second column has nowhere to go, so #49 will need its own minimum-width
call (e.g. collapse to Projects-only below some threshold) — flagged there,
not resolved here.
