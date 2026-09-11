package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// doNextResult is the outcome of one core.DoNext draw: the candidate and
// whether the pool held one, or a load error. The dashboard keeps at most one
// of these — non-nil while the Do Next view is showing, replaced wholesale on
// reroll.
type doNextResult struct {
	candidate core.DoNextCandidate
	ok        bool
	err       error
}

// doNextLoadedMsg carries a Do Next draw back to the dashboard, for both the
// initial pick and every reroll.
type doNextLoadedMsg struct {
	candidate core.DoNextCandidate
	ok        bool
	err       error
}

// doNextCmd asks core for one Do Next pick.
func doNextCmd(c *core.Core) tea.Cmd {
	return func() tea.Msg {
		cand, ok, err := c.DoNext(context.Background())
		return doNextLoadedMsg{candidate: cand, ok: ok, err: err}
	}
}

// handleDoNextKey routes a key while the Do Next view is showing: r rerolls
// (another draw from the same pool); q/esc dismiss back to the dashboard.
// Every other key is a no-op — it neither dismisses nor leaks through to the
// dashboard's own bindings while this view is up.
func (d *dashboardModel) handleDoNextKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "r":
		return doNextCmd(d.core)
	case "q", "esc":
		d.doNext = nil
	}
	return nil
}

// renderDoNext draws the Do Next view: the drawn Task and its Project, the
// load error, or a graceful message when the pool was empty — plus the
// reroll / dismiss hint.
func renderDoNext(r *doNextResult) string {
	var b strings.Builder
	b.WriteString("Do Next\n\n")
	switch {
	case r.err != nil:
		b.WriteString("Could not draw a Task: " + r.err.Error() + "\n")
	case !r.ok:
		b.WriteString("Nothing to do next — no Active Project has a Next step.\n")
	default:
		task, project := r.candidate.NextStep, r.candidate.Project
		due := ""
		if task.DueDate != nil {
			due = "  (due " + task.DueDate.Format(taskDueDateLayout) + ")"
		}
		fmt.Fprintf(&b, "%s%s%s\n", priorityStar(task.Priority), task.Title, due)
		fmt.Fprintf(&b, "  in %s%s\n", priorityStar(project.Priority), project.Name)
	}
	b.WriteString("\nr: reroll   q/esc: back to dashboard\n")
	return b.String()
}
