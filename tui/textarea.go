package tui

import "strings"

// textEntry is the editing surface shared by textInput and textArea: append a
// rune, delete the last one. taskForm routes keystrokes to whichever field
// holds focus through this interface so it does not care which kind it is.
type textEntry interface {
	insertRune(r rune)
	backspace()
}

// textArea is a minimal multi-line editable field. Like textInput, editing
// happens only at the end of the value: printable runes and newlines append,
// backspace removes the last rune. That is enough for the freeform Task notes
// the form needs and keeps the widget trivially testable.
type textArea struct {
	value []rune
}

// newTextArea seeds a field with existing text (empty for a fresh field).
func newTextArea(seed string) textArea {
	return textArea{value: []rune(seed)}
}

// insertRune appends a printable rune. Control runes other than a newline are
// ignored.
func (t *textArea) insertRune(r rune) {
	if r != '\n' && r < ' ' {
		return
	}
	t.value = append(t.value, r)
}

// newline appends a line break.
func (t *textArea) newline() {
	t.value = append(t.value, '\n')
}

// backspace removes the last rune, if any.
func (t *textArea) backspace() {
	if len(t.value) > 0 {
		t.value = t.value[:len(t.value)-1]
	}
}

// String is the current text, newlines and all.
func (t textArea) String() string {
	return string(t.value)
}

// lines renders the field as one or more display lines, appending a caret to
// the last line when focused. An empty unfocused field is a single placeholder
// line so the row never collapses.
func (t textArea) lines(focused bool) []string {
	s := string(t.value)
	if s == "" && !focused {
		return []string{"(empty)"}
	}
	if focused {
		s += "_"
	}
	return strings.Split(s, "\n")
}
