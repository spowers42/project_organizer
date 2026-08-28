package tui

// textInput is a minimal single-line editable field. Editing happens only at
// the end of the value: printable runes append and backspace removes the last
// rune. That is enough for the short name / description entry the forms need
// and keeps the field trivially testable.
type textInput struct {
	value []rune
}

// newTextInput seeds a field with existing text (empty for a fresh field).
func newTextInput(seed string) textInput {
	return textInput{value: []rune(seed)}
}

// insertRune appends a printable rune. Control runes are ignored.
func (t *textInput) insertRune(r rune) {
	if r < ' ' {
		return
	}
	t.value = append(t.value, r)
}

// backspace removes the last rune, if any.
func (t *textInput) backspace() {
	if len(t.value) > 0 {
		t.value = t.value[:len(t.value)-1]
	}
}

// String is the current text.
func (t textInput) String() string {
	return string(t.value)
}

// render draws the field, showing a caret when it holds focus.
func (t textInput) render(focused bool) string {
	s := string(t.value)
	if focused {
		return s + "_"
	}
	if s == "" {
		return "(empty)"
	}
	return s
}
