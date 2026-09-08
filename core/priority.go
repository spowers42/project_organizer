package core

import "context"

// Priority is a boolean "star" on a Task or a Project (CONTEXT.md), marking it
// as wanting attention. It is not a numeric scale. Today it feeds one thing: the
// list views offer a Priority-first sort order over it (ProjectsByPriority,
// TasksByPriority). Per CONTEXT.md it is also meant to weight the Do Next pick
// once that exists. It never reorders a stored Project body, which stays
// hand-ordered (ADR 0001).

// SetProjectPriority sets a Project's Priority star on (priority true) or off.
// The call is idempotent — setting the value it already holds is not an error.
// ErrProjectNotFound if id does not name a live Project.
func (c *Core) SetProjectPriority(ctx context.Context, id int64, priority bool) (Project, error) {
	return c.store.SetProjectPriority(ctx, id, priority)
}

// SetTaskPriority sets a Task's Priority star on (priority true) or off. The
// call is idempotent — setting the value it already holds is not an error.
// ErrTaskNotFound if id does not name a live Task.
func (c *Core) SetTaskPriority(ctx context.Context, id int64, priority bool) (Task, error) {
	return c.store.SetTaskPriority(ctx, id, priority)
}

// ProjectsByPriority returns projects reordered Priority-first: every starred
// Project in its original relative order, then every unstarred one in theirs (a
// stable partition). It is purely a view ordering — the input slice is left
// untouched and nothing is persisted, so a Priority-first list never disturbs
// stored order.
func ProjectsByPriority(projects []Project) []Project {
	return priorityFirst(projects, func(p Project) bool { return p.Priority })
}

// TasksByPriority is ProjectsByPriority for a Task list: starred Tasks first,
// stable within each group, input untouched, nothing persisted. The Project
// body's stored order (ADR 0001) is unaffected.
func TasksByPriority(tasks []Task) []Task {
	return priorityFirst(tasks, func(t Task) bool { return t.Priority })
}

// priorityFirst returns a new slice with the items starred reports true for
// first, then the rest, each group keeping its incoming order.
func priorityFirst[T any](items []T, starred func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, it := range items {
		if starred(it) {
			out = append(out, it)
		}
	}
	for _, it := range items {
		if !starred(it) {
			out = append(out, it)
		}
	}
	return out
}
