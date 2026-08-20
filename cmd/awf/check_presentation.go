package main

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

type checkCollection struct {
	warnings    []string
	information []string
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
	// Ordinary evidence is not identity data: equal advisories and findings from
	// the repository and staged universes remain separate, source-ordered facts.
	// Plan warnings are deduplicated at their dedicated planNoteSink boundary.
	c.warnings = append(c.warnings, other.warnings...)
	c.information = append(c.information, other.information...)
	c.categories = append(c.categories, other.categories...)
	c.failures = append(c.failures, other.failures...)
	c.operational = append(c.operational, other.operational...)
	return c
}

func checkReport(warningNotes, informationNotes []string, categories []presentation.ReportCategory) (presentation.Report, error) {
	warningRecords, err := advisoryRecords(warningNotes)
	if err != nil {
		return presentation.Report{}, err
	}
	informationRecords, err := advisoryRecords(informationNotes)
	if err != nil {
		return presentation.Report{}, err
	}
	errorRecords := []presentation.Record{}
	for _, category := range categories {
		switch category.Label {
		case "errors":
			errorRecords = append(errorRecords, category.Records...)
		case "warnings":
			warningRecords = append(warningRecords, category.Records...)
		case "information":
			informationRecords = append(informationRecords, category.Records...)
		default:
			return presentation.Report{}, fmt.Errorf("unknown check report category %q", category.Label)
		}
	}
	status := "completed"
	if len(errorRecords) > 0 {
		status = "failed"
	} else if len(warningRecords) > 0 {
		status = "warnings"
	}
	value, err := presentation.Literal(fmt.Sprintf("%d errors, %d warnings, %d information", len(errorRecords), len(warningRecords), len(informationRecords)))
	if err != nil { // coverage-ignore: the fixed decimal-only summary format always satisfies Literal's grammar
		return presentation.Report{}, err
	}
	summary, err := presentation.NewField("findings", value)
	if err != nil { // coverage-ignore: the fixed grammar-valid findings label receives the validated Literal value
		return presentation.Report{}, err
	}
	output := []presentation.ReportCategory{}
	if len(errorRecords) > 0 {
		output = append(output, presentation.ReportCategory{Label: "errors", Schema: []string{"check", "detail"}, Records: errorRecords})
	}
	if len(warningRecords) > 0 {
		output = append(output, presentation.ReportCategory{Label: "warnings", Schema: []string{"check", "detail"}, Records: warningRecords})
	}
	if len(informationRecords) > 0 {
		output = append(output, presentation.ReportCategory{Label: "information", Schema: []string{"check", "detail"}, Records: informationRecords})
	}
	return presentation.Report{Status: status, Summary: []presentation.Field{summary}, Categories: output}, nil
}

func advisoryRecords(notes []string) ([]presentation.Record, error) {
	records := make([]presentation.Record, 0, len(notes))
	for _, note := range notes {
		check, err := presentation.Prose("advisory")
		if err != nil { // coverage-ignore: the fixed nonempty advisory literal is normalized by Prose before validation
			return nil, err
		}
		detail, err := presentation.Prose(note)
		if err != nil {
			return nil, err
		}
		record, err := presentation.NewRecord(check, detail)
		if err != nil { // coverage-ignore: both Record values were validated by Prose immediately above
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}
