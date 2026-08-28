package tui

import (
	"strings"
	"testing"
)

func TestConfirmAnswerShortcuts(t *testing.T) {
	tests := []struct {
		name          string
		keys          []string
		wantDone      bool
		wantConfirmed bool
	}{
		{"y confirms", []string{"y"}, true, true},
		{"n declines", []string{"n"}, true, false},
		{"esc dismisses", []string{"esc"}, true, false},
		{"enter takes the default (No)", []string{"enter"}, true, false},
		{"toggle then enter confirms", []string{"right", "enter"}, true, true},
		{"a stray key does nothing", []string{"x"}, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newConfirm("Proceed?")
			var done, confirmed bool
			for _, k := range tt.keys {
				done, confirmed = c.update(key(k))
			}
			if done != tt.wantDone || confirmed != tt.wantConfirmed {
				t.Errorf("keys %v => done=%v confirmed=%v, want %v/%v",
					tt.keys, done, confirmed, tt.wantDone, tt.wantConfirmed)
			}
		})
	}
}

func TestConfirmRenderHighlightsCurrentChoice(t *testing.T) {
	c := newConfirm("Sure?")
	if !strings.Contains(c.render(), "> No <") {
		t.Errorf("render = %q, want No highlighted by default", c.render())
	}
	c.update(key("tab"))
	if !strings.Contains(c.render(), "> Yes <") {
		t.Errorf("render = %q, want Yes highlighted after toggle", c.render())
	}
}
