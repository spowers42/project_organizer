// Package tui holds the Bubble Tea models for the two screens (a dashboard and
// a Project view). It depends on core only and holds no domain logic.
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// Run launches the TUI on the given Core and blocks until the user quits.
func Run(c *core.Core) error {
	p := tea.NewProgram(newDashboard(c), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// dashboard is the entry screen: it lists every Active Project ("in flight"
// work) and will offer Do Next. Next step resolution is a later ticket. It
// holds the Core it queries for the Active Project list.
type dashboard struct {
	core     *core.Core
	projects []core.Project
	loadErr  error
}

func newDashboard(c *core.Core) dashboard {
	return dashboard{core: c}
}

// projectsLoadedMsg carries the result of the initial Active Projects load.
type projectsLoadedMsg struct {
	projects []core.Project
	err      error
}

// Init kicks off the Active Projects load.
func (d dashboard) Init() tea.Cmd {
	return d.loadProjects
}

func (d dashboard) loadProjects() tea.Msg {
	projects, err := d.core.ActiveProjects(context.Background())
	return projectsLoadedMsg{projects: projects, err: err}
}

// Update stores the loaded Projects and handles quit. There is no other
// interaction yet.
func (d dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		d.projects = msg.projects
		d.loadErr = msg.err
		return d, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return d, tea.Quit
		}
	}
	return d, nil
}

// View renders the dashboard. It must not panic with no Projects loaded.
func (d dashboard) View() string {
	if d.loadErr != nil {
		return renderDashboardError(d.loadErr)
	}
	return renderDashboard(d.projects)
}

const dashboardTitle = "Project Organizer"

func renderDashboard(projects []core.Project) string {
	var b strings.Builder
	b.WriteString(dashboardTitle)
	b.WriteString("\n\n")
	if len(projects) == 0 {
		b.WriteString("No Active Projects yet.\n")
	} else {
		b.WriteString("Active Projects:\n")
		for _, p := range projects {
			fmt.Fprintf(&b, "  • %s\n", p.Name)
		}
	}
	b.WriteString("\nPress q to quit.\n")
	return b.String()
}

func renderDashboardError(err error) string {
	return fmt.Sprintf("%s\n\nCould not load Projects: %v\n\nPress q to quit.\n", dashboardTitle, err)
}
