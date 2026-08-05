package main

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

type checkCollection struct {
	notes       []string
	categories  []presentation.ReportCategory
	failures    []error
	operational []error
}

type producedCheckFailure struct{ err error }

func (e producedCheckFailure) Error() string {
	return e.err.Error()
}
func (e producedCheckFailure) Unwrap() error { return e.err }

func (c checkCollection) append(other checkCollection) checkCollection {
	// Ordinary evidence is not identity data: equal notes and findings from the
	// repository and staged universes remain separate, source-ordered facts.
	// Plan notes are deduplicated at their dedicated planNoteSink boundary.
	c.notes = append(c.notes, other.notes...)
	c.categories = append(c.categories, other.categories...)
	c.failures = append(c.failures, other.failures...)
	c.operational = append(c.operational, other.operational...)
	return c
}

func checkReport(notes []string, categories []presentation.ReportCategory) (presentation.Report, error) {
	warnings := make([]presentation.Record, 0, len(notes))
	for _, note := range notes {
		check, err := presentation.Prose("advisory")
		if err != nil { // coverage-ignore: the fixed nonempty advisory literal is normalized by Prose before validation
			return presentation.Report{}, err
		}
		detail, err := presentation.Prose(note)
		if err != nil {
			return presentation.Report{}, err
		}
		record, err := presentation.NewRecord(check, detail)
		if err != nil { // coverage-ignore: both Record values were validated by Prose immediately above
			return presentation.Report{}, err
		}
		warnings = append(warnings, record)
	}
	errors := []presentation.Record{}
	for _, category := range categories {
		switch category.Label {
		case "errors":
			errors = append(errors, category.Records...)
		case "warnings":
			warnings = append(warnings, category.Records...)
		}
	}
	status := "completed"
	if len(errors) > 0 {
		status = "failed"
	} else if len(warnings) > 0 {
		status = "warnings"
	}
	value, err := presentation.Literal(fmt.Sprintf("%d errors, %d warnings", len(errors), len(warnings)))
	if err != nil { // coverage-ignore: the fixed decimal-only summary format always satisfies Literal's grammar
		return presentation.Report{}, err
	}
	summary, err := presentation.NewField("findings", value)
	if err != nil { // coverage-ignore: the fixed grammar-valid findings label receives the validated Literal value
		return presentation.Report{}, err
	}
	output := []presentation.ReportCategory{}
	if len(errors) > 0 {
		output = append(output, presentation.ReportCategory{Label: "errors", Schema: []string{"check", "detail"}, Records: errors})
	}
	if len(warnings) > 0 {
		output = append(output, presentation.ReportCategory{Label: "warnings", Schema: []string{"check", "detail"}, Records: warnings})
	}
	return presentation.Report{Status: status, Summary: []presentation.Field{summary}, Categories: output}, nil
}
