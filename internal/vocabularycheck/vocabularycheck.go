// Package vocabularycheck owns glossary and pitfall-tag vocabulary policy.
package vocabularycheck

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

const (
	// PropertyCorrectness is the protected property for vocabulary validity.
	PropertyCorrectness checkresult.Property = "correctness"
	// PropertyHeuristic is the protected property for vocabulary warnings.
	PropertyHeuristic     checkresult.Property = "heuristic-quality"
	tagFrequencyThreshold                      = .25
)

// Input is a defensive prepared semantic projection.
type Input struct {
	GlossaryEnabled  bool
	Authored, Merged []glossary.Record
	Domains          []string
	Tags             map[string]string
	Pitfalls         pitfall.Corpus
}

// Results keeps glossary and tag results distinct so aggregation can preserve
// their established error and advisory placements without inspecting kinds.
type Results struct {
	Glossary checkresult.Result
	Tags     checkresult.Result
}

// Evaluate evaluates each vocabulary concern once over prepared values.
func Evaluate(input Input) (Results, error) {
	glossaryResult, err := checkGlossary(input)
	if err != nil { // coverage-ignore: prepared glossary checks construct fixed valid evidence
		return Results{}, err
	}
	tagResult, err := checkTags(input)
	if err != nil { // coverage-ignore: prepared tag checks construct fixed valid evidence
		return Results{}, err
	}
	return Results{Glossary: glossaryResult, Tags: tagResult}, nil
}

// Check returns the glossary-then-tag compatibility projection.
func checkGlossary(input Input) (checkresult.Result, error) {
	if !input.GlossaryEnabled {
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

func checkTags(input Input) (checkresult.Result, error) {
	if len(input.Tags) == 0 {
		return checkresult.New(nil, nil)
	}
	var findings []checkresult.Finding
	for _, tag := range slices.Sorted(maps.Keys(input.Tags)) {
		if strings.TrimSpace(input.Tags[tag]) == "" {
			findings = append(findings, errorFinding(".awf/config.yaml", "tag-vocabulary", fmt.Sprintf("tag %q has an empty meaning", tag)))
		}
		if slices.Contains(input.Domains, tag) {
			findings = append(findings, errorFinding(".awf/config.yaml", "tag-domain-collision", fmt.Sprintf("tag %q equals a configured domain name: tags must be finer than domains", tag)))
		}
	}
	tagged := 0
	frequency := map[string]int{}
	for _, entry := range input.Pitfalls.All() {
		for _, tag := range entry.Tags {
			if _, ok := input.Tags[tag]; !ok {
				findings = append(findings, errorFinding(entry.SourcePath, "pitfall-tag", fmt.Sprintf("%s (%q): unknown tag %q", entry.Slug, entry.Title, tag)))
			}
		}
		var vocabularyTags []string
		for _, tag := range entry.Tags {
			if _, ok := input.Tags[tag]; ok {
				vocabularyTags = append(vocabularyTags, tag)
			}
		}
		if len(entry.Tags) == 0 {
			findings = append(findings, warning(entry.SourcePath+" carries no tags: add a narrow topic tag"))
			continue
		}
		if len(vocabularyTags) == 0 {
			continue
		}
		tagged++
		for _, tag := range vocabularyTags {
			frequency[tag]++
		}
	}
	if tagged > 0 {
		for _, tag := range slices.Sorted(maps.Keys(frequency)) {
			if float64(frequency[tag]) > tagFrequencyThreshold*float64(tagged) {
				findings = append(findings, warning(fmt.Sprintf("tag %q is on %d/%d tagged pitfalls (>%.0f%%): coarsening toward domain scale", tag, frequency[tag], tagged, tagFrequencyThreshold*100)))
			}
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
