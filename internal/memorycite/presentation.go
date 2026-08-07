package memorycite

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

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

// CommitGateDocument maps commit-message memory citations into the gate's
// complete ordinary presentation. Scan ownership and finding wording remain here.
func CommitGateDocument(findings []Reference) (presentation.Document, error) {
	values := make([]presentation.Value, 0, len(findings))
	for _, finding := range findings {
		value, err := presentation.Prose(fmt.Sprintf("%s line %d names the effort-owned memory file %q", finding.Path, finding.Line, finding.Segment))
		if err != nil { // coverage-ignore: fixed finding prose remains nonempty after normalization
			return presentation.Document{}, err
		}
		values = append(values, value)
	}
	list, err := presentation.NewList("errors", values...)
	if err != nil { // coverage-ignore: every reference value is validated above and errors is a fixed grammar-valid label
		return presentation.Document{}, err
	}
	section, err := presentation.NewSection("check staged commit", list)
	if err != nil { // coverage-ignore: the validated List is always an admitted Section child
		return presentation.Document{}, err
	}
	return presentation.NewDocument(section)
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
