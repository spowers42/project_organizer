package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// lifecycleFilterOptions are the rows of the lifecycle filter picker, paired
// with the core.Lifecycle each selects. The empty Lifecycle ("All") means no
// constraint.
var lifecycleFilterOptions = []struct {
	label string
	value core.Lifecycle
}{
	{"All", ""},
	{"Active", core.Active},
	{"Paused", core.Paused},
	{"Someday", core.Someday},
	{"Done", core.Done},
	{"Abandoned", core.Abandoned},
}

// filterForm chooses a lifecycle-state and/or Category constraint for the
// dashboard's Project list. It holds no Core; the dashboard reads filter() and
// re-queries.
type filterForm struct {
	lifecycle picker
	category  picker
	catIDs    []int64 // parallel to category rows; 0 == "All"
	focus     int     // 0 == lifecycle, 1 == category
}

// newFilterForm builds the overlay pre-set to current so reopening it shows
// the constraint already in effect.
func newFilterForm(categories []core.Category, current core.ProjectFilter) filterForm {
	lifeLabels := make([]string, len(lifecycleFilterOptions))
	lifeStart := 0
	for i, o := range lifecycleFilterOptions {
		lifeLabels[i] = o.label
		if o.value == current.Lifecycle {
			lifeStart = i
		}
	}

	catLabels := make([]string, 0, len(categories)+1)
	catIDs := make([]int64, 0, len(categories)+1)
	catLabels = append(catLabels, "All")
	catIDs = append(catIDs, 0)
	catStart := 0
	for i, c := range categories {
		catLabels = append(catLabels, c.Name)
		catIDs = append(catIDs, c.ID)
		if c.ID == current.CategoryID {
			catStart = i + 1
		}
	}

	return filterForm{
		lifecycle: newPicker(lifeLabels, lifeStart),
		category:  newPicker(catLabels, catStart),
		catIDs:    catIDs,
	}
}

// update advances the overlay for one key. done is true once the user applies
// (applied true) or cancels (applied false).
func (f *filterForm) update(msg tea.KeyMsg) (done, applied bool) {
	switch msg.String() {
	case "esc":
		return true, false
	case "enter":
		return true, true
	case "tab", "shift+tab":
		f.focus = 1 - f.focus
		return false, false
	case "up", "left":
		f.active().up()
		return false, false
	case "down", "right":
		f.active().down()
		return false, false
	default:
		return false, false
	}
}

func (f *filterForm) active() *picker {
	if f.focus == 1 {
		return &f.category
	}
	return &f.lifecycle
}

// filter is the constraint the overlay currently describes.
func (f filterForm) filter() core.ProjectFilter {
	life := lifecycleFilterOptions[f.lifecycle.selectedIndex()].value
	var catID int64
	if i := f.category.selectedIndex(); i >= 0 && i < len(f.catIDs) {
		catID = f.catIDs[i]
	}
	return core.ProjectFilter{Lifecycle: life, CategoryID: catID}
}

// render draws both pickers with the focused one marked.
func (f filterForm) render() string {
	var b strings.Builder
	b.WriteString("Filter Projects\n\n")
	b.WriteString(rowMarker(f.focus == 0) + " Lifecycle state\n")
	b.WriteString(indentLines(f.lifecycle.render(), "    "))
	b.WriteString(rowMarker(f.focus == 1) + " Category\n")
	b.WriteString(indentLines(f.category.render(), "    "))
	b.WriteString("\ntab: switch list   ↑/↓: choose   enter: apply   esc: cancel\n")
	return b.String()
}

// filterLabel is a one-line summary of a filter for the dashboard header.
func filterLabel(f core.ProjectFilter, categories []core.Category) string {
	life := "All states"
	if f.Lifecycle != "" {
		life = string(f.Lifecycle)
	}
	cat := "all Categories"
	if f.CategoryID != 0 {
		cat = "Category #" + strconv.FormatInt(f.CategoryID, 10)
		for _, c := range categories {
			if c.ID == f.CategoryID {
				cat = c.Name
				break
			}
		}
	}
	return life + " · " + cat
}
