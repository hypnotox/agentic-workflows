package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// runUpgrade applies every registered migration past the project's current
// schema generation, then always runs a normal sync — even when no migration
// applies — so a same-schema binary bump still re-renders every managed file
// and re-pins the bootstrap (ADR-0085 Decision 4). Truthful edge states
// (ADR-0076 Decision 4): no config layout at all → the awf init hint; a tree
// whose schema is ahead of this binary → the version-gate guidance.
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
	applied, err := migrate.Upgrade(root, stdout)
	if err != nil {
		return err
	}
	// invariant: upgrade-always-syncs
	if len(applied) == 0 {
		fmt.Fprintf(stdout, "awf upgrade: config already at schema %d\n", gen)
	}
	for _, name := range applied {
		fmt.Fprintf(stdout, "awf upgrade: applied %s\n", name)
	}
	return runSync(root, stdout)
}
