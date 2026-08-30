package memorycite

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// CommitGateDocument maps commit-message memory citations into the gate's
// complete ordinary presentation. Scan ownership and finding wording remain here.
func CommitGateDocument(findings []Reference) (presentation.Document, error) {
	values := make([]presentation.Value, 0, len(findings))
	for _, finding := range findings {
		value, err := presentation.Prose(fmt.Sprintf("%s line %d names the effort-owned memory file %q", finding.Path, finding.Line, finding.Segment))
		if err != nil {
			return presentation.Document{}, err
		}
		values = append(values, value)
	}
	list, err := presentation.NewList("errors", values...)
	if err != nil {
		return presentation.Document{}, err
	}
	section, err := presentation.NewSection("check staged commit", list)
	if err != nil {
		return presentation.Document{}, err
	}
	return presentation.NewDocument(section)
}
