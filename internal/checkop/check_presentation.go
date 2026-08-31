package checkop

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// reportedResult carries only the presentation facts that do not belong to an
// owner-classified check result. Result remains the sole check-result model.
type reportedResult struct {
	check          string
	result         checkresult.Result
	evidencePrefix bool
}

type checkCollection struct {
	results     []reportedResult
	failures    []error
	operational []error
}

type producedReportError struct{ error }

func (e *producedReportError) Unwrap() error { return e.error }

type producedCheckFailure struct{ err error }

func (e producedCheckFailure) Error() string { return e.err.Error() }
func (e producedCheckFailure) Unwrap() error { return e.err }

func (c *checkCollection) add(check string, result checkresult.Result, evidencePrefix bool) {
	c.results = append(c.results, reportedResult{check: check, result: result, evidencePrefix: evidencePrefix})
}

func (c checkCollection) append(other checkCollection) checkCollection {
	// Equal evidence from repository and staged universes remains separate,
	// source-ordered facts rather than identity data.
	c.results = append(c.results, other.results...)
	c.failures = append(c.failures, other.failures...)
	c.operational = append(c.operational, other.operational...)
	return c
}

// checkReport is the single repository-check report assembly path. It projects
// owner-classified results directly into the central presentation model.
func checkReport(results []reportedResult) (presentation.Report, error) {
	var errorRecords, warningRecords, informationRecords []presentation.Record
	for _, reported := range results {
		for _, finding := range reported.result.Findings() {
			detail := finding.Evidence.Detail
			if reported.evidencePrefix {
				detail = fmt.Sprintf("%s: %s: %s", finding.Evidence.Kind, finding.Evidence.Path, detail)
			}
			record, err := checkRecord(reported.check, detail)
			if err != nil {
				return presentation.Report{}, err
			}
			if finding.Rank == severity.Error {
				errorRecords = append(errorRecords, record)
			} else {
				warningRecords = append(warningRecords, record)
			}
		}
		for _, information := range reported.result.Information() {
			detail := information.Evidence.Detail
			if reported.evidencePrefix {
				detail = fmt.Sprintf("%s: %s: %s", information.Evidence.Kind, information.Evidence.Path, detail)
			}
			record, err := checkRecord(reported.check, detail)
			if err != nil {
				return presentation.Report{}, err
			}
			informationRecords = append(informationRecords, record)
		}
	}

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
	var categories []presentation.ReportCategory
	if len(errorRecords) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "errors", Schema: []string{"check", "detail"}, Records: errorRecords})
	}
	if len(warningRecords) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "warnings", Schema: []string{"check", "detail"}, Records: warningRecords})
	}
	if len(informationRecords) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "information", Schema: []string{"check", "detail"}, Records: informationRecords})
	}
	return presentation.Report{Status: status, Summary: []presentation.Field{summary}, Categories: categories}, nil
}

func checkRecord(check, detail string) (presentation.Record, error) {
	name, err := presentation.Prose(check)
	if err != nil {
		return presentation.Record{}, err
	}
	value, err := presentation.Prose(detail)
	if err != nil {
		return presentation.Record{}, err
	}
	return presentation.NewRecord(name, value)
}
