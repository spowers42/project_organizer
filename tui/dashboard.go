package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spowers42/project_organizer/core"
)

const dashboardTitle = "Project Organizer"

// defaultDashboardFilter is what the dashboard shows until the user narrows it:
// exactly the Active Projects — the spec's "in flight" work. The `c` key
// returns to it from any other filter.
var defaultDashboardFilter = core.ProjectFilter{Lifecycle: core.Active}

// dashboardModel is the entry screen. It lists the Projects matching the
// current filter (Active by default — the spec's "in flight"), shows each
// Active Project's Next step beneath its row, opens a Project on enter, hosts
// the create-Project and filter overlays, and offers Do Next — one weighted
// pick from those Next steps for a moment of indecision.
type dashboardModel struct {
	core         *core.Core
	projects     []core.Project
	nextSteps    map[int64]core.Task // Project id -> its Next step, absent when none
	cats         []core.Category
	sel          int
	filter       core.ProjectFilter
	prioritySort bool // Priority-first display sort of the list — a view toggle, never a stored reorder
	loadErr      error
	status       string
	overlay      overlayHost
	doNext       *doNextResult // non-nil while the Do Next view is showing
	ideasOpen    bool          // true while the Ideas panel is showing in place of the Project list
	ideas        []core.Idea
	ideaSel      int
	ideasErr     error

	categoriesOpen bool // true while the Category management panel is showing in place of the Project list
	categorySel    int
}

// newDashboard builds the screen with the default (Active-only) filter; Init
// runs the first load.
func newDashboard(c *core.Core) *dashboardModel {
	return &dashboardModel{core: c, filter: defaultDashboardFilter}
}

// loadCategoriesCmd reads the shared Category list. Both screens load it once
// for their overlays.
func loadCategoriesCmd(c *core.Core) tea.Cmd {
	return func() tea.Msg {
		cs, err := c.ListCategories(context.Background())
		return categoriesLoadedMsg{cats: cs, err: err}
	}
}

