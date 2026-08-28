package tui

import "testing"

func TestTextInputAppendsAndDeletesAtTheEnd(t *testing.T) {
	ti := newTextInput("")
	for _, r := range "hi!" {
		ti.insertRune(r)
	}
	if got := ti.String(); got != "hi!" {
		t.Fatalf("String = %q, want %q", got, "hi!")
	}
	ti.backspace()
	if got := ti.String(); got != "hi" {
		t.Errorf("after backspace, String = %q, want %q", got, "hi")
	}
}

func TestTextInputBackspaceOnEmptyIsSafe(t *testing.T) {
	ti := newTextInput("")
	ti.backspace()
	if got := ti.String(); got != "" {
		t.Errorf("String = %q, want empty", got)
	}
}

func TestTextInputIgnoresControlRunes(t *testing.T) {
	ti := newTextInput("ok")
	ti.insertRune('\t')
	ti.insertRune('\n')
	if got := ti.String(); got != "ok" {
		t.Errorf("String = %q, want control runes ignored", got)
	}
}

func TestTextInputSeedsFromExistingText(t *testing.T) {
	ti := newTextInput("existing")
	if got := ti.String(); got != "existing" {
		t.Errorf("String = %q, want the seed value", got)
	}
}

func TestTextInputRenderShowsFocusCaret(t *testing.T) {
	ti := newTextInput("abc")
	if got := ti.render(true); got != "abc_" {
		t.Errorf("focused render = %q, want a caret", got)
	}
	if got := ti.render(false); got != "abc" {
		t.Errorf("unfocused render = %q, want the plain value", got)
	}
	empty := newTextInput("")
	if got := empty.render(false); got != "(empty)" {
		t.Errorf("unfocused empty render = %q, want a placeholder", got)
	}
}
