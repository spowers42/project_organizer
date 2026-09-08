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

// milestoneCompletePrompt is confirm adapted for the Milestone completion
// prompt: Confirm and Decline are both terminal answers that acknowledge the
// Milestone (core.AckMilestoneComplete) so it does not re-prompt — they differ
// only in the status message shown afterward — so every answer, esc included,
// submits rather than cancels. The chosen answer is read back with confirmed.
type milestoneCompletePrompt struct {
	c      confirm
	answer bool // valid once update has reported done
}

// newMilestoneCompletePrompt builds the prompt defaulting to No (decline).
func newMilestoneCompletePrompt(prompt string) milestoneCompletePrompt {
	return milestoneCompletePrompt{c: newConfirm(prompt)}
}

// update advances the underlying yes/no modal, then always reports submit
// once answered — esc counts as an explicit Decline, not a cancel.
func (m *milestoneCompletePrompt) update(msg tea.KeyMsg) (done, ok bool) {
	done, confirmed := m.c.update(msg)
	if !done {
		return false, false
	}
	m.answer = confirmed
	return true, true
}

// confirmed is the answer once update reports done: true for Confirm, false
// for Decline.
func (m milestoneCompletePrompt) confirmed() bool { return m.answer }

// render draws the prompt with the current highlight and the key-hint line.
func (m milestoneCompletePrompt) render() string { return m.c.render() }
