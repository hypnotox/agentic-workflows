package main

import (
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
// logic as data instead of aborting the open (ADR-0194 item 12).
func runADR(c *cmdCtx) error {
	if c.sub != "number" {
		return &usageErr{"usage: awf adr number [<slug>...]"}
	}
	p, err := project.Open(c.ctx, c.root)
	if err != nil {
		return err
	}
	report, err := p.NumberPendingADRs(c.ctx, c.inv.positionals)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(c.stdout, report.String())
	return err
}
