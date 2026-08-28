package tui

import (
	"strings"
	"testing"
)

func TestTextAreaInsertNewlineAndBackspace(t *testing.T) {
	ta := newTextArea("")
	for _, r := range "first" {
		ta.insertRune(r)
	}
	ta.newline()
	for _, r := range "second" {
		ta.insertRune(r)
	}
	if got := ta.String(); got != "first\nsecond" {
		t.Fatalf("String() = %q, want %q", got, "first\nsecond")
	}

	// Control runes other than newline are dropped.
	ta.insertRune('\t')
	if got := ta.String(); got != "first\nsecond" {
		t.Errorf("String() = %q, want the tab ignored", got)
	}

	// Backspace deletes the last rune, including across a line break.
	for i := 0; i < len("second"); i++ {
		ta.backspace()
	}
	ta.backspace() // removes the newline
	if got := ta.String(); got != "first" {
		t.Errorf("String() after backspacing = %q, want %q", got, "first")
	}
}

func TestTextAreaLines(t *testing.T) {
	if got := newTextArea("").lines(false); len(got) != 1 || got[0] != "(empty)" {
		t.Errorf("empty unfocused lines = %q, want [(empty)]", got)
	}
	if got := newTextArea("").lines(true); len(got) != 1 || got[0] != "_" {
		t.Errorf("empty focused lines = %q, want a caret line", got)
	}
	got := newTextArea("a\nb").lines(true)
	if strings.Join(got, "|") != "a|b_" {
		t.Errorf("lines = %q, want the caret on the last line", got)
	}
}
