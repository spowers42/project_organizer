package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeOverlay reports a fixed (done, ok) for named keys and false otherwise.
type fakeOverlay struct {
	result map[string][2]bool
}

func (f *fakeOverlay) update(msg tea.KeyMsg) (done, ok bool) {
	if r, hit := f.result[msg.String()]; hit {
		return r[0], r[1]
	}
	return false, false
}

func (f *fakeOverlay) render() string { return "fake overlay" }

// sentinelMsg is a distinct message value for asserting a returned command.
type sentinelMsg struct{}

func TestOverlayHostIgnoresKeysWhenEmpty(t *testing.T) {
	var h overlayHost
	if h.active() {
		t.Fatal("zero-value host is active")
	}
	cmd, handled := h.handleKey(key("x"))
	if handled || cmd != nil {
		t.Fatalf("empty host handled a key: cmd=%v handled=%v", cmd, handled)
	}
}

func TestOverlayHostConsumesKeysWhileOpen(t *testing.T) {
	var h overlayHost
	h.open(&fakeOverlay{result: map[string][2]bool{}}, nil)

	if _, handled := h.handleKey(key("j")); !handled {
		t.Fatal("open host did not consume a non-terminal key")
	}
	if !h.active() {
		t.Fatal("host cleared itself on a non-terminal key")
	}
}

func TestOverlayHostClearsOnCancelOnly(t *testing.T) {
	var h overlayHost
	ran := 0
	onOK := func() tea.Cmd { ran++; return nil }

	// submit: onOK runs, overlay stays up for the screen to close
	h.open(&fakeOverlay{result: map[string][2]bool{"enter": {true, true}}}, onOK)
	if _, handled := h.handleKey(key("enter")); !handled {
		t.Fatal("submit was not handled")
	}
	if ran != 1 {
		t.Fatalf("onOK ran %d times on submit, want 1", ran)
	}
	if !h.active() {
		t.Fatal("host cleared itself on submit; want it left for the screen")
	}
	h.close()
	if h.active() {
		t.Fatal("host still active after close()")
	}

	// cancel: host clears itself, onOK does not run
	ran = 0
	h.open(&fakeOverlay{result: map[string][2]bool{"esc": {true, false}}}, onOK)
	if _, handled := h.handleKey(key("esc")); !handled {
		t.Fatal("cancel was not handled")
	}
	if h.active() {
		t.Fatal("host did not clear itself on cancel")
	}
	if ran != 0 {
		t.Fatalf("onOK ran %d times on cancel, want 0", ran)
	}
}

func TestOverlayHostReturnsOnOKCommand(t *testing.T) {
	var h overlayHost
	h.open(
		&fakeOverlay{result: map[string][2]bool{"enter": {true, true}}},
		func() tea.Cmd { return func() tea.Msg { return sentinelMsg{} } },
	)

	cmd, handled := h.handleKey(key("enter"))
	if !handled || cmd == nil {
		t.Fatalf("submit => handled=%v cmd=%v, want true / non-nil", handled, cmd)
	}
	if _, ok := cmd().(sentinelMsg); !ok {
		t.Fatalf("host returned %T, want the onOK command's sentinelMsg", cmd())
	}
}

func TestOverlayHostSubmitWithNilOnOK(t *testing.T) {
	var h overlayHost
	h.open(&fakeOverlay{result: map[string][2]bool{"enter": {true, true}}}, nil)
	if cmd, handled := h.handleKey(key("enter")); cmd != nil || !handled {
		t.Fatalf("nil-onOK submit => cmd=%v handled=%v", cmd, handled)
	}
}

func TestListOverlayMovementIsClampedBothWays(t *testing.T) {
	o := newListOverlay([]string{"a", "b", "c"}, 0, "Pick one", "enter: ok")

	for _, k := range []string{"down", "j", "right"} { // three advances past two rows
		o.update(key(k))
	}
	if got := o.selectedIndex(); got != 2 {
		t.Fatalf("after advances, selectedIndex = %d, want 2 (clamped)", got)
	}
	for _, k := range []string{"up", "k", "left", "up"} {
		o.update(key(k))
	}
	if got := o.selectedIndex(); got != 0 {
		t.Fatalf("after retreats, selectedIndex = %d, want 0 (clamped)", got)
	}
}

func TestListOverlayReportsSubmitAndCancel(t *testing.T) {
	o := newListOverlay([]string{"a", "b"}, 0, "t", "h")

	if done, ok := o.update(key("x")); done || ok {
		t.Errorf("stray key => done=%v ok=%v, want false / false", done, ok)
	}
	if done, ok := o.update(key("enter")); !done || !ok {
		t.Errorf("enter => done=%v ok=%v, want true / true", done, ok)
	}
	if done, ok := o.update(key("esc")); !done || ok {
		t.Errorf("esc => done=%v ok=%v, want true / false", done, ok)
	}
}

func TestListOverlayRenderShowsTitleSelectionAndHint(t *testing.T) {
	o := newListOverlay([]string{"a", "b"}, 1, "Set lifecycle state", "↑/↓: choose   enter: set   esc: cancel")
	got := o.render()
	for _, want := range []string{"Set lifecycle state", "> b", "enter: set"} {
		if !strings.Contains(got, want) {
			t.Errorf("render = %q, want it to contain %q", got, want)
		}
	}
}
