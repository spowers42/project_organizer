package tui

import tea "github.com/charmbracelet/bubbletea"

// confirm is a reusable yes/no modal. A screen holds one while it needs an
// answer and routes key messages to update; when update reports done the
// screen acts on confirmed and drops the modal.
type confirm struct {
	prompt string
	choice bool // true == Yes
}

// newConfirm builds a modal defaulting to No.
func newConfirm(prompt string) confirm {
	return confirm{prompt: prompt}
}

// update advances the modal for one key. done is true once the user has
// answered (confirmed carries the answer) or dismissed it (confirmed false).
func (c *confirm) update(msg tea.KeyMsg) (done, confirmed bool) {
	switch msg.String() {
	case "left", "right", "tab", "h", "l":
		c.choice = !c.choice
		return false, false
	case "y", "Y":
		return true, true
	case "n", "N", "esc":
		return true, false
	case "enter":
		return true, c.choice
	default:
		return false, false
	}
}

// render draws the prompt with the current highlight and the key-hint line.
func (c confirm) render() string {
	yes, no := "  Yes  ", "  No  "
	if c.choice {
		yes = "> Yes <"
	} else {
		no = "> No <"
	}
	return c.prompt + "\n\n" + yes + "   " + no + "\n" +
		"\n←/→: choose   y/n: answer   esc: cancel\n"
}
