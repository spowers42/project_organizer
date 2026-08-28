package core_test

import (
	"context"
	"testing"
)

func TestListCategoriesReturnsSeededSet(t *testing.T) {
	c, _ := newTestCore(t)

	cats, err := c.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}

	var names []string
	for _, cat := range cats {
		if cat.ID == 0 {
			t.Errorf("category %q has zero ID", cat.Name)
		}
		names = append(names, cat.Name)
	}

	want := []string{"Programming", "Course", "Other"}
	if len(names) != len(want) {
		t.Fatalf("category names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("category names = %v, want %v", names, want)
			break
		}
	}
}
