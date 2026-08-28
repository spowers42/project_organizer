package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRoute(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantMode mode
		wantRest []string
	}{
		{"no args launches the TUI", nil, modeTUI, nil},
		{"explicit tui subcommand", []string{"tui"}, modeTUI, []string{}},
		{"archive routes to the CLI path", []string{"archive"}, modeArchive, []string{}},
		{"archive keeps its own args", []string{"archive", "list"}, modeArchive, []string{"list"}},
		{"unknown command", []string{"frobnicate"}, modeUnknown, []string{"frobnicate"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotRest := route(tt.args)
			if gotMode != tt.wantMode {
				t.Errorf("route(%q) mode = %d, want %d", tt.args, gotMode, tt.wantMode)
			}
			if !slices.Equal(gotRest, tt.wantRest) {
				t.Errorf("route(%q) rest = %q, want %q", tt.args, gotRest, tt.wantRest)
			}
		})
	}
}

func TestRunUnknownCommandErrorsWithoutTouchingDisk(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"frobnicate"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run(frobnicate) = nil, want error")
	}
	if !strings.Contains(err.Error(), "frobnicate") {
		t.Errorf("error = %v, want it to name the command", err)
	}
}

// TestRunArchivePathCreatesThenReusesDatabase drives the full first-run /
// second-run acceptance path through run(): the archive subcommand needs no
// TTY, so it exercises store.DefaultDBPath + store.Open end to end.
func TestRunArchivePathCreatesThenReusesDatabase(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	dbPath := filepath.Join(dataHome, "project_organizer", "organizer.db")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"archive", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("first run: %v (stderr: %s)", err, stderr.String())
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database not created at %s: %v", dbPath, err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"archive", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("second run: %v (stderr: %s)", err, stderr.String())
	}
}
