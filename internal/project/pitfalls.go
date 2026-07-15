package project

import (
	"fmt"
	"maps"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// pitfallsSidecarPath names the authoring surface in every pitfalls content error.
const pitfallsSidecarPath = config.DirName + "/docs/pitfalls.yaml"

// pitfallEntry is one authored pitfall: a heading title, the optional owning
// domains that drive awf-context surfacing, optional related ADR numbers, the
// optional governed tags, and the markdown body. Shared by the render transform,
// checkPitfalls, and ContextFor (ADR-0099).
type pitfallEntry struct {
	Title   string
	Domains []string
	Related []int
	Tags    []string
	Body    string
}

// pitfallEntries validates data.pitfalls into the ordered entry list. An absent or
// null key yields nil, nil (the template's else branch renders the placeholder).
// Structural violations - a non-list value, a non-mapping element, an
// empty/newline-bearing title, an empty body, a wrong-typed field - are hard errors
// naming the sidecar. Domain and ADR-link resolution is checkPitfalls' job (it
// needs the project's domains and ADRs); this validates shape only.
// touches-invariant: pitfall-data-validated - pitfall shape validation; proof in pitfalls_test.go
func pitfallEntries(raw any) ([]pitfallEntry, error) {
	if raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, pitfallErr("must be a list of pitfall entries")
	}
	out := make([]pitfallEntry, 0, len(list))
	for i, el := range list {
		m, err := pitfallStringMap(i, el)
		if err != nil {
			return nil, err
		}
		e, err := pitfallEntryFrom(i, m)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// pitfallStringMap normalizes the two shapes yaml.v3 hands a mapping element.
func pitfallStringMap(i int, el any) (map[string]any, error) {
	switch m := el.(type) {
	case map[string]any:
		return m, nil
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			ks, isStr := k.(string)
			if !isStr {
				return nil, pitfallErr(fmt.Sprintf("entry %d: key %v is not a string", i, k))
			}
			out[ks] = v
		}
		return out, nil
	default:
		return nil, pitfallErr(fmt.Sprintf("entry %d must be a mapping", i))
	}
}

// pitfallEntryFrom validates one mapping into a pitfallEntry.
func pitfallEntryFrom(i int, m map[string]any) (pitfallEntry, error) {
	title, err := pitfallString(i, m, "title")
	if err != nil {
		return pitfallEntry{}, err
	}
	if strings.TrimSpace(title) == "" {
		return pitfallEntry{}, pitfallErr(fmt.Sprintf("entry %d: title is empty", i))
	}
	if strings.Contains(title, "\n") {
		return pitfallEntry{}, pitfallErr(fmt.Sprintf("entry %d: title %q contains a newline; titles are single-line headings", i, title))
	}
	body, err := pitfallString(i, m, "body")
	if err != nil {
		return pitfallEntry{}, err
	}
	if strings.TrimSpace(body) == "" {
		return pitfallEntry{}, pitfallErr(fmt.Sprintf("entry %d (%q): body is empty", i, title))
	}
	domains, err := pitfallStrings(i, title, m, "domains")
	if err != nil {
		return pitfallEntry{}, err
	}
	related, err := pitfallInts(i, title, m, "related")
	if err != nil {
		return pitfallEntry{}, err
	}
	tags, err := pitfallStrings(i, title, m, "tags")
	if err != nil {
		return pitfallEntry{}, err
	}
	return pitfallEntry{Title: strings.TrimSpace(title), Domains: domains, Related: related, Tags: tags, Body: strings.TrimRight(body, "\n")}, nil
}

// pitfallString reads a required string field.
func pitfallString(i int, m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", pitfallErr(fmt.Sprintf("entry %d: missing %q", i, key))
	}
	s, isStr := v.(string)
	if !isStr {
		return "", pitfallErr(fmt.Sprintf("entry %d: %q must be a string", i, key))
	}
	return s, nil
}

// pitfallStrings reads an optional list-of-strings field (nil when absent).
func pitfallStrings(i int, title string, m map[string]any, key string) ([]string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	list, isList := v.([]any)
	if !isList {
		return nil, pitfallErr(fmt.Sprintf("entry %d (%q): %q must be a list", i, title, key))
	}
	out := make([]string, 0, len(list))
	for _, el := range list {
		s, isStr := el.(string)
		if !isStr || strings.TrimSpace(s) == "" {
			return nil, pitfallErr(fmt.Sprintf("entry %d (%q): %q entries must be non-empty strings", i, title, key))
		}
		out = append(out, strings.TrimSpace(s))
	}
	return out, nil
}

// pitfallInts reads an optional list-of-ints field (nil when absent).
func pitfallInts(i int, title string, m map[string]any, key string) ([]int, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	list, isList := v.([]any)
	if !isList {
		return nil, pitfallErr(fmt.Sprintf("entry %d (%q): %q must be a list", i, title, key))
	}
	out := make([]int, 0, len(list))
	for _, el := range list {
		n, isInt := el.(int)
		if !isInt {
			return nil, pitfallErr(fmt.Sprintf("entry %d (%q): %q entries must be ADR numbers", i, title, key))
		}
		out = append(out, n)
	}
	return out, nil
}

// pitfallsTransform replaces data.pitfalls - the authored entry list - with the
// finished markdown, mirroring glossaryTransform (ADR-0089). An absent key is left
// untouched; a null/empty list yields "" so the template's else branch renders the
// placeholder.
func pitfallsTransform(sc config.Sidecar) (config.Sidecar, error) {
	raw, ok := sc.Data["pitfalls"]
	if !ok {
		return sc, nil
	}
	entries, err := pitfallEntries(raw)
	if err != nil {
		return sc, err
	}
	out := sc
	out.Data = maps.Clone(sc.Data)
	out.Data["pitfalls"] = pitfallsMarkdown(entries)
	return out, nil
}

// pitfallsMarkdown renders entries in authored order (a YAML sequence is already
// deterministic - no sort needed, unlike the glossary map). Each entry is a
// `## <title>` section, an optional italic Domains line, an optional italic Related
// line of plain ADR-NNNN references (the transform cannot resolve numbers to
// filenames, so these are text not links), and the body.
func pitfallsMarkdown(entries []pitfallEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "## %s\n\n", e.Title)
		if len(e.Domains) > 0 {
			fmt.Fprintf(&b, "_Domains: %s_\n\n", strings.Join(e.Domains, ", "))
		}
		if len(e.Related) > 0 {
			refs := make([]string, len(e.Related))
			for j, n := range e.Related {
				refs[j] = fmt.Sprintf("ADR-%04d", n)
			}
			fmt.Fprintf(&b, "_Related: %s_\n\n", strings.Join(refs, ", "))
		}
		b.WriteString(e.Body)
		b.WriteString("\n")
	}
	return b.String()
}

// pitfallErr prefixes every content violation with the authoring surface.
func pitfallErr(msg string) error {
	return fmt.Errorf("%s data.pitfalls: %s", pitfallsSidecarPath, msg)
}
