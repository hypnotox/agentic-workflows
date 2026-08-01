package topic

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

const (
	originPrefix    = "Origin: "
	revisedByPrefix = "Revised-by: "
	partFileName    = "current-state.md"
)

// SubstituteProvenance rewrites the authored claim parts under root so every
// `ADR-<slug>` provenance entry whose slug is a key of renames takes that
// record's assigned `ADR-NNNN` form, and every touched `Revised-by:` list is
// rewritten to the duplicate-free ascending order ADR-0191 requires.
//
// This is the substitution half of numbering (ADR-0202 item 9), and it lives
// here because the `Origin:`/`Revised-by:` line grammar is this package's. Two
// scoping rules keep the effect exhaustive: it is anchored on those two
// metadata lines, so a slug named in claim prose is never rewritten, and it
// walks only .awf/topics/parts, so no generated topic doc, no domain part, no
// ADR body, and no plan is reachable from here. Generated outputs follow from
// the caller's re-render.
func SubstituteProvenance(root string, renames map[string]string) error {
	if len(renames) == 0 {
		return nil
	}
	partsRoot := filepath.Join(root, config.DirName, "topics", "parts")
	err := collectFiles(partsRoot, func(path string) error {
		if filepath.Base(path) != partFileName {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil { // coverage-ignore: the walk just discovered this file; failure requires a concurrent filesystem race
			return err
		}
		body, changed := substituteProvenanceLines(string(data), renames)
		if !changed {
			return nil
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil { // coverage-ignore: the file was just read from the same path; a write fails only on a permission fault a test cannot trigger
			return err
		}
		return nil
	})
	if err != nil { // coverage-ignore: collectFiles tolerates a missing parts root and the callback's own error paths are unreachable
		return err
	}
	return nil
}

// substituteProvenanceLines applies the substitution to one part's bytes,
// reporting whether anything moved. A line whose value does not parse as
// provenance is left alone: the corpus loader owns that diagnosis, and
// numbering must not turn a malformed part into a differently malformed one.
func substituteProvenanceLines(body string, renames map[string]string) (string, bool) {
	lines := strings.SplitAfter(body, "\n")
	changed := false
	for i, raw := range lines {
		trimmed := strings.TrimSuffix(raw, "\n")
		eol := raw[len(trimmed):]
		switch {
		case strings.HasPrefix(trimmed, originPrefix):
			ref, err := parseADRRef(strings.TrimSpace(strings.TrimPrefix(trimmed, originPrefix)))
			if err != nil {
				continue
			}
			number, renamed := renames[ref]
			if !renamed {
				continue
			}
			lines[i], changed = originPrefix+"ADR-"+number+eol, true
		case strings.HasPrefix(trimmed, revisedByPrefix):
			refs, err := parseADRList(strings.TrimSpace(strings.TrimPrefix(trimmed, revisedByPrefix)))
			if err != nil {
				continue
			}
			touched := false
			for j, ref := range refs {
				if number, renamed := renames[ref]; renamed {
					refs[j], touched = number, true
				}
			}
			if !touched {
				continue
			}
			lines[i], changed = revisedByPrefix+strings.Join(canonicalADRRefs(refs), ", ")+eol, true
		}
	}
	return strings.Join(lines, ""), changed
}

// canonicalADRRefs renders a provenance list in the duplicate-free ascending
// order ADR-0191 requires: a substituted entry that lands numerically below an
// entry already in the list is re-sorted into place, not appended after it.
func canonicalADRRefs(refs []string) []string {
	unique := make([]string, 0, len(refs))
	for _, ref := range refs {
		if !slices.Contains(unique, ref) {
			unique = append(unique, ref)
		}
	}
	slices.SortStableFunc(unique, func(a, b string) int {
		return adr.IdentityOrder(a) - adr.IdentityOrder(b)
	})
	for i, ref := range unique {
		unique[i] = "ADR-" + ref
	}
	return unique
}
