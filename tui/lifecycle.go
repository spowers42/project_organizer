package tui

import "github.com/spowers42/project_organizer/core"

// lifecycleOrder lists the five lifecycle states in display order. It is the
// single in-package source for every lifecycle picker and filter, so a change
// to the set (unlikely — the spec fixes it) touches one line.
var lifecycleOrder = []core.Lifecycle{
	core.Active, core.Paused, core.Someday, core.Done, core.Abandoned,
}

// lifecycleLabels renders lifecycleOrder as picker rows.
func lifecycleLabels() []string {
	labels := make([]string, len(lifecycleOrder))
	for i, l := range lifecycleOrder {
		labels[i] = string(l)
	}
	return labels
}

// lifecycleIndex is the row in lifecycleOrder for state, or 0 when state is not
// one of the five.
func lifecycleIndex(state core.Lifecycle) int {
	for i, l := range lifecycleOrder {
		if l == state {
			return i
		}
	}
	return 0
}

// lifecycleFilterRows are the rows of the lifecycle filter picker: a leading
// "All" (no constraint, the empty Lifecycle) followed by lifecycleOrder. The
// returned slices are parallel.
func lifecycleFilterRows() (labels []string, values []core.Lifecycle) {
	labels = make([]string, 0, len(lifecycleOrder)+1)
	values = make([]core.Lifecycle, 0, len(lifecycleOrder)+1)
	labels = append(labels, "All")
	values = append(values, "")
	for _, l := range lifecycleOrder {
		labels = append(labels, string(l))
		values = append(values, l)
	}
	return labels, values
}
