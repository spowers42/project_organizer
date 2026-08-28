package tui

import tea "github.com/charmbracelet/bubbletea"

// overlay is the shared contract for the modal widgets a screen hosts one of at
// a time: the Project form, the filter form, the yes/no confirm, and the
// single-select list. update advances it for one key — done is true once the
// user submits (ok true) or cancels (ok false); render draws it in full,
// including its own trailing key-hint line.
type overlay interface {
	update(tea.KeyMsg) (done, ok bool)
	render() string
}

// overlayHost owns the zero-or-one overlay a screen is currently showing. A
// screen opens one together with the command to run on submit, routes keys to it
// through handleKey, and renders it through render while active reports true.
//
// The host clears itself on cancel. On submit it runs the follow-up command but
// leaves the overlay in place, so the owning screen can keep it open (e.g. to
// show a validation error from core) and call close itself once the result
// lands. That mirrors what the screens did by hand: nil the field on a
// successful result message, keep it on an error.
type overlayHost struct {
	current overlay
	onOK    func() tea.Cmd
}

// open shows o, replacing any overlay already up. onOK runs when o reports a
// submit and may be nil.
func (h *overlayHost) open(o overlay, onOK func() tea.Cmd) {
	h.current = o
	h.onOK = onOK
}

// active reports whether an overlay is currently shown.
func (h *overlayHost) active() bool { return h.current != nil }

// close drops the current overlay.
func (h *overlayHost) close() {
	h.current = nil
	h.onOK = nil
}

// handleKey routes msg to the open overlay. handled is false when none is open,
// so the screen falls through to its own keys; an open overlay consumes every
// key. On cancel the host clears the overlay; on submit it returns the follow-up
// command (if any) and leaves the overlay up for the screen to close.
func (h *overlayHost) handleKey(msg tea.KeyMsg) (cmd tea.Cmd, handled bool) {
	if h.current == nil {
		return nil, false
	}
	done, ok := h.current.update(msg)
	if !done {
		return nil, true
	}
	if !ok {
		h.close()
		return nil, true
	}
	if h.onOK != nil {
		return h.onOK(), true
	}
	return nil, true
}

// render draws the current overlay. It must not be called unless active.
func (h *overlayHost) render() string {
	return h.current.render()
}

// listOverlay adapts a picker into an overlay: a titled single-select list with
// its own key-hint line, dismissed with esc and submitted with enter. The caller
// reads the chosen row back with selectedIndex after a submit. picker itself
// stays a pure list widget — it is also a sub-widget of the forms, where esc and
// enter mean something else.
type listOverlay struct {
	title string
	hint  string
	list  picker
}

// newListOverlay builds the overlay over labels with row start selected.
func newListOverlay(labels []string, start int, title, hint string) listOverlay {
	return listOverlay{title: title, hint: hint, list: newPicker(labels, start)}
}

// selectedIndex is the chosen row, or -1 for an empty list.
func (o listOverlay) selectedIndex() int { return o.list.selectedIndex() }

// update advances the list for one key. Movement is up/k/left and down/j/right,
// matching the lifecycle picker this replaces; esc cancels and enter submits.
func (o *listOverlay) update(msg tea.KeyMsg) (done, ok bool) {
	switch msg.String() {
	case "esc":
		return true, false
	case "enter":
		return true, true
	case "up", "k", "left":
		o.list.up()
	case "down", "j", "right":
		o.list.down()
	}
	return false, false
}

// render draws the title, the list, and the hint line.
func (o listOverlay) render() string {
	return o.title + "\n\n" + o.list.render() + "\n" + o.hint + "\n"
}
