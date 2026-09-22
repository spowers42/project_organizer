package core

import (
	"context"
	"errors"
	"strings"
)

// Errors returned by the Category operations. Callers match them with
// errors.Is.
var (
	ErrEmptyCategoryName = errors.New("category name must not be empty")
	ErrCategoryNotFound  = errors.New("category not found")
	ErrCategoryInUse     = errors.New("category is referenced by a project or idea")
)

// CreateCategory adds a new Category to the shared list used to classify both
// Projects and Ideas. The name is trimmed and must be non-empty.
func (c *Core) CreateCategory(ctx context.Context, name string) (Category, error) {
	name, err := validateCategoryName(name)
	if err != nil {
		return Category{}, err
	}
	return c.store.CreateCategory(ctx, name)
}

// RenameCategory rewrites a Category's name. Every Project and Idea that
// references it keeps that reference untouched. Same validation as
// CreateCategory; ErrCategoryNotFound if id does not name a Category.
func (c *Core) RenameCategory(ctx context.Context, id int64, name string) (Category, error) {
	name, err := validateCategoryName(name)
	if err != nil {
		return Category{}, err
	}
	return c.store.RenameCategory(ctx, id, name)
}

// DeleteCategory removes a Category. ErrCategoryNotFound if id does not name a
// Category; ErrCategoryInUse if any Project or Idea — including archived
// ones — still references it via category_id.
func (c *Core) DeleteCategory(ctx context.Context, id int64) error {
	if err := c.requireCategory(ctx, id); err != nil {
		return err
	}
	referenced, err := c.store.CategoryReferenced(ctx, id)
	if err != nil {
		return err
	}
	if referenced {
		return ErrCategoryInUse
	}
	return c.store.DeleteCategory(ctx, id)
}

// validateCategoryName trims and checks a user-supplied Category name.
func validateCategoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrEmptyCategoryName
	}
	return name, nil
}