// projectsLoadedMsg carries the result of a Project list load, together with
// the Next step resolved for each listed Project (absent when the Project has
// no incomplete Task).
type projectsLoadedMsg struct {
	projects  []core.Project
	nextSteps map[int64]core.Task
	err       error
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

// priorityToggledMsg is the result of toggling a Project's Priority star from
// the dashboard. A nil err means the change persisted; the dashboard then
// reloads so the star (and any re-sort) shows.
type priorityToggledMsg struct {
	err error
}

// ideasLoadedMsg carries the result of an Idea list load, for the Ideas panel.
type ideasLoadedMsg struct {
	ideas []core.Idea
	err   error
}

// ideaSavedMsg is the result of capturing a new Idea or editing an existing
// one. A nil err means it persisted; edited distinguishes the status message.
type ideaSavedMsg struct {
	edited bool
	err    error
}

// ideaDeletedMsg is the result of plain-deleting an Idea. A nil err means it
// was archived.
type ideaDeletedMsg struct {
	err error
}

// ideaPromotedMsg is the result of promoting an Idea into a Project. A nil err
// means the Project was created and the Idea archived with a link to it.
type ideaPromotedMsg struct {
	err error
}

// categorySavedMsg is the result of creating a new Category or renaming an
// existing one. A nil err means it persisted; edited distinguishes the
// status message.
type categorySavedMsg struct {
	edited bool
	err    error
}

// categoryDeletedMsg is the result of deleting a Category. A nil err means it
// was removed; core.ErrCategoryInUse means it is still referenced by a
// Project or Idea and was left in place.
type categoryDeletedMsg struct {
	err error
}

// Init loads the filtered Projects and the Category list.
func (d *dashboardModel) Init() tea.Cmd {
	return tea.Batch(d.loadProjects, loadCategoriesCmd(d.core))
}

// loadProjects queries core for the Projects to show. On the default
// (Active-only) view it uses core.Dashboard, which pairs each Active Project
// with its resolved Next step. A narrowed filter can surface non-Active
// Projects, which have no Next step to show, so that path is a plain list.
func (d *dashboardModel) loadProjects() tea.Msg {
	ctx := context.Background()
	if d.filter == defaultDashboardFilter {
		rows, err := d.core.Dashboard(ctx)
		if err != nil {
			return projectsLoadedMsg{err: err}
		}
		projects := make([]core.Project, len(rows))
		next := make(map[int64]core.Task, len(rows))
		for i, r := range rows {
			projects[i] = r.Project
			if r.NextStep != nil {
				next[r.Project.ID] = *r.NextStep
			}
		}
		return projectsLoadedMsg{projects: projects, nextSteps: next}
	}
	ps, err := d.core.ListProjects(ctx, d.filter)
	if err != nil {
		return projectsLoadedMsg{err: err}
	}
	return projectsLoadedMsg{projects: ps}
}

// reload re-runs the Project query; the root model calls it on return from the
// Project view so edits there show up.
func (d *dashboardModel) reload() tea.Cmd {
	return d.loadProjects
}

// visibleProjects is the Project list in display order: creation order, or
// Priority-first (starred Projects first, stable) when the sort toggle is on.
// The stored order is never touched — the sort lives only here (ADR 0001).
func (d *dashboardModel) visibleProjects() []core.Project {
	if d.prioritySort {
		return core.ProjectsByPriority(d.projects)
	}
	return d.projects
}

// toggleSelectedPriority flips the Priority star on the Project under the
// cursor, then reloads so the new state (and any re-sort) shows.
func (d *dashboardModel) toggleSelectedPriority() tea.Cmd {
	vis := d.visibleProjects()
	if d.sel < 0 || d.sel >= len(vis) {
		return nil
	}
	p := vis[d.sel]
	return func() tea.Msg {
		_, err := d.core.SetProjectPriority(context.Background(), p.ID, !p.Priority)
		return priorityToggledMsg{err: err}
	}
}

// createProject persists a new Project from the form's fields.
func (d *dashboardModel) createProject(in core.ProjectInput) tea.Cmd {
	return func() tea.Msg {
		_, err := d.core.CreateProject(context.Background(), in)
		return projectSavedMsg{err: err}
	}
}

// loadIdeas queries core for the live Ideas shown in the Ideas panel.
func (d *dashboardModel) loadIdeas() tea.Msg {
	ideas, err := d.core.ListIdeas(context.Background())
	return ideasLoadedMsg{ideas: ideas, err: err}
}

// selectedIdea is the Idea under the cursor in the Ideas panel, or false when
// the list is empty.
func (d *dashboardModel) selectedIdea() (core.Idea, bool) {
	if d.ideaSel < 0 || d.ideaSel >= len(d.ideas) {
		return core.Idea{}, false
	}
	return d.ideas[d.ideaSel], true
}

// createIdea persists a newly captured Idea from the form's fields.
func (d *dashboardModel) createIdea(in core.IdeaInput) tea.Cmd {
	return func() tea.Msg {
		_, err := d.core.CreateIdea(context.Background(), in)
		return ideaSavedMsg{err: err}
	}
}

// editIdea rewrites an existing Idea's fields from the form.
func (d *dashboardModel) editIdea(id int64, in core.IdeaInput) tea.Cmd {
	return func() tea.Msg {
		_, err := d.core.EditIdea(context.Background(), id, in)
		return ideaSavedMsg{edited: true, err: err}
	}
}

// deleteIdea plain-deletes the given Idea.
func (d *dashboardModel) deleteIdea(id int64) tea.Cmd {
	return func() tea.Msg {
		return ideaDeletedMsg{err: d.core.DeleteIdea(context.Background(), id)}
	}
}

// promoteIdea promotes the given Idea into a Project.
func (d *dashboardModel) promoteIdea(id int64) tea.Cmd {
	return func() tea.Msg {
		_, err := d.core.PromoteIdea(context.Background(), id)
		return ideaPromotedMsg{err: err}
	}
}

// selectedCategory is the Category under the cursor in the Category
// management panel, or false when the list is empty.
func (d *dashboardModel) selectedCategory() (core.Category, bool) {
	if d.categorySel < 0 || d.categorySel >= len(d.cats) {
		return core.Category{}, false
	}
	return d.cats[d.categorySel], true
}

// createCategory persists a newly named Category.
func (d *dashboardModel) createCategory(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := d.core.CreateCategory(context.Background(), name)
		return categorySavedMsg{err: err}
	}
}

