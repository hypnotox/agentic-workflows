package prosegate

import "github.com/hypnotox/agentic-workflows/internal/presentation"

// Categories maps prose scan results into their check-report vocabulary.
func Categories(findings []Finding, skipped []string) ([]presentation.ReportCategory, error) {
	warnings := make([]presentation.Record, 0, len(skipped))
	for _, path := range skipped {
		record, err := record("skipped binary: " + path)
		if err != nil { // coverage-ignore: the fixed skipped-binary diagnostic prefix makes every formatted path nonempty
			return nil, err
		}
		warnings = append(warnings, record)
	}
	errors := make([]presentation.Record, 0, len(findings))
	for _, finding := range findings {
		record, err := record(Format(finding))
		if err != nil { // coverage-ignore: Format always produces nonempty prose from the fixed diagnostic template
			return nil, err
		}
		errors = append(errors, record)
	}
	categories := []presentation.ReportCategory{}
	if len(errors) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "errors", Schema: []string{"check", "detail"}, Records: errors})
	}
	if len(warnings) > 0 {
		categories = append(categories, presentation.ReportCategory{Label: "warnings", Schema: []string{"check", "detail"}, Records: warnings})
	}
	return categories, nil
}

// DisabledCategory describes an explicitly disabled prose gate.
func DisabledCategory() ([]presentation.ReportCategory, error) {
	record, err := record("disabled (proseGate.enabled)")
	if err != nil { // coverage-ignore: the fixed nonempty disabled diagnostic always validates as prose
		return nil, err
	}
	return []presentation.ReportCategory{{Label: "warnings", Schema: []string{"check", "detail"}, Records: []presentation.Record{record}}}, nil
}

func record(detail string) (presentation.Record, error) {
	check, err := presentation.Prose("prose")
	if err != nil { // coverage-ignore: the fixed nonempty prose label is normalized by Prose before validation
		return presentation.Record{}, err
	}
	value, err := presentation.Prose(detail)
	if err != nil { // coverage-ignore: record callers supply only fixed diagnostics or nonempty formatted findings
		return presentation.Record{}, err
	}
	return presentation.NewRecord(check, value)
}
