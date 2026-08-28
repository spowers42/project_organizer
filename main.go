// Command project_organizer is a local-only TUI for tracking larger personal
// projects. The binary launches the TUI by default (or on the `tui` subcommand)
// and routes the `archive` subcommand to the CLI path. Both entrypoints are
// thin: they parse input and call into the core package.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spowers42/project_organizer/cli"
	"github.com/spowers42/project_organizer/core"
	"github.com/spowers42/project_organizer/internal/store"
	"github.com/spowers42/project_organizer/tui"
)

// mode is the entrypoint selected from the command line.
type mode int

const (
	modeTUI mode = iota
	modeArchive
	modeUnknown
)

// route decides which entrypoint handles args (everything after the program
// name) and returns the arguments left for that entrypoint.
func route(args []string) (mode, []string) {
	if len(args) == 0 {
		return modeTUI, nil
	}
	switch args[0] {
	case "tui":
		return modeTUI, args[1:]
	case "archive":
		return modeArchive, args[1:]
	default:
		return modeUnknown, args
	}
}

// run wires the real dependencies and hands off to the selected entrypoint.
// Every failure is returned; main is the single place that reports it.
func run(args []string, stdout, stderr io.Writer) error {
	m, rest := route(args)
	if m == modeUnknown {
		return fmt.Errorf("unknown command %q", rest[0])
	}

	dbPath, err := store.DefaultDBPath()
	if err != nil {
		return err
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	c := core.New(st, core.SystemClock{}, core.NewRand(time.Now().UnixNano()))

	switch m {
	case modeArchive:
		return cli.RunArchive(c, rest, stdout, stderr)
	default:
		return tui.Run(c)
	}
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "project_organizer: %v\n", err)
		os.Exit(1)
	}
}
