package project

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// DriftCategories maps drift findings into their check-report vocabulary.
// Unused vocabulary is unranked information; every other drift kind protects
// reproducibility, correctness, or authority and remains an error.
func DriftCategories(drift []manifest.Drift, staged bool) ([]presentation.ReportCategory, error) {
	check := "drift"
	if staged {
		check = "staged drift"
	}
	var errors, information []presentation.Record
	for _, finding := range drift {
		record, err := checkRecord(check, fmt.Sprintf("%s: %s: %s", finding.Kind, finding.Path, finding.Detail))
		if err != nil { // coverage-ignore: the fixed separators make the formatted drift detail nonempty
			return nil, err
		}
		if finding.Kind == "unused-var" || finding.Kind == "unused-data" {
			information = append(information, record)
		} else {
			errors = append(errors, record)
		}
	}
	categories := errorCategory(errors)
	if len(information) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "information", Schema: []string{"check", "detail"}, Records: information})
	}
	return categories, nil
}

// CurrentStateCategories maps current-state findings into their check-report vocabulary.
func CurrentStateCategories(report CurrentStateReport, staged bool) ([]presentation.ReportCategory, error) {
	check := "current-state"
	if staged {
		check = "staged current-state"
	}
	records := make([]presentation.Record, 0, len(report.Findings()))
	for _, finding := range report.Findings() {
		record, err := checkRecord(check, finding)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return errorCategory(records), nil
}

func checkRecord(check, detail string) (presentation.Record, error) {
	checkValue, err := presentation.Prose(check)
	if err != nil { // coverage-ignore: callers select only the fixed nonempty drift or current-state labels
		return presentation.Record{}, err
	}
	detailValue, err := presentation.Prose(detail)
	if err != nil {
		return presentation.Record{}, err
	}
	record, err := presentation.NewRecord(checkValue, detailValue)
	if err != nil { // coverage-ignore: both Record values were validated by Prose immediately above
		return presentation.Record{}, err
	}
	return record, nil
}

func errorCategory(records []presentation.Record) []presentation.ReportCategory {
	if len(records) == 0 {
		return nil
	}
	return []presentation.ReportCategory{{Label: "errors", Schema: []string{"check", "detail"}, Records: records}}
}
