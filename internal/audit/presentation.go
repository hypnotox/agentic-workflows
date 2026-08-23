package audit

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// ConventionalCommitDocument maps conventional-commit findings for the commit gate.
func ConventionalCommitDocument(findings []Finding) (presentation.Document, error) {
	values := make([]presentation.Value, 0, len(findings))
	for _, finding := range findings {
		value, err := presentation.Prose(finding.Detail)
		if err != nil {
			return presentation.Document{}, err
		}
		values = append(values, value)
	}
	list, err := presentation.NewList("errors", values...)
	if err != nil { // coverage-ignore: every finding value is validated above and errors is a fixed grammar-valid label
		return presentation.Document{}, err
	}
	section, err := presentation.NewSection("check staged commit", list)
	if err != nil { // coverage-ignore: the validated List is always an admitted Section child
		return presentation.Document{}, err
	}
	return presentation.NewDocument(section)
}

// reportOutcome maps audit-owned findings into the shared representation and
// classifies command failure in one pass without changing evaluation order.
func reportOutcome(findings []Finding, commits int, base, head string) (presentation.Report, bool, error) {
	contextValue, err := presentation.Literal(fmt.Sprintf("%d commit(s) in %s..%s", commits, base, head))
	if err != nil {
		return presentation.Report{}, false, err
	}
	context, err := presentation.NewField("scope", contextValue)
	if err != nil { // coverage-ignore: the fixed grammar-valid scope label receives the validated Literal value
		return presentation.Report{}, false, err
	}
	contextFields := []presentation.Field{context}
	if commits == 0 {
		// ParseRange has already rejected line breaks, and the fixed label is
		// grammar-valid, so this closed report fact cannot fail construction.
		noticeValue, _ := presentation.Literal(fmt.Sprintf("%s..%s resolved to 0 commit(s); no history rule evaluated", base, head))
		notice, _ := presentation.NewField("notice", noticeValue)
		contextFields = append(contextFields, notice)
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
			if err != nil {
				return presentation.Report{}, false, err
			}
		}
		record, err := presentation.NewRecord(recordValues...)
		if err != nil { // coverage-ignore: every record element was validated by Prose in the preceding loop
			return presentation.Report{}, false, err
		}
		if finding.Severity == severity.Error {
			errors = append(errors, record)
		} else {
			warnings = append(warnings, record)
		}
	}
	failed := len(errors) > 0
	status := "clean"
	switch {
	case failed:
		status = "failed"
	case len(warnings) > 0:
		status = "warnings"
	case commits == 0:
		status = "empty"
	}
	countValue, err := presentation.Literal(fmt.Sprintf("%d errors, %d warnings", len(errors), len(warnings)))
	if err != nil { // coverage-ignore: the fixed decimal-only count format always satisfies Literal's grammar
		return presentation.Report{}, false, err
	}
	count, err := presentation.NewField("findings", countValue)
	if err != nil { // coverage-ignore: the fixed grammar-valid findings label receives the validated Literal value
		return presentation.Report{}, false, err
	}
	categories := []presentation.ReportCategory{}
	if len(errors) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "errors", Schema: []string{"rule", "location", "detail"}, Records: errors})
	}
	if len(warnings) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "warnings", Schema: []string{"rule", "location", "detail"}, Records: warnings})
	}
	return presentation.Report{Status: status, Context: contextFields, Summary: []presentation.Field{count}, Categories: categories}, failed, nil
}
