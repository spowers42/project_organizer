// Package cli wires the `archive` subcommand (list / restore / purge). It
// depends on core only and holds no domain logic.
package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spowers42/project_organizer/core"
)

const archiveUsage = `usage: project_organizer archive <command>

commands:
  list             show everything in the Archive
  restore <id>     restore an archived entity
  purge <id>       permanently delete an archived entity

ids are printed by "archive list" in kind:number form, e.g. project:12
`

// archiveKindNames maps the lowercase id prefix used on the command line to
// its core.ArchiveKind, and back.
var archiveKindNames = map[core.ArchiveKind]string{
	core.ArchivedProject:   "project",
	core.ArchivedMilestone: "milestone",
	core.ArchivedTask:      "task",
	core.ArchivedIdea:      "idea",
}

// formatArchiveRef renders ref in the kind:number form archive list prints,
// which restore and purge parse back with parseArchiveRef.
func formatArchiveRef(ref core.ArchiveRef) string {
	name, ok := archiveKindNames[ref.Kind]
	if !ok {
		name = string(ref.Kind)
	}
	return fmt.Sprintf("%s:%d", name, ref.ID)
}

// parseArchiveRef parses the kind:number id form restore and purge take on
// the command line, as printed by archive list.
func parseArchiveRef(id string) (core.ArchiveRef, error) {
	kindName, numPart, ok := strings.Cut(id, ":")
	if !ok {
		return core.ArchiveRef{}, fmt.Errorf("archive: invalid id %q, want kind:number (e.g. project:12)", id)
	}
	var kind core.ArchiveKind
	found := false
	for k, name := range archiveKindNames {
		if name == kindName {
			kind = k
			found = true
			break
		}
	}
	if !found {
		return core.ArchiveRef{}, fmt.Errorf("archive: unknown kind %q in id %q", kindName, id)
	}
	num, err := strconv.ParseInt(numPart, 10, 64)
	if err != nil {
		return core.ArchiveRef{}, fmt.Errorf("archive: invalid id %q, want kind:number (e.g. project:12)", id)
	}
	return core.ArchiveRef{Kind: kind, ID: num}, nil
}

// RunArchive dispatches an `archive` invocation. args is everything after the
// `archive` word.
func RunArchive(c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, archiveUsage)
		return fmt.Errorf("archive: no command given")
	}

	ctx := context.Background()
	switch args[0] {
	case "list":
		return runArchiveList(ctx, c, stdout)
	case "restore":
		return runArchiveRestore(ctx, c, args[1:], stdout, stderr)
	case "purge":
		return runArchivePurge(ctx, c, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprint(stderr, archiveUsage)
		return fmt.Errorf("archive: unknown command %q", args[0])
	}
}

// runArchiveList prints every entity currently in the Archive, one per line,
// as "<id>\t<label>".
func runArchiveList(ctx context.Context, c *core.Core, stdout io.Writer) error {
	entries, err := c.ListArchived(ctx)
	if err != nil {
		return fmt.Errorf("archive list: %w", err)
	}
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(stdout, "the Archive is empty")
		return nil
	}
	for _, e := range entries {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", formatArchiveRef(e.Ref), e.Label)
	}
	return nil
}

// runArchiveRestore parses args as a single id and restores that entity.
func runArchiveRestore(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	ref, err := requireOneID(args, stderr, "restore")
	if err != nil {
		return err
	}
	if err := c.RestoreArchived(ctx, ref); err != nil {
		return fmt.Errorf("archive restore %s: %w", formatArchiveRef(ref), err)
	}
	_, _ = fmt.Fprintf(stdout, "restored %s\n", formatArchiveRef(ref))
	return nil
}

// runArchivePurge parses args as a single id and permanently deletes that
// entity.
func runArchivePurge(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	ref, err := requireOneID(args, stderr, "purge")
	if err != nil {
		return err
	}
	if err := c.PurgeArchived(ctx, ref); err != nil {
		return fmt.Errorf("archive purge %s: %w", formatArchiveRef(ref), err)
	}
	_, _ = fmt.Fprintf(stdout, "purged %s\n", formatArchiveRef(ref))
	return nil
}

// requireOneID validates that args holds exactly one id and parses it,
// printing usage to stderr on any failure.
func requireOneID(args []string, stderr io.Writer, command string) (core.ArchiveRef, error) {
	if len(args) != 1 {
		_, _ = fmt.Fprint(stderr, archiveUsage)
		return core.ArchiveRef{}, fmt.Errorf("archive %s: want exactly one id", command)
	}
	ref, err := parseArchiveRef(args[0])
	if err != nil {
		_, _ = fmt.Fprint(stderr, archiveUsage)
		return core.ArchiveRef{}, err
	}
	return ref, nil
}
