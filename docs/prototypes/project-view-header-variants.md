# Project view header — three variants to compare (issue #48)

All three keep the same footer and body-list-of-Tasks section; they differ
only in how much of the Project's own fields sit in the header vs. the body.

---

## Variant A — Header summary (what the original #48 mockup proposed)

Header collapses Lifecycle + Priority into one summary line; Description and
Category move down into the body as its opening content.

```
Learn Rust                                              [Active]  ★ starred

Description: Work through the book, one chapter a week
Category:    Course

Body:
> Read chapter 4                              (due 2026-09-15)
  Milestone: Finish the book
    Write the final exercise

↑/↓: select   space: toggle done   a: add Task   e: edit   ?: help
```

Tradeoff: shortest header (2 lines + status), most "at a glance" — but
Description/Category now scroll away with the Task list on a long body,
and un-starred/non-Active projects lose that visual distinction unless you
look at the header line.

---

## Variant B — Everything stays visible, all in the header

None of the four fields move — the header just becomes the fixed identity
block it already almost is today, unchanged in content, just formally
"header zone" instead of undifferentiated top-of-screen text.

```
Learn Rust
Description: Work through the book, one chapter a week
Category:    Course
Lifecycle:   Active                                          ★ starred

Body:
> Read chapter 4                              (due 2026-09-15)
  Milestone: Finish the book
    Write the final exercise

↑/↓: select   space: toggle done   a: add Task   e: edit   ?: help
```

Tradeoff: nothing about the Project ever scrolls out of view — Description,
Category, Lifecycle, Priority are always on screen, at the cost of a taller
header (4 lines + status = up to 5) that eats into body space on a short
terminal, and it's the least "trimmed" of the three, working against the
header being a quick-glance zone.

---

## Variant C — Compromise: identity + state in header, fields stay but compact

Header carries name + the two state markers (matches Variant A's top line),
but Description/Category get one shared compact line instead of moving to
the body — so they're still always visible, just not each on their own line.

```
Learn Rust                                              [Active]  ★ starred
Course — Work through the book, one chapter a week

Body:
> Read chapter 4                              (due 2026-09-15)
  Milestone: Finish the book
    Write the final exercise

↑/↓: select   space: toggle done   a: add Task   e: edit   ?: help
```

Tradeoff: everything stays always-visible like B, but in 2 lines instead of
4 — closest to a middle ground. Cost: Description gets truncated on a long
one and Category+Description now read as one merged phrase, which can blur
which is which for a new/rare project without a Description.

---

## With a status line (all three, using Variant C as the example)

```
Learn Rust                                              [Active]  ★ starred
Course — Work through the book, one chapter a week
Saved.

Body:
...
```

Same status-line rule as the original #48 mockup: header grows to 3 lines
only while there's something to say, then shrinks back.
