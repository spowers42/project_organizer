package tui

import (
	"errors"
	"testing"

	"github.com/spowers42/project_organizer/core"
)

func TestErrorMessageMapsTypedCoreErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"empty name", core.ErrEmptyProjectName, "Project name must not be empty."},
		{"unknown category", core.ErrCategoryNotFound, "That Category no longer exists."},
		{"missing project", core.ErrProjectNotFound, "That Project no longer exists."},
		{"bad lifecycle", core.ErrInvalidLifecycle, "That is not a valid lifecycle state."},
		{"wrapped typed error", errors.Join(errors.New("op failed"), core.ErrProjectNotFound), "That Project no longer exists."},
		{"unknown error falls through", errors.New("disk on fire"), "disk on fire"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errorMessage(tt.err); got != tt.want {
				t.Errorf("errorMessage(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
