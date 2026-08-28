package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

const dashboardTitle = "Project Organizer"

// dashboardModel is the entry screen. It lists the Projects matching the
// current filter (Active by default — the spec's "in flight"), opens a Project
// on enter, and hosts the create-Project and filter overlays. No Next step yet.
type dashboardModel struct {
	core     *core.Core
	projects []core.Project
	cats     []core.Category
	sel      int
	filter   core.ProjectFilter
	loadErr  error
	status   string
	form     *projectForm
	filterUI *filterForm
}

func newDashboard(c *core.Core) *dashboardModel {
	return &dashboardModel{core: c, filter: core.ProjectFilter{Lifecycle: core.Active}}
}

// projectsLoadedMsg carries the result of a Project list load.
type projectsLoadedMsg struct {
	projects []core.Project
	err      error
}

// categoriesLoadedMsg carries the shared Category list, loaded once per screen
// for the overlays.
type categoriesLoadedMsg struct {
	cats []core.Category
	err  error
}

// projectSavedMsg is the result of a create / edit / lifecycle mutation. A nil
// err means the change persisted.
type projectSavedMsg struct {
	err error
}

// Init loads the filtered Projects and the Category list.
func (d *dashboardModel) Init() tea.Cmd {
	return tea.Batch(d.loadProjects, d.loadCategories)
}

func (d *dashboardModel) loadProjects() tea.Msg {
	ps, err := d.core.ListProjects(context.Background(), d.filter)
	return projectsLoadedMsg{projects: ps, err: err}
}

func (d *dashboardModel) loadCategories() tea.Msg {
	cs, err := d.core.ListCategories(context.Background())
	return categoriesLoadedMsg{cats: cs, err: err}
}

// reload re-runs the Project query; the root model calls it on return from the
// Project view so edits there show up.
func (d *dashboardModel) reload() tea.Cmd {
	return d.loadProjects
}

func (d *dashboardModel) createProject(in core.ProjectInput) tea.Cmd {
	return func() tea.Msg {
		_, err := d.core.CreateProject(context.Background(), in)
		return projectSavedMsg{err: err}
	}
}

// Update advances the dashboard for one message and returns any follow-up
// command.
func (d *dashboardModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		d.projects, d.loadErr = msg.projects, msg.err
		if d.sel >= len(d.projects) {
			d.sel = 0
		}
		return nil
	case categoriesLoadedMsg:
		d.cats = msg.cats
		if msg.err != nil {
			d.status = errorMessage(msg.err)
		}
		return nil
	case projectSavedMsg:
		if msg.err != nil {
			d.status = errorMessage(msg.err)
			return nil // keep the form open so the user can fix and retry
		}
		d.form = nil
		d.status = "Project created."
		return d.reload()
	case tea.KeyMsg:
		return d.handleKey(msg)
	}
	return nil
}

func (d *dashboardModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case d.form != nil:
		done, submitted := d.form.update(msg)
		if !done {
			return nil
		}
		if !submitted {
			d.form = nil
			return nil
		}
		return d.createProject(d.form.input())
	case d.filterUI != nil:
		done, applied := d.filterUI.update(msg)
		if !done {
			return nil
		}
		if applied {
			d.filter = d.filterUI.filter()
			d.sel = 0
			d.filterUI = nil
			return d.reload()
		}
		d.filterUI = nil
		return nil
	}

	switch msg.String() {
	case "q", "esc":
		return tea.Quit
	case "up", "k":
		if d.sel > 0 {
			d.sel--
		}
	case "down", "j":
		if d.sel < len(d.projects)-1 {
			d.sel++
		}
	case "enter":
		if len(d.projects) > 0 {
			id := d.projects[d.sel].ID
			return func() tea.Msg { return openProjectMsg(id) }
		}
	case "n":
		f := newProjectForm("New Project", d.cats, nil)
		d.form = &f
		d.status = ""
	case "f":
		ff := newFilterForm(d.cats, d.filter)
		d.filterUI = &ff
	}
	return nil
}

// View renders the dashboard, its overlays, and the key hints.
func (d *dashboardModel) View() string {
	if d.form != nil {
		return d.form.render() + statusBlock(d.status)
	}
	if d.filterUI != nil {
		return d.filterUI.render() + statusBlock(d.status)
	}

	var b strings.Builder
	b.WriteString(dashboardTitle + "\n")
	b.WriteString("Filter: " + filterLabel(d.filter, d.cats) + "\n\n")
	if d.loadErr != nil {
		b.WriteString("Could not load Projects: " + d.loadErr.Error() + "\n")
	} else {
		b.WriteString(renderProjectRows(d.projects, d.sel))
	}
	b.WriteString(statusBlock(d.status))
	b.WriteString("\n↑/↓: select   enter: open   n: new Project   f: filter   q: quit\n")
	return b.String()
}

// renderProjectRows lists Projects with a caret against the selected row and
// each Project's lifecycle state. An empty list shows a filter-aware message.
func renderProjectRows(projects []core.Project, selected int) string {
	if len(projects) == 0 {
		return "No Projects match the current filter.\n"
	}
	var b strings.Builder
	for i, p := range projects {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%s  [%s]\n", marker, p.Name, p.Lifecycle)
	}
	return b.String()
}
