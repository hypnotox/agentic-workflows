package prosegate

import "github.com/hypnotox/agentic-workflows/internal/presentation"

// Categories maps prose findings into their check-report vocabulary. Binary
// files are skipped by Scan but intentionally have no user-facing diagnostic.
func Categories(findings []Finding, _ []string) ([]presentation.ReportCategory, error) {
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
	return categories, nil
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
