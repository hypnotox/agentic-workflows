package main

import (
	"context"
	"io"
	"strconv"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// runUninstall removes awf's generated footprint from a project (delegated to
// resident.Uninstall: lock-tracked files, the dirs they leave empty, and the
// lock). It deliberately leaves the authored .awf/ config (config.yaml,
// sidecars, convention parts) in place.
func runUninstall(ctx context.Context, root string, stdout io.Writer) error {
	report, err := resident.Uninstall(ctx, root)
	if err != nil {
		return err
	}
	removed, err := presentation.Literal(strconv.Itoa(report.Removed))
	if err != nil { // coverage-ignore: decimal formatting always produces a nonempty literal without line breaks
		return err
	}
	removedField, err := presentation.NewField("generated files removed", removed)
	if err != nil { // coverage-ignore: Literal validated the value and the label is fixed and grammar-valid
		return err
	}
	note, err := presentation.Prose("the authored .awf config remains in place; delete it to fully remove")
	if err != nil { // coverage-ignore: fixed nonempty prose contains no forbidden line break
		return err
	}
	notes := []presentation.Value{note}
	if len(report.PreservedRoots) > 0 {
		for _, root := range report.PreservedRoots {
			value, valueErr := presentation.Prose("preserved resident data under .awf/" + root)
			if valueErr != nil { // coverage-ignore: the fixed prefix keeps normalized resident-root prose nonempty
				return valueErr
			}
			notes = append(notes, value)
		}
	}
	document, err := (presentation.Mutation{Status: "uninstall completed", Identity: []presentation.Field{removedField}, Notes: notes}).Document()
	if err != nil { // coverage-ignore: every value is validated above and Mutation uses fixed grammar-valid labels
		return err
	}
	return presentation.Render(stdout, document)
}
