package project

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// glossarySidecarPath names the authoring surface in every glossary content error.
const glossarySidecarPath = config.DirName + "/docs/glossary.yaml"

// glossaryRecord is one authored term: the term itself, its meaning, and the
// optional owning domains. Domains resolve against the project's configured
// domains in checkGlossary, which this render-time path cannot see.
type glossaryRecord struct {
	Term    string
	Meaning string
	Domains []string
}

// glossaryRecordKeys is the closed set of keys a record may carry. An unknown
// key is a typo that would otherwise render as silent absence.
var glossaryRecordKeys = map[string]bool{"term": true, "meaning": true, "domains": true}

// glossaryMeaningMax bounds a meaning at roughly two sentences of ordinary
// prose, counted in runes (ADR-0207 decision 9). It is a fixed constant rather
// than a config key on purpose: an adopter-raisable threshold is a suppressing
// value, which this project's severity model does not have. The advisory
// evaluates the merged set, so this bounds the shipped standard vocabulary as
// well as authored terms (decision 10); the portability test is the additional
// guard that awf never ships an over-length term in the first place.
const glossaryMeaningMax = 280

// mergedGlossaryRecords is the single home of the two-layer merge: the standard
// vocabulary awf ships, overlaid by the project's authored terms. An authored
// record overrides a shipped record whose term matches case-insensitively,
// which is the only way to remove an unwanted shipped term. A case-insensitive
// duplicate WITHIN either layer stays a hard error; a duplicate ACROSS layers is
// the override. Order is not guaranteed; glossaryRows sorts.
func mergedGlossaryRecords(sc config.Sidecar) ([]glossaryRecord, error) {
	shipped, err := glossaryRecords(sc.Data["standardTerms"])
	if err != nil {
		// The shipped layer is awf's own closed list, so a violation here is a
		// defect in this binary rather than anything the adopter authored.
		return nil, fmt.Errorf("standard vocabulary is malformed: %w", err)
	}
	authored, err := glossaryRecords(sc.Data["terms"])
	if err != nil {
		return nil, err
	}
	overridden := make(map[string]bool, len(authored))
	for _, r := range authored {
		overridden[strings.ToLower(r.Term)] = true
	}
	out := make([]glossaryRecord, 0, len(shipped)+len(authored))
	for _, r := range shipped {
		if !overridden[strings.ToLower(r.Term)] {
			out = append(out, r)
		}
	}
	return append(out, authored...), nil
}

// glossaryRecords validates one authored layer into its record list. An absent
// or null value yields nil, nil (the template's else branch renders the
// placeholder). A non-list value, a non-mapping element, a missing or
// wrong-typed field, an unknown record key, or a case-insensitive duplicate
// term within this layer is a hard error. Domain resolution is checkGlossary's
// job (it needs the project's domains); this validates shape only.
func glossaryRecords(raw any) ([]glossaryRecord, error) {
	if raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, glossaryErr("must be a list of {term, meaning} records")
	}
	out := make([]glossaryRecord, 0, len(list))
	seen := map[string]string{} // lower(term) -> first term carrying it
	for i, el := range list {
		m, err := glossaryStringMap(i, el)
		if err != nil {
			return nil, err
		}
		r, err := glossaryRecordFrom(i, m)
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[strings.ToLower(r.Term)]; dup {
			return nil, glossaryErr(fmt.Sprintf("terms %q and %q are case-insensitive duplicates", prev, r.Term))
		}
		seen[strings.ToLower(r.Term)] = r.Term
		out = append(out, r)
	}
	return out, nil
}

// glossaryStringMap normalizes the two shapes yaml.v3 hands a mapping element:
// map[string]any when every key is a string, map[any]any once any key is not.
func glossaryStringMap(i int, el any) (map[string]any, error) {
	switch m := el.(type) {
	case map[string]any:
		return m, nil
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			ks, isStr := k.(string)
			if !isStr {
				return nil, glossaryErr(fmt.Sprintf("record %d: key %v is not a string", i, k))
			}
			out[ks] = v
		}
		return out, nil
	default:
		return nil, glossaryErr(fmt.Sprintf("record %d must be a mapping", i))
	}
}

// glossaryRecordFrom validates one mapping into a glossaryRecord. The term is
// read first so every later violation can name it; only a malformed term falls
// back to the record index.
func glossaryRecordFrom(i int, m map[string]any) (glossaryRecord, error) {
	term, err := glossaryTerm(i, m)
	if err != nil {
		return glossaryRecord{}, err
	}
	meaning, err := glossaryMeaning(term, m)
	if err != nil {
		return glossaryRecord{}, err
	}
	domains, err := glossaryDomains(term, m)
	if err != nil {
		return glossaryRecord{}, err
	}
	// Sorted so a record carrying several unknown keys reports the same one every run.
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if !glossaryRecordKeys[k] {
			return glossaryRecord{}, glossaryErr(fmt.Sprintf("term %q: unknown key %q", term, k))
		}
	}
	return glossaryRecord{Term: term, Meaning: meaning, Domains: domains}, nil
}

// glossaryTerm reads the required term. It is what names every other violation,
// so its own violations name the record index instead.
func glossaryTerm(i int, m map[string]any) (string, error) {
	v, ok := m["term"]
	if !ok {
		return "", glossaryErr(fmt.Sprintf("record %d: missing %q", i, "term"))
	}
	s, isStr := v.(string)
	if !isStr {
		return "", glossaryErr(fmt.Sprintf("record %d: %q must be a non-empty string", i, "term"))
	}
	term := strings.TrimSpace(s)
	if term == "" {
		return "", glossaryErr(fmt.Sprintf("record %d: term is empty", i))
	}
	if strings.Contains(term, "\n") {
		return "", glossaryErr(fmt.Sprintf("record %d: term %q contains a newline; table rows are single-line", i, term))
	}
	return term, nil
}

// glossaryMeaning reads the required meaning, naming the term it belongs to.
func glossaryMeaning(term string, m map[string]any) (string, error) {
	v, ok := m["meaning"]
	if !ok {
		return "", glossaryErr(fmt.Sprintf("term %q: missing %q", term, "meaning"))
	}
	s, isStr := v.(string)
	if !isStr {
		return "", glossaryErr(fmt.Sprintf("term %q: meaning must be a non-empty string", term))
	}
	meaning := strings.TrimSpace(s)
	if meaning == "" {
		return "", glossaryErr(fmt.Sprintf("term %q: meaning is empty", term))
	}
	if strings.Contains(meaning, "\n") {
		return "", glossaryErr(fmt.Sprintf("term %q: meaning contains a newline; table rows are single-line", term))
	}
	return meaning, nil
}

// glossaryDomains reads the optional domains list (nil when absent or null).
func glossaryDomains(term string, m map[string]any) ([]string, error) {
	v, ok := m["domains"]
	if !ok || v == nil {
		return nil, nil
	}
	list, isList := v.([]any)
	if !isList {
		return nil, glossaryErr(fmt.Sprintf("term %q: %q must be a list", term, "domains"))
	}
	out := make([]string, 0, len(list))
	for _, el := range list {
		s, isStr := el.(string)
		if !isStr || strings.TrimSpace(s) == "" {
			return nil, glossaryErr(fmt.Sprintf("term %q: %q entries must be non-empty strings", term, "domains"))
		}
		out = append(out, strings.TrimSpace(s))
	}
	return out, nil
}

// glossaryErr prefixes every content violation with the authoring surface.
func glossaryErr(msg string) error {
	return fmt.Errorf("%s data.terms: %s", glossarySidecarPath, msg)
}
