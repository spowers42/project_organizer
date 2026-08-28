// Command project_organizer is a local-only TUI for tracking larger personal
// projects. This entrypoint is a temporary tracer-bullet skeleton: ticket 1
// replaces it with the real application scaffold and first-run database.
package main

import (
	"fmt"
	"io"
	"os"
)

// greeting is the placeholder banner printed by the skeleton entrypoint.
func greeting() string {
	return "project_organizer: skeleton entrypoint"
}

// run writes the greeting to out and reports any write error.
func run(out io.Writer) error {
	_, err := fmt.Fprintln(out, greeting())
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
