// Package glossarycheck owns glossary validation and terseness policy.
package glossarycheck

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

const (
	PropertyCorrectness checkresult.Property = "correctness"
	PropertyHeuristic   checkresult.Property = "heuristic-quality"
)

type Input struct {
	Enabled          bool
	Authored, Merged []glossary.Record
	Domains          []string
}

func Evaluate(input Input) (checkresult.Result, error) {
	if !input.Enabled {
		return checkresult.New(nil, nil)
	}
	var findings []checkresult.Finding
	for _, record := range input.Authored {
		for _, domain := range record.Domains {
			if !slices.Contains(input.Domains, domain) {
				findings = append(findings, errorFinding(glossary.SidecarPath, "glossary-domain", fmt.Sprintf("%q: unknown domain %q", record.Term, domain)))
			}
		}
	}
	ordered := slices.Clone(input.Merged)
	slices.SortFunc(ordered, func(a, b glossary.Record) int {
		return strings.Compare(strings.ToLower(a.Term), strings.ToLower(b.Term))
	})
	for _, record := range ordered {
		if n := utf8.RuneCountInString(record.Meaning); n > glossary.MeaningMax {
			findings = append(findings, warning(fmt.Sprintf("%s: term %q meaning is %d characters, over the %d-character guideline; tighten it", glossary.SidecarPath, record.Term, n, glossary.MeaningMax)))
		}
	}
	return checkresult.New(findings, nil)
}
func errorFinding(path, kind, detail string) checkresult.Finding {
	return checkresult.Finding{Rank: severity.Error, Property: PropertyCorrectness, Evidence: checkresult.Evidence{Path: path, Kind: kind, Detail: detail}}
}
func warning(detail string) checkresult.Finding {
	return checkresult.Finding{Rank: severity.Warn, Property: PropertyHeuristic, Evidence: checkresult.Evidence{Kind: "advisory", Detail: detail}}
}
