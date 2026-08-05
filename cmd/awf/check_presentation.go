package main

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// checkFinding is a command-owned semantic check result. It carries no
// presentation bytes; the terminal report assigns its fixed record schema.
type checkFinding struct {
	severity string
	check    string
	detail   string
}

type checkCollection struct {
	notes    []string
	findings []checkFinding
	failures []error
}

func (c checkCollection) append(other checkCollection) checkCollection {
	notes := make(map[string]struct{}, len(c.notes))
	for _, note := range c.notes {
		notes[note] = struct{}{}
	}
	for _, note := range other.notes {
		if _, seen := notes[note]; !seen {
			c.notes = append(c.notes, note)
			notes[note] = struct{}{}
		}
	}
	findings := make(map[checkFinding]struct{}, len(c.findings))
	for _, finding := range c.findings {
		findings[finding] = struct{}{}
	}
	for _, finding := range other.findings {
		if _, seen := findings[finding]; !seen {
			c.findings = append(c.findings, finding)
			findings[finding] = struct{}{}
		}
	}
	c.failures = append(c.failures, other.failures...)
	return c
}

func checkReport(notes []string, findings []checkFinding) (presentation.Report, error) {
	errors, warnings := make([]presentation.Record, 0), make([]presentation.Record, 0)
	add := func(target *[]presentation.Record, check, detail string) error {
		checkValue, err := presentation.Prose(check)
		if err != nil {
			return err
		}
		detailValue, err := presentation.Prose(detail)
		if err != nil {
			return err
		}
		record, err := presentation.NewRecord(checkValue, detailValue)
		if err != nil { // coverage-ignore: both Prose calls returned validated values, so fixed-arity record construction cannot fail
			return err
		}
		*target = append(*target, record)
		return nil
	}
	for _, note := range notes {
		if err := add(&warnings, "advisory", note); err != nil {
			return presentation.Report{}, err
		}
	}
	for _, finding := range findings {
		if finding.severity == "warn" {
			if err := add(&warnings, finding.check, finding.detail); err != nil {
				return presentation.Report{}, err
			}
			continue
		}
		if err := add(&errors, finding.check, finding.detail); err != nil {
			return presentation.Report{}, err
		}
	}
	status := "completed"
	if len(errors) > 0 {
		status = "failed"
	} else if len(warnings) > 0 {
		status = "warnings"
	}
	value, err := presentation.Literal(fmt.Sprintf("%d errors, %d warnings", len(errors), len(warnings)))
	if err != nil { // coverage-ignore: the fixed decimal summary is grammar-valid
		return presentation.Report{}, err
	}
	summary, err := presentation.NewField("findings", value)
	if err != nil { // coverage-ignore: Literal validated the value and findings is a fixed grammar-valid label
		return presentation.Report{}, err
	}
	categories := []presentation.ReportCategory{}
	if len(errors) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "errors", Schema: []string{"check", "detail"}, Records: errors})
	}
	if len(warnings) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "warnings", Schema: []string{"check", "detail"}, Records: warnings})
	}
	return presentation.Report{Status: status, Summary: []presentation.Field{summary}, Categories: categories}, nil
}