// renameCategory rewrites an existing Category's name.
func (d *dashboardModel) renameCategory(id int64, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := d.core.RenameCategory(context.Background(), id, name)
		return categorySavedMsg{edited: true, err: err}
	}
}

// deleteCategory removes the given Category. core.ErrCategoryInUse comes back
// while it is still referenced by a Project or Idea; the Category is left in
// place.
func (d *dashboardModel) deleteCategory(id int64) tea.Cmd {
	return func() tea.Msg {
		return categoryDeletedMsg{err: d.core.DeleteCategory(context.Background(), id)}
	}
}

// Update advances the dashboard for one message and returns any follow-up
// command.
func (d *dashboardModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		d.projects, d.nextSteps, d.loadErr = msg.projects, msg.nextSteps, msg.err
		if d.sel >= len(d.projects) {
			d.sel = 0
		}
		return nil
	case categoriesLoadedMsg:
		d.cats = msg.cats
		if d.categorySel >= len(d.cats) {
			d.categorySel = 0
		}
		if msg.err != nil {
			d.status = errorMessage(msg.err)
		}
		return nil
	case projectSavedMsg:
		if msg.err != nil {
			d.status = errorMessage(msg.err)
			return nil // keep the overlay open so the user can fix and retry
		}
		d.overlay.close()
		d.status = "Project created."
		return d.reload()
	case priorityToggledMsg:
		if msg.err != nil {
			d.status = errorMessage(msg.err)
			return nil
		}
		d.status = "Priority updated."
		return d.reload()
	case doNextLoadedMsg:
		d.doNext = &doNextResult{candidate: msg.candidate, ok: msg.ok, err: msg.err}
		return nil
	case ideasLoadedMsg:
		d.ideas, d.ideasErr = msg.ideas, msg.err
		if d.ideaSel >= len(d.ideas) {
			d.ideaSel = 0
		}
		return nil
	case ideaSavedMsg:
		if msg.err != nil {
			d.status = errorMessage(msg.err)
			return nil // keep the overlay open so the user can fix and retry
		}
		d.overlay.close()
		if msg.edited {
			d.status = "Idea updated."
		} else {
			d.status = "Idea captured."
		}
		return d.loadIdeas
	case ideaDeletedMsg:
		if msg.err != nil {
			d.status = errorMessage(msg.err)
			return nil
		}
		d.overlay.close()
		d.status = "Idea deleted."
		return d.loadIdeas
	case ideaPromotedMsg:
		if msg.err != nil {
			d.status = errorMessage(msg.err)
			return nil
		}
		d.overlay.close()
		d.status = "Idea promoted to Project."
		return tea.Batch(d.loadIdeas, d.reload())
	case categorySavedMsg:
		if msg.err != nil {
			d.status = errorMessage(msg.err)
			return nil // keep the overlay open so the user can fix and retry
		}
		d.overlay.close()
		if msg.edited {
			d.status = "Category renamed."
		} else {
			d.status = "Category added."
		}
		return loadCategoriesCmd(d.core)
	case categoryDeletedMsg:
		if msg.err != nil {
			d.status = errorMessage(msg.err)
			return nil // keep the overlay open so the user can see why and dismiss
		}
		d.overlay.close()
		d.status = "Category deleted."
		return loadCategoriesCmd(d.core)
	case tea.KeyMsg:
		return d.handleKey(msg)
	}
	return nil
}

