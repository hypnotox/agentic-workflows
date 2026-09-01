package publisher

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
)

const glossarySidecarPath = glossary.SidecarPath

const glossaryMeaningMax = glossary.MeaningMax

// docDataTransform computes document data before rendering.
func docDataTransform(name string, sc config.Sidecar) (config.Sidecar, error) {
	if name == "glossary" {
		return glossaryTransform(sc)
	}
	return sc, nil
}

func glossaryTransform(sc config.Sidecar) (config.Sidecar, error) {
	_, authored := sc.Data["terms"]
	_, standard := sc.Data["standardTerms"]
	if !authored && !standard {
		return sc, nil
	}
	records, err := glossary.Merge(sc)
	if err != nil {
		return sc, err
	}
	out := sc
	out.Data = maps.Clone(sc.Data)
	out.Data["terms"] = glossaryRows(records)
	delete(out.Data, "standardTerms")
	return out, nil
}

func glossaryRows(records []glossary.Record) string {
	if len(records) == 0 {
		return ""
	}
	sorted := slices.Clone(records)
	slices.SortFunc(sorted, func(a, b glossary.Record) int {
		return strings.Compare(strings.ToLower(a.Term), strings.ToLower(b.Term))
	})
	var b strings.Builder
	for _, r := range sorted {
		fmt.Fprintf(&b, "| %s | %s |\n", escapePipes(r.Term), escapePipes(r.Meaning))
	}
	return b.String()
}
func escapePipes(s string) string { return strings.ReplaceAll(s, "|", `\|`) }
