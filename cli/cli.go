// Package cli wires the `archive` subcommand (list / restore / purge). It
// depends on core only and holds no domain logic. Ticket 1 establishes the
// routing; the individual operations land in a later ticket.
package cli

import (
	"fmt"
	"io"

	"github.com/spowers42/project_organizer/core"
)

const archiveUsage = `usage: project_organizer archive <command>

commands:
  list             show everything in the Archive
  restore <id>     restore an archived entity
  purge <id>       permanently delete an archived entity
`

// RunArchive dispatches an `archive` invocation. args is everything after the
// `archive` word.
func RunArchive(_ *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, archiveUsage)
		return fmt.Errorf("archive: no command given")
	}

	switch args[0] {
	case "list", "restore", "purge":
		_, _ = fmt.Fprintf(stdout, "archive %s: not implemented yet\n", args[0])
		return nil
	default:
		_, _ = fmt.Fprint(stderr, archiveUsage)
		return fmt.Errorf("archive: unknown command %q", args[0])
	}
}
