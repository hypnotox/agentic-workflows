// Package plan parses plan files under docs/plans and scaffolds new plans from
// the rendered plans template (awf new plan). Unlike internal/adr it is not
// coupled to sequential numbering — plans are date-prefixed.
package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

// ValidStatuses are the two plan lifecycle states (ADR-0097): mutable while
// Proposed, frozen at Implemented.
var ValidStatuses = map[string]bool{"Proposed": true, "Implemented": true}

// FilenameRe matches a plan filename (YYYY-MM-DD-slug.md); it excludes
// template.md and README.md just as adr.FilenameRe's numeric form does.
var FilenameRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-.+\.md$`)

// Plan is a parsed plan record. HasFrontmatter is false for the grandfathered
// pre-convention corpus (ADR-0098), which the checks skip.
type Plan struct {
	Filename       string
	Path           string
	Date           string
	ADRs           []int
	Status         string
	HasFrontmatter bool
}

type planFrontmatter struct {
	Date   string `yaml:"date"`
	ADRs   []int  `yaml:"adrs"`
	Status string `yaml:"status"`
}

// ParseDir scans dir for plan files (YYYY-MM-DD-*.md) and parses each. Files
// without frontmatter parse to a Plan with HasFrontmatter false.
func ParseDir(dir string) ([]Plan, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", dir, err)
	}
	var plans []Plan
	for _, path := range matches {
		base := filepath.Base(path)
		if !FilenameRe.MatchString(base) {
			continue // skip template.md, README.md, and any non-plan file
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		var fm planFrontmatter
		_, found, err := frontmatter.Parse(data, &fm)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", base, err)
		}
		plans = append(plans, Plan{
			Filename: base, Path: path, Date: fm.Date, ADRs: fm.ADRs,
			Status: fm.Status, HasFrontmatter: found,
		})
	}
	return plans, nil
}
