package memorycite

import "github.com/hypnotox/agentic-workflows/internal/presentation"

// Categories maps memory-citation scan results into their check-report vocabulary.
func Categories(findings []Finding) ([]presentation.ReportCategory, error) {
	records := make([]presentation.Record, 0, len(findings))
	for _, finding := range findings {
		record, err := record(Format(finding))
		if err != nil { // coverage-ignore: Format always produces nonempty prose from the fixed diagnostic template
			return nil, err
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return []presentation.ReportCategory{{Label: "errors", Schema: []string{"check", "detail"}, Records: records}}, nil
}

// DisabledCategory describes an explicitly disabled memory-citation gate.
func DisabledCategory() ([]presentation.ReportCategory, error) {
	record, err := record("disabled (memoryCite.enabled)")
	if err != nil { // coverage-ignore: the fixed nonempty disabled diagnostic always validates as prose
		return nil, err
	}
	return []presentation.ReportCategory{{Label: "warnings", Schema: []string{"check", "detail"}, Records: []presentation.Record{record}}}, nil
}

func record(detail string) (presentation.Record, error) {
	check, err := presentation.Prose("memory")
	if err != nil { // coverage-ignore: the fixed nonempty memory label is normalized by Prose before validation
		return presentation.Record{}, err
	}
	value, err := presentation.Prose(detail)
	if err != nil { // coverage-ignore: record callers supply only fixed diagnostics or Format's nonempty template output
		return presentation.Record{}, err
	}
	return presentation.NewRecord(check, value)
}
