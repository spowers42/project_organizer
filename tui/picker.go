package tui

import "strings"

// picker is a reusable single-select vertical list. It carries only display
// labels; the caller keeps any parallel slice of ids or domain values and
// reads the chosen index back with selectedIndex.
type picker struct {
	items    []string
	selected int
}

// newPicker builds a picker over labels with the given initial selection. An
// out-of-range start is clamped into the list (or left at zero for an empty
// list).
func newPicker(items []string, start int) picker {
	p := picker{items: items, selected: start}
	p.clamp()
	return p
}

// up moves the selection one row earlier, stopping at the top.
func (p *picker) up() {
	p.selected--
	p.clamp()
}

// down moves the selection one row later, stopping at the bottom.
func (p *picker) down() {
	p.selected++
	p.clamp()
}

// clamp pulls a stray selection back into range (or to zero for an empty list).
func (p *picker) clamp() {
	if p.selected < 0 || len(p.items) == 0 {
		p.selected = 0
		return
	}
	if p.selected >= len(p.items) {
		p.selected = len(p.items) - 1
	}
}

// selectedIndex is the index of the current row, or -1 when the list is empty.
func (p picker) selectedIndex() int {
	if len(p.items) == 0 {
		return -1
	}
	return p.selected
}

// value is the label of the current row, or "" when the list is empty.
func (p picker) value() string {
	if len(p.items) == 0 {
		return ""
	}
	return p.items[p.selected]
}

// render draws the list with a caret against the selected row.
func (p picker) render() string {
	if len(p.items) == 0 {
		return "  (none)\n"
	}
	var b strings.Builder
	for i, item := range p.items {
		marker := "  "
		if i == p.selected {
			marker = "> "
		}
		b.WriteString(marker)
		b.WriteString(item)
		b.WriteByte('\n')
	}
	return b.String()
}
