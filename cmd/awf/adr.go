package main

import (
	"errors"

	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
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
func runADR(c *cmdCtx) error {
	if c.sub != "number" {
		return &usageErr{"usage: awf adr number [<slug>...]"}
	}
	state, cfg, _, err := openProjectOperation(c.ctx, c.root)
	if err != nil {
		return err
	}
	// The mapping prints even when numbering returns an error. Refusals report
	// no assignments, so nothing is printed for them; past the first rename the
	// renames are on disk, and the operator needs the mapping for the
	// integration commit message whatever failed afterwards.
	report, numberErr := currentstatecoord.NumberPendingADRs(state.Root(), c.inv.positionals, func() error { _, err := composePublisher(state, cfg).Sync(); return err })
	if len(report.Assignments) == 0 {
		return numberErr
	}
	document, presentationErr := numberingPresentation(report)
	if presentationErr != nil { // coverage-ignore: numbering emits validated slugs and four-digit numeric assignments
		return errors.Join(numberErr, presentationErr)
	}
	return errors.Join(numberErr, presentation.Render(c.stdout, document))
}

// numberingPresentation maps the coordinator-owned assignments to command output.
func numberingPresentation(report currentstatecoord.NumberingReport) (presentation.Document, error) {
	records := make([]presentation.Record, 0, len(report.Assignments))
	for _, assignment := range report.Assignments {
		slug, err := presentation.Literal(assignment.Slug)
		if err != nil {
			return presentation.Document{}, err
		}
		number, err := presentation.Literal(assignment.Number)
		if err != nil {
			return presentation.Document{}, err
		}
		record, err := presentation.NewRecord(slug, number)
		if err != nil { // coverage-ignore: two validated nonempty literals always form a valid record
			return presentation.Document{}, err
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return presentation.Document{}, nil
	}
	return (presentation.Collection{Status: "ADR numbering completed", Categories: []presentation.CollectionCategory{{Label: "assignments", Schema: []string{"slug", "number"}, Records: records}}}).Document()
}
