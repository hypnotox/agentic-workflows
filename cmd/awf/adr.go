package main

import (
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runADR dispatches the adr group. The group is Gated, so the driver has
// already run the binary-version gate by the time a handler is reached; the
// handler's own job is the subcommand grammar and the project open.
//
// The open is the ordinary one on purpose. It loads config and the catalog but
// never the ADR corpus, so a corpus carrying a duplicate identity - exactly the
// corpus `awf adr number` exists to diagnose - reaches the engine's refusal
// logic as data instead of aborting the open (ADR-0202 item 12).
func runADR(c *cmdCtx) error {
	if c.sub != "number" {
		return &usageErr{"usage: awf adr number [<slug>...]"}
	}
	p, err := project.Open(c.ctx, c.root)
	if err != nil {
		return err
	}
	// The mapping prints even when numbering returns an error. Refusals report
	// no assignments, so nothing is printed for them; past the first rename the
	// renames are on disk, and the operator needs the mapping for the
	// integration commit message whatever failed afterwards.
	report, numberErr := p.NumberPendingADRs(c.ctx, c.inv.positionals)
	_, writeErr := fmt.Fprint(c.stdout, report.String())
	return errors.Join(numberErr, writeErr)
}
