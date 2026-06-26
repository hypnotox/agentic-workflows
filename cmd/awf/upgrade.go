package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// runUpgrade applies every registered migration past the project's current
// schema generation, then runs a normal sync to write the lock (stamping the
// current schema version) and verify the rendered output. A no-op when the
// project is already current.
func runUpgrade(root string, stdout io.Writer) error {
	applied, err := migrate.Upgrade(root)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Fprintln(stdout, "awf upgrade: already current")
		return nil
	}
	for _, name := range applied {
		fmt.Fprintf(stdout, "awf upgrade: applied %s\n", name)
	}
	return runSync(root, stdout)
}
