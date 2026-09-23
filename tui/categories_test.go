package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestDashboardOpensAndClosesCategoriesPanel(t *testing.T) {
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("C"))
	if !d.categoriesOpen {
		t.Fatal("pressing C did not open the Categories panel")
	}
	view := d.View()
	if !strings.Contains(view, "Categories") {
		t.Errorf("view = %q, want the Categories panel", view)
	}

	d.Update(key("esc"))
	if d.categoriesOpen {
		t.Error("esc did not close the Categories panel")
	}
}

func TestDashboardAddCategoryShowsInPanelAndPickers(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("C"))
	d.Update(key("n"))
	if !d.overlay.active() {
		t.Fatal("pressing n in the Categories panel did not open the add form")
	}
	for _, k := range typeString("Woodworking") {
		d.Update(k)
	}
	cmd := d.Update(key("enter"))
	if cmd == nil {
		t.Fatal("submitting the Category form produced no command")
	}
	if cmd = d.Update(cmd()); cmd == nil { // apply categorySavedMsg
		t.Fatal("saving the Category produced no follow-up reload")
	}
	d.Update(cmd()) // apply categoriesLoadedMsg

	if d.overlay.active() {
		t.Error("Category form stayed open after a successful add")
	}
	view := d.View()
	if !strings.Contains(view, "Woodworking") {
		t.Errorf("view = %q, want the added Category listed", view)
	}

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	found := false
	for _, cat := range cats {
		if cat.Name == "Woodworking" {
			found = true
		}
	}
	if !found {
		t.Errorf("ListCategories = %+v, want a Category named %q", cats, "Woodworking")
	}

	// The dashboard's own Category list (used by the New Project / Filter
	// pickers) refreshes to include it too.
	foundInDash := false
	for _, cat := range d.cats {
		if cat.Name == "Woodworking" {
			foundInDash = true
		}
	}
	if !foundInDash {
		t.Errorf("dashboard cats = %+v, want the new Category so pickers show it", d.cats)
	}
}

func TestDashboardRenameCategoryRewritesName(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("C"))

	d.Update(key("e"))
	if !d.overlay.active() {
		t.Fatal("pressing e in the Categories panel did not open the rename form")
	}
	original, ok := d.selectedCategory()
	if !ok {
		t.Fatal("no Category selected to rename")
	}
	for i := 0; i < len(original.Name); i++ {
		d.Update(key("backspace"))
	}
	for _, k := range typeString("Renamed") {
		d.Update(k)
	}
	cmd := d.Update(key("enter"))
	if cmd == nil {
		t.Fatal("submitting the rename form produced no command")
	}
	if cmd = d.Update(cmd()); cmd == nil { // apply categorySavedMsg
		t.Fatal("saving the renamed Category produced no follow-up reload")
	}
	d.Update(cmd()) // apply categoriesLoadedMsg

	if d.overlay.active() {
		t.Error("Category form stayed open after a successful rename")
	}

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	found := false
	for _, cat := range cats {
		if cat.ID == original.ID && cat.Name == "Renamed" {
			found = true
		}
	}
	if !found {
		t.Errorf("ListCategories = %+v, want Category %d renamed to %q", cats, original.ID, "Renamed")
	}
}

func TestDashboardDeleteUnreferencedCategoryRemovesIt(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	if _, err := c.CreateCategory(ctx, "Spare"); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("C"))
	// Select the Category we just created (last in the list).
	for d.categorySel < len(d.cats)-1 {
		d.Update(key("down"))
	}
	target, ok := d.selectedCategory()
	if !ok || target.Name != "Spare" {
		t.Fatalf("selected Category = %+v, want the unreferenced %q", target, "Spare")
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

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	for _, cat := range cats {
		if cat.Name == "Spare" {
			t.Errorf("ListCategories = %+v, want %q removed", cats, "Spare")
		}
	}
	if d.overlay.active() {
		t.Error("delete confirmation stayed open after a successful delete")
	}
}

func TestDashboardDeleteReferencedCategoryShowsInUseMessageAndKeepsIt(t *testing.T) {
	ctx := context.Background()
	c := newTestCore(t)
	catID := firstCategoryID(t, c)
	if _, err := c.CreateIdea(ctx, core.IdeaInput{Name: "Anchors a Category", CategoryID: catID}); err != nil {
		t.Fatalf("CreateIdea: %v", err)
	}

	d := newDashboard(c)
	drainInit(d.Update, d.Init())

	d.Update(key("C"))
	d.categorySel = 0
	target, ok := d.selectedCategory()
	if !ok || target.ID != catID {
		t.Fatalf("selected Category = %+v, want the referenced one (id %d)", target, catID)
	}

	d.Update(key("x"))
	cmd := d.Update(key("y"))
	if cmd == nil {
		t.Fatal("confirming delete produced no command")
	}
	d.Update(cmd())

	if !strings.Contains(d.status, "still used") {
		t.Errorf("status = %q, want a clear in-use rejection message", d.status)
	}

	cats, err := c.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	found := false
	for _, cat := range cats {
		if cat.ID == catID {
			found = true
		}
	}
	if !found {
		t.Errorf("ListCategories = %+v, want the referenced Category left in place", cats)
	}
}
