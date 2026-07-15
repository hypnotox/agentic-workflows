package project

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// glossarySidecarPath names the authoring surface in every glossary content error.
const glossarySidecarPath = config.DirName + "/docs/glossary.yaml"

// docDataTransform is the docs renderKindSpec transform (ADR-0089): the seam
// where a doc's sidecar data is computed into rendered content upstream of both
// renderTarget and artifactConfigHash, so a change to the computation itself
// reflags the doc exactly like a config edit (the ADR-0045 both-consumers
// pattern). The glossary and pitfalls docs compute today.
func docDataTransform(name string, sc config.Sidecar) (config.Sidecar, error) {
	switch name {
	case "glossary":
		return glossaryTransform(sc)
	case "pitfalls":
		return pitfallsTransform(sc)
	default:
		return sc, nil
	}
}

// glossaryTransform replaces data.terms - the authored term→meaning map - with
// the finished, always-sorted markdown table rows (ADR-0089). An absent key is
// left untouched and a null or empty map yields "", so the template's else
// branch renders the coherent placeholder either way. Content violations are
// hard errors naming the sidecar and the offending key.
func glossaryTransform(sc config.Sidecar) (config.Sidecar, error) {
	raw, ok := sc.Data["terms"]
	if !ok {
		return sc, nil
	}
	entries, err := glossaryEntries(raw)
	if err != nil {
		return sc, err
	}
	out := sc
	out.Data = maps.Clone(sc.Data)
	out.Data["terms"] = glossaryRows(entries)
	return out, nil
}

// glossaryEntries validates the authored value into a term→meaning map.
// invariant: glossary-terms-validated
func glossaryEntries(raw any) (map[string]string, error) {
	m, err := glossaryStringMap(raw)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]string, len(m))
	seen := map[string]string{} // lower(term) → first term carrying it
	for k, v := range m {
		term := strings.TrimSpace(k)
		if term == "" {
			return nil, glossaryErr(fmt.Sprintf("term %q is empty", k))
		}
		if strings.Contains(term, "\n") {
			return nil, glossaryErr(fmt.Sprintf("term %q contains a newline: table rows are single-line", term))
		}
		s, isStr := v.(string)
		if !isStr {
			return nil, glossaryErr(fmt.Sprintf("term %q: meaning must be a non-empty string", term))
		}
		meaning := strings.TrimSpace(s)
		if meaning == "" {
			return nil, glossaryErr(fmt.Sprintf("term %q: meaning is empty", term))
		}
		if strings.Contains(meaning, "\n") {
			return nil, glossaryErr(fmt.Sprintf("term %q: meaning contains a newline; table rows are single-line", term))
		}
		if prev, dup := seen[strings.ToLower(term)]; dup {
			return nil, glossaryErr(fmt.Sprintf("terms %q and %q are case-insensitive duplicates", prev, term))
		}
		seen[strings.ToLower(term)] = term
		entries[term] = meaning
	}
	return entries, nil
}

// glossaryStringMap normalizes the two shapes yaml.v3 hands an `any` mapping:
// map[string]any when every key is a string, map[any]any once any key is not.
func glossaryStringMap(raw any) (map[string]any, error) {
	switch m := raw.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return m, nil
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			ks, isStr := k.(string)
			if !isStr {
				return nil, glossaryErr(fmt.Sprintf("map key %v is not a string", k))
			}
			out[ks] = v
		}
		return out, nil
	default:
		return nil, glossaryErr("must be a mapping of term: meaning")
	}
}

// glossaryRows renders the sorted table rows. Ordering is case-insensitive by
// term; ties are impossible because case-insensitive duplicates are rejected
// upstream, so equal entry sets always render byte-identically.
// invariant: glossary-terms-sorted
func glossaryRows(entries map[string]string) string {
	if len(entries) == 0 {
		return ""
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "| %s | %s |\n", escapePipes(k), escapePipes(entries[k]))
	}
	return b.String()
}

// escapePipes keeps a term or meaning inside one GFM table cell.
func escapePipes(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// glossaryErr prefixes every content violation with the authoring surface.
func glossaryErr(msg string) error {
	return fmt.Errorf("%s data.terms: %s", glossarySidecarPath, msg)
}
