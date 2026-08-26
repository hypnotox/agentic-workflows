package main

import (
	"errors"

	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// runADR dispatches the adr group. The group is Gated, so the driver has
// already run the binary-version gate by the time a handler is reached; the
// handler's own job is the subcommand grammar and the project open.
//
// The open is the ordinary one on purpose. It loads config and the catalog but
// never the ADR corpus, so a corpus carrying a duplicate identity - exactly the
// corpus `awf adr number` exists to diagnose - reaches the engine's refusal
// logic as data instead of aborting the open (ADR-0202 item 12).
func runADR(c *cmdCtx) (returnErr error) {
	if c.sub != "number" {
		return &usageErr{"usage: awf adr number [<slug>...]"}
	}
	// Numbering changes authored tracked inputs and then renders both roots.
	// Take the complete lease before opening config or the ADR corpus, and retain
	// it through mapping presentation so the returned outcome describes one
	// authority snapshot.
	lease, err := filesystem.AcquireProjectLease(c.ctx, c.root, awfgit.ProjectResidentRoot(c.ctx, c.root))
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
	state, cfg, _, err := openProjectOperation(c.ctx, c.root)
	if err != nil {
		return err
	}
	// The mapping prints even when numbering returns an error. Refusals report
	// no assignments, so nothing is printed for them; past the first rename the
	// renames are on disk, and the operator needs the mapping for the
	// integration commit message whatever failed afterwards.
	report, numberErr := currentstatecoord.NumberPendingADRsLeased(state.Root(), c.inv.positionals, func() (currentstatecoord.PublicationOutcome, error) {
		return composePublisher(state, cfg).SyncLeased(c.ctx, lease)
	}, lease)
	if len(report.Assignments) == 0 {
		return numberErr
	}
	document, presentationErr := report.Document()
	var partial *currentstatecoord.PartialNumberingError
	if errors.As(numberErr, &partial) {
		document, presentationErr = partial.Document()
	}
	if presentationErr != nil { // coverage-ignore: numbering emits validated slugs and four-digit numeric assignments
		return errors.Join(numberErr, presentationErr)
	}
	return errors.Join(numberErr, presentation.Render(c.stdout, document))
}
