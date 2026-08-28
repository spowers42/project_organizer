// Package tui holds the Bubble Tea models for the two screens (a dashboard and
// a Project view). It depends on core only and holds no domain logic.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// Run launches the TUI on the given Core and blocks until the user quits.
func Run(c *core.Core) error {
	p := tea.NewProgram(newDashboard(c), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// dashboard is the entry screen: it lists every Active Project with its Next
// step and offers Do Next. There are no Projects yet, so it renders the empty
// state. It holds the Core it will query once Projects exist.
type dashboard struct {
	core *core.Core
}

func newDashboard(c *core.Core) dashboard {
	return dashboard{core: c}
}

// Init has nothing to load yet; Projects arrive in a later ticket.
func (d dashboard) Init() tea.Cmd { return nil }

// Update handles quit. There is no other interaction yet.
func (d dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c", "esc":
			return d, tea.Quit
		}
	}
	return d, nil
}

// View renders the dashboard. There are no Projects yet, so it always renders
// the empty state; it must not panic.
func (d dashboard) View() string {
	return renderDashboard()
}

func renderDashboard() string {
	const title = "Project Organizer"
	return title + "\n\nNo Active Projects yet.\n\nPress q to quit.\n"
}
