package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestDashboardOpensAndClosesIdeasPanel(t *testing.T) {
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("i"))
	if !d.ideasOpen {
		t.Fatal("pressing i did not open the Ideas panel")
	}
	view := d.View()
	if !strings.Contains(view, "Ideas") {
		t.Errorf("view = %q, want the Ideas panel", view)
	}

	d.Update(key("esc"))
	if d.ideasOpen {
		t.Error("esc did not close the Ideas panel")
	}
}

func TestDashboardCaptureIdeaShowsInPanel(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("i"))
	d.Update(key("n"))
	if !d.overlay.active() {
		t.Fatal("pressing n in the Ideas panel did not open the capture form")
	}
	for _, k := range typeString("Learn pottery") {
		d.Update(k)
	}
	cmd := d.Update(key("enter"))
	if cmd == nil {
		t.Fatal("submitting the Idea form produced no command")
	}
	if cmd = d.Update(cmd()); cmd == nil { // apply ideaSavedMsg
		t.Fatal("saving the Idea produced no follow-up reload")
	}
	d.Update(cmd()) // apply ideasLoadedMsg

	if d.overlay.active() {
		t.Error("Idea form stayed open after a successful capture")
	}
	view := d.View()
	if !strings.Contains(view, "Learn pottery") {
		t.Errorf("view = %q, want the captured Idea listed", view)
	}

	ideas, err := c.ListIdeas(ctx)
	if err != nil {
		t.Fatalf("ListIdeas: %v", err)
	}
	if len(ideas) != 1 || ideas[0].Name != "Learn pottery" {
		t.Errorf("ListIdeas = %+v, want one Idea named %q", ideas, "Learn pottery")
	}
}

func TestDashboardDeleteIdeaRemovesItFromPanel(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	if _, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Fleeting", CategoryID: catID}); err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("i"))
	drainInit(d.Update, d.loadIdeas)
	if len(d.ideas) != 1 {
		t.Fatalf("ideas loaded = %+v, want one", d.ideas)
	}

	d.Update(key("x"))
	if !d.overlay.active() {
		t.Fatal("pressing x did not open the delete confirmation")
	}
	cmd := d.Update(key("y"))
	if cmd == nil {
		t.Fatal("confirming delete produced no command")
	}
	d.Update(cmd())

	ideas, err := c.ListIdeas(ctx)
	if err != nil {
		t.Fatalf("ListIdeas: %v", err)
	}
	if len(ideas) != 0 {
		t.Errorf("ListIdeas after delete = %+v, want empty", ideas)
	}
}

func TestDashboardPromoteIdeaCreatesProjectAndClearsIdea(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	if _, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Build a shed", CategoryID: catID}); err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("i"))
	drainInit(d.Update, d.loadIdeas)

	d.Update(key("p"))
	if !d.overlay.active() {
		t.Fatal("pressing p did not open the promote confirmation")
	}
	cmd := d.Update(key("y"))
	if cmd == nil {
		t.Fatal("confirming promote produced no command")
	}
	d.Update(cmd())

	ideas, err := c.ListIdeas(ctx)
	if err != nil {
		t.Fatalf("ListIdeas: %v", err)
	}
	if len(ideas) != 0 {
		t.Errorf("ListIdeas after promote = %+v, want empty", ideas)
	}

	projects, err := c.ActiveProjects(ctx)
	if err != nil {
		t.Fatalf("ActiveProjects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Build a shed" {
		t.Errorf("ActiveProjects = %+v, want the promoted Project", projects)
	}
}

// Ideas never appear on the main dashboard's Active Project list.
func TestDashboardMainViewNeverShowsIdeas(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	if _, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Never here", CategoryID: catID}); err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	view := d.View()
	if strings.Contains(view, "Never here") {
		t.Errorf("view = %q, want the Idea absent from the main dashboard", view)
	}
}
