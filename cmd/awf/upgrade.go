package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// runUpgrade applies every registered migration past the project's current
// schema generation, then runs a normal sync to write the lock (stamping the
// current schema version) and verify the rendered output. A no-op when the
// project is already current. Truthful edge states (ADR-0076 Decision 4): no
// config layout at all → the awf init hint; a tree whose schema is ahead of
// this binary → the version-gate guidance, never "already current".
func runUpgrade(root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	state, gen, err := migrate.GateState(root)
	if err != nil {
		return err
	}
	if state == "ahead" {
		return schemaAheadError(gen)
	}
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
