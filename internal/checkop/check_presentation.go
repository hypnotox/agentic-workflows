package checkop

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
)

type checkCollection struct {
	warnings     []string
	information  []string
	presentation repositorycheck.Presentation
	failures     []error
	operational  []error
}

type producedReportError struct{ error }

func (e *producedReportError) Unwrap() error { return e.error }

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
	c.presentation = c.presentation.Append(other.presentation)
	c.failures = append(c.failures, other.failures...)
	c.operational = append(c.operational, other.operational...)
	return c
}

func checkReport(warningNotes, informationNotes []string, projected repositorycheck.Presentation) (presentation.Report, error) {
	warningRecords, err := advisoryRecords(warningNotes)
	if err != nil {
		return presentation.Report{}, err
	}
	informationRecords, err := advisoryRecords(informationNotes)
	if err != nil {
		return presentation.Report{}, err
	}
	errorRecords := projected.Errors
	warningRecords = append(warningRecords, projected.Warnings...)
	informationRecords = append(informationRecords, projected.Information...)
	status := "completed"
	if len(errorRecords) > 0 {
		status = "failed"
	} else if len(warningRecords) > 0 {
		status = "warnings"
	}
	value, err := presentation.Literal(fmt.Sprintf("%d errors, %d warnings, %d information", len(errorRecords), len(warningRecords), len(informationRecords)))
	if err != nil {
		return presentation.Report{}, err
	}
	summary, err := presentation.NewField("findings", value)
	if err != nil {
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
		if err != nil {
			return nil, err
		}
		detail, err := presentation.Prose(note)
		if err != nil {
			return nil, err
		}
		record, err := presentation.NewRecord(check, detail)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}