// handleKey routes a key to the open overlay, or to the dashboard's own
// navigation and actions.
func (d *dashboardModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	if cmd, handled := d.overlay.handleKey(msg); handled {
		return cmd
	}
	if d.doNext != nil {
		return d.handleDoNextKey(msg)
	}
	if d.ideasOpen {
		return d.handleIdeasKey(msg)
	}
	if d.categoriesOpen {
		return d.handleCategoriesKey(msg)
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
		vis := d.visibleProjects()
		if len(vis) > 0 {
			id := vis[d.sel].ID
			return func() tea.Msg { return openProjectMsg(id) }
		}
	case "p":
		return d.toggleSelectedPriority()
	case "s":
		d.prioritySort = !d.prioritySort
		d.sel = 0
		if d.prioritySort {
			d.status = "Sorted Priority-first."
		} else {
			d.status = "Sorted in creation order."
		}
	case "n":
		f := newProjectForm("New Project", d.cats, nil)
		d.overlay.open(&f, func() tea.Cmd { return d.createProject(f.input()) })
		d.status = ""
	case "d":
		return doNextCmd(d.core)
	case "i":
		d.ideasOpen = true
		d.ideaSel = 0
		d.status = ""
		return d.loadIdeas
	case "C":
		d.categoriesOpen = true
		d.categorySel = 0
		d.status = ""
	case "f":
		ff := newFilterForm(d.cats, d.filter)
		d.overlay.open(&ff, func() tea.Cmd {
			d.filter = ff.filter()
			d.sel = 0
			d.overlay.close()
			return d.reload()
		})
	case "c":
		if d.filter != defaultDashboardFilter {
			d.filter = defaultDashboardFilter
			d.sel = 0
			d.status = ""
			return d.reload()
		}
	}
	return nil
}

// handleIdeasKey routes a key while the Ideas panel is showing: up/down
// selects, n captures a new Idea, e edits the selected one, x plain-deletes
// it (with confirmation), p promotes it into a Project (with confirmation),
// and esc/i return to the main dashboard.
func (d *dashboardModel) handleIdeasKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "i":
		d.ideasOpen = false
		d.status = ""
	case "up", "k":
		if d.ideaSel > 0 {
			d.ideaSel--
		}
	case "down", "j":
		if d.ideaSel < len(d.ideas)-1 {
			d.ideaSel++
		}
	case "n":
		f := newIdeaForm("New Idea", d.cats, nil)
		d.overlay.open(&f, func() tea.Cmd { return d.createIdea(f.input()) })
		d.status = ""
	case "e":
		if idea, ok := d.selectedIdea(); ok {
			f := newIdeaForm("Edit Idea", d.cats, &idea)
			d.overlay.open(&f, func() tea.Cmd { return d.editIdea(idea.ID, f.input()) })
			d.status = ""
		}
	case "x":
		if idea, ok := d.selectedIdea(); ok {
			cu := newConfirm(fmt.Sprintf("Delete idea %q? It moves to the Archive.", idea.Name))
			d.overlay.open(&cu, func() tea.Cmd { return d.deleteIdea(idea.ID) })
			d.status = ""
		}
	case "p":
		if idea, ok := d.selectedIdea(); ok {
			cu := newConfirm(fmt.Sprintf("Promote %q to a Project?", idea.Name))
			d.overlay.open(&cu, func() tea.Cmd { return d.promoteIdea(idea.ID) })
			d.status = ""
		}
	}
	return nil
}

// handleCategoriesKey routes a key while the Category management panel is
// showing: up/down selects, n adds a new Category, e renames the selected
// one, x deletes it (with confirmation, rejected with a status message while
// it is still referenced by a Project or Idea), and esc/C return to the main
// dashboard.
func (d *dashboardModel) handleCategoriesKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "C":
		d.categoriesOpen = false
		d.status = ""
	case "up", "k":
		if d.categorySel > 0 {
			d.categorySel--
		}
	case "down", "j":
		if d.categorySel < len(d.cats)-1 {
			d.categorySel++
		}
	case "n":
		f := newCategoryForm("New Category", nil)
		d.overlay.open(&f, func() tea.Cmd { return d.createCategory(f.value()) })
		d.status = ""
	case "e":
		if cat, ok := d.selectedCategory(); ok {
			f := newCategoryForm("Rename Category", &cat)
			d.overlay.open(&f, func() tea.Cmd { return d.renameCategory(cat.ID, f.value()) })
			d.status = ""
		}
	case "x":
		if cat, ok := d.selectedCategory(); ok {
			cu := newConfirm(fmt.Sprintf("Delete category %q?", cat.Name))
			d.overlay.open(&cu, func() tea.Cmd { return d.deleteCategory(cat.ID) })
			d.status = ""
		}
	}
	return nil
}

