package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

// projectViewModel is the second screen: one Project's fields, with actions to
// edit them and to move the Project through its lifecycle. It reads core and
// mutates through core; it holds no domain logic.
type projectViewModel struct {
	core      *core.Core
	projectID int64
	project   core.Project
	cats      []core.Category
	loaded    bool
	loadErr   error
	status    string
	form      *projectForm
	lifeUI    *picker
}

// newProjectView builds the screen bound to one Project id; Init loads it.
func newProjectView(c *core.Core, id int64) *projectViewModel {
	return &projectViewModel{core: c, projectID: id}
}

// projectLoadedMsg carries the result of loading the viewed Project.
type projectLoadedMsg struct {
	project core.Project
	err     error
}

// Init loads the Project and the shared Category list.
func (v *projectViewModel) Init() tea.Cmd {
	return tea.Batch(v.loadProject, loadCategoriesCmd(v.core))
}

// loadProject reads the viewed Project by id.
func (v *projectViewModel) loadProject() tea.Msg {
	p, err := v.core.GetProject(context.Background(), v.projectID)
	return projectLoadedMsg{project: p, err: err}
}

// editProject persists the form's fields onto the viewed Project.
func (v *projectViewModel) editProject(in core.ProjectInput) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.EditProject(context.Background(), v.projectID, in)
		return projectSavedMsg{err: err}
	}
}

// setLifecycle moves the viewed Project to state.
func (v *projectViewModel) setLifecycle(state core.Lifecycle) tea.Cmd {
	return func() tea.Msg {
		_, err := v.core.SetProjectLifecycle(context.Background(), v.projectID, state)
		return projectSavedMsg{err: err}
	}
}

// Update advances the Project view for one message.
func (v *projectViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectLoadedMsg:
		v.project, v.loadErr, v.loaded = msg.project, msg.err, true
		return nil
	case categoriesLoadedMsg:
		v.cats = msg.cats
		if msg.err != nil {
			v.status = errorMessage(msg.err)
		}
		return nil
	case projectSavedMsg:
		if msg.err != nil {
			v.status = errorMessage(msg.err)
			return nil // keep the overlay open to fix and retry
		}
		v.form = nil
		v.lifeUI = nil
		v.status = "Saved."
		return v.loadProject
	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

// handleKey routes a key to the open overlay, or to the screen's own actions.
func (v *projectViewModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	if v.form != nil {
		done, submitted := v.form.update(msg)
		if !done {
			return nil
		}
		if !submitted {
			v.form = nil
			return nil
		}
		return v.editProject(v.form.input())
	}
	if v.lifeUI != nil {
		switch msg.String() {
		case "esc":
			v.lifeUI = nil
		case "up", "k", "left":
			v.lifeUI.up()
		case "down", "j", "right":
			v.lifeUI.down()
		case "enter":
			return v.setLifecycle(lifecycleOrder[v.lifeUI.selectedIndex()])
		}
		return nil
	}

	switch msg.String() {
	case "esc", "b":
		return func() tea.Msg { return backToDashboardMsg{} }
	case "q":
		return tea.Quit
	case "e":
		if v.ready() {
			p := v.project
			f := newProjectForm("Edit Project", v.cats, &p)
			v.form = &f
			v.status = ""
		}
	case "s":
		if v.ready() {
			p := newPicker(lifecycleLabels(), lifecycleIndex(v.project.Lifecycle))
			v.lifeUI = &p
			v.status = ""
		}
	}
	return nil
}

// ready reports whether the Project has loaded without error, so the edit and
// lifecycle actions have something to act on.
func (v *projectViewModel) ready() bool {
	return v.loaded && v.loadErr == nil
}

// View renders the Project view or whichever overlay is open.
func (v *projectViewModel) View() string {
	if v.form != nil {
		return v.form.render() + statusBlock(v.status)
	}
	if v.lifeUI != nil {
		return "Set lifecycle state\n\n" + v.lifeUI.render() +
			"\n↑/↓: choose   enter: set   esc: cancel\n" + statusBlock(v.status)
	}
	if !v.loaded {
		return "Loading Project…\n"
	}
	if v.loadErr != nil {
		return "Could not load Project: " + v.loadErr.Error() + "\n\nesc: back\n"
	}

	var b strings.Builder
	p := v.project
	fmt.Fprintf(&b, "%s\n\n", p.Name)
	fmt.Fprintf(&b, "Description: %s\n", orDash(p.Description))
	fmt.Fprintf(&b, "Category:    %s\n", v.categoryName(p.CategoryID))
	fmt.Fprintf(&b, "Lifecycle:   %s\n", p.Lifecycle)
	b.WriteString(statusBlock(v.status))
	b.WriteString("\ne: edit   s: set lifecycle   esc: back   q: quit\n")
	return b.String()
}

// categoryName is the display name for a Category id, or a neutral placeholder
// when the id is not in the loaded list.
func (v *projectViewModel) categoryName(id int64) string {
	for _, c := range v.cats {
		if c.ID == id {
			return c.Name
		}
	}
	return "(unknown Category)"
}

// orDash renders an em dash for an empty optional string.
func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
