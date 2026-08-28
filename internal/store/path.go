package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultDBPath returns the on-disk location of the database:
//
//	${XDG_DATA_HOME:-~/.local/share}/project_organizer/organizer.db
//
// A relative or empty XDG_DATA_HOME is ignored per the XDG Base Directory spec,
// falling back to ~/.local/share.
func DefaultDBPath() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locating home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "project_organizer", "organizer.db"), nil
}