// renderCategories draws the Category management panel: the shared Category
// list with a caret against the selected row, or a graceful empty-state
// message, plus its key hints.
func (d *dashboardModel) renderCategories() string {
	var b strings.Builder
	b.WriteString("Categories\n\n")
	if len(d.cats) == 0 {
		b.WriteString("No Categories yet.\n")
	} else {
		for i, cat := range d.cats {
			marker := "  "
			if i == d.categorySel {
				marker = "> "
			}
			fmt.Fprintf(&b, "%s%s\n", marker, cat.Name)
		}
	}
	b.WriteString(statusBlock(d.status))
	b.WriteString("\n↑/↓: select   n: add Category   e: rename   x: delete   esc: back\n")
	return b.String()
}

// renderIdeas draws the Ideas panel: the list with a caret against the
// selected row, or a graceful empty-state message, plus its key hints.
func (d *dashboardModel) renderIdeas() string {
	var b strings.Builder
	b.WriteString("Ideas\n\n")
	switch {
	case d.ideasErr != nil:
		b.WriteString("Could not load Ideas: " + d.ideasErr.Error() + "\n")
	case len(d.ideas) == 0:
		b.WriteString("No Ideas captured yet.\n")
	default:
		for i, idea := range d.ideas {
			marker := "  "
			if i == d.ideaSel {
				marker = "> "
			}
			fmt.Fprintf(&b, "%s%s\n", marker, idea.Name)
		}
	}
	b.WriteString(statusBlock(d.status))
	b.WriteString("\n↑/↓: select   n: capture Idea   e: edit   p: promote to Project   x: delete   esc: back\n")
	return b.String()
}

// View renders the dashboard, its overlays, and the key hints.
func (d *dashboardModel) View() string {
	if d.overlay.active() {
		return d.overlay.render() + statusBlock(d.status)
	}
	if d.doNext != nil {
		return renderDoNext(d.doNext)
	}
	if d.ideasOpen {
		return d.renderIdeas()
	}
	if d.categoriesOpen {
		return d.renderCategories()
	}

	filtered := d.filter != defaultDashboardFilter

	var b strings.Builder
	b.WriteString(dashboardTitle + "\n")
	scope := "Showing: Active Projects (in flight)"
	if filtered {
		scope = "Showing: " + filterLabel(d.filter, d.cats)
	}
	b.WriteString(scope + "\n\n")
	if d.loadErr != nil {
		b.WriteString("Could not load Projects: " + d.loadErr.Error() + "\n")
	} else {
		b.WriteString(renderProjectRows(d.visibleProjects(), d.sel, d.nextSteps))
	}
	b.WriteString(statusBlock(d.status))
	sortHint := "s: sort Priority-first"
	if d.prioritySort {
		sortHint = "s: sort in creation order"
	}
	secondLine := "f: filter   q: quit\n"
	if filtered {
		secondLine = "f: filter   c: back to Active   q: quit\n"
	}
	b.WriteString("\n↑/↓: select   enter: open   n: new Project   p: toggle Priority   d: Do Next   i: Ideas   C: Categories   " + sortHint + "\n")
	b.WriteString(secondLine)
	return b.String()
}

// renderProjectRows lists Projects with a caret against the selected row, a
// Priority star on any starred Project, each Project's lifecycle state, and —
// indented beneath it — the Next step the Project is waiting on. A Project with
// no incomplete Task shows no Next-step line. An empty list shows a
// filter-aware message.
func renderProjectRows(projects []core.Project, selected int, nextSteps map[int64]core.Task) string {
	if len(projects) == 0 {
		return "No Projects match the current filter.\n"
	}
	var b strings.Builder
	for i, p := range projects {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%s%s  [%s]\n", marker, priorityStar(p.Priority), p.Name, p.Lifecycle)
		if step, ok := nextSteps[p.ID]; ok {
			fmt.Fprintf(&b, "      Next step: %s\n", step.Title)
		}
	}
	return b.String()
}
