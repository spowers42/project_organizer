// Package tui holds the Bubble Tea models for the two screens (a dashboard and
// a Project view) plus the shared overlay widgets they use for text entry,
// single-select pickers, and yes/no confirmation. It depends on core only and
// holds no domain logic: the models parse input and call core.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// screenID names the visible screen. The spec fixes the TUI at exactly two.
type screenID int

const (
	screenDashboard screenID = iota
	screenProject
)

// openProjectMsg asks the root model to switch to the Project view for the
// given Project id. The dashboard emits it when the user opens a row.
type openProjectMsg int64

// backToDashboardMsg asks the root model to return to the dashboard. The
// Project view emits it on esc / b.
type backToDashboardMsg struct{}

// model is the root Bubble Tea model. It owns both screens and routes every
// message to whichever one is visible, intercepting only the global quit key
// and the two navigation messages.
type model struct {
	core   *core.Core
	screen screenID
	dash   *dashboardModel
	proj   *projectViewModel
}

// Run launches the TUI on the given Core and blocks until the user quits.
func Run(c *core.Core) error {
	p := tea.NewProgram(newModel(c), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// newModel builds the root model on the dashboard screen.
func newModel(c *core.Core) *model {
	return &model{core: c, screen: screenDashboard, dash: newDashboard(c)}
}

// Init starts the dashboard's initial load.
func (m *model) Init() tea.Cmd {
	return m.dash.Init()
}

// Update handles global quit and navigation, then delegates to the visible
// screen.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case openProjectMsg:
		m.proj = newProjectView(m.core, int64(msg))
		m.screen = screenProject
		return m, m.proj.Init()
	case backToDashboardMsg:
		m.screen = screenDashboard
		return m, m.dash.reload()
	}

	if m.screen == screenProject {
		return m, m.proj.Update(msg)
	}
	return m, m.dash.Update(msg)
}

// View renders the visible screen.
func (m *model) View() string {
	if m.screen == screenProject {
		return m.proj.View()
	}
	return m.dash.View()
}

// statusBlock formats the status / error line shared by both screens. An empty
// status renders nothing.
func statusBlock(status string) string {
	if status == "" {
		return ""
	}
	return "\n" + status + "\n"
}
