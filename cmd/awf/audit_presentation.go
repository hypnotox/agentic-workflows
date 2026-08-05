package main

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// auditReport maps audit-owned findings into the shared representation without
// changing their rank tokens or evaluation order.
func auditReport(findings []audit.Finding, commits int, base, head string) (presentation.Report, error) {
	contextValue, err := presentation.Literal(fmt.Sprintf("%d commit(s) in %s..%s", commits, base, head))
	if err != nil { // coverage-ignore: fixed scope text is grammar-valid
		return presentation.Report{}, err
	}
	context, err := presentation.NewField("scope", contextValue)
	if err != nil { // coverage-ignore: fixed scope label is grammar-valid
		return presentation.Report{}, err
	}
	errors, warnings := []presentation.Record{}, []presentation.Record{}
	for _, finding := range findings {
		location := finding.Commit
		if location == "" {
			location = "branch"
		}
		values := []string{finding.Rule, location, finding.Detail}
		recordValues := make([]presentation.Value, len(values))
		for i, value := range values {
			recordValues[i], err = presentation.Prose(value)
			if err != nil { // coverage-ignore: audit findings normalize to one-line prose
				return presentation.Report{}, err
			}
		}
		record, err := presentation.NewRecord(recordValues...)
		if err != nil { // coverage-ignore: three validated values always form a record
			return presentation.Report{}, err
		}
		if finding.Severity == severity.Error {
			errors = append(errors, record)
		} else {
			warnings = append(warnings, record)
		}
	}
	status := "clean"
	switch {
	case commits == 0:
		status = "empty"
	case len(errors) > 0:
		status = "failed"
	case len(warnings) > 0:
		status = "warnings"
	}
	countValue, err := presentation.Literal(fmt.Sprintf("%d errors, %d warnings", len(errors), len(warnings)))
	if err != nil { // coverage-ignore: fixed count text is grammar-valid
		return presentation.Report{}, err
	}
	count, err := presentation.NewField("findings", countValue)
	if err != nil { // coverage-ignore: fixed summary label is grammar-valid
		return presentation.Report{}, err
	}
	categories := []presentation.ReportCategory{}
	if len(errors) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "errors", Schema: []string{"rule", "location", "detail"}, Records: errors})
	}
	if len(warnings) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "warnings", Schema: []string{"rule", "location", "detail"}, Records: warnings})
	}
	return presentation.Report{Status: status, Context: []presentation.Field{context}, Summary: []presentation.Field{count}, Categories: categories}, nil
}
