// Package initspec resolves awf init answers against the catalog's value
// descriptors and emits the descriptor schema (ADR-0029). It bridges the
// catalog's VarDescriptor set to a resolved (vars, invariants-config, catalog-trim)
// triple via explicit answers, an optional line-based prompter, or the silent
// default.
package initspec

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

// Describe marshals the descriptor set as JSON ({"descriptors": [...]}) for
// `awf init --describe`. An empty Target is normalized to "var".
func Describe(descs []catalog.VarDescriptor) ([]byte, error) {
	out := make([]catalog.VarDescriptor, len(descs))
	for i, d := range descs {
		if d.Target == "" {
			d.Target = "var"
		}
		out[i] = d
	}
	return json.MarshalIndent(map[string]any{"descriptors": out}, "", "  ")
}

// CatalogVars returns the catalog's value descriptors with the catalog-trim
// multiselect descriptors' Options and Default computed from the catalog itself:
// Options lists every skill (or doc) name sorted, and Default comma-joins the
// curated-core names (the pre-selected set). Other descriptors pass through
// unchanged. Describe and Resolve operate on the returned slice so the trim option
// list stays derived from the catalog (ADR-0029).
func CatalogVars(cat *catalog.Catalog) []catalog.VarDescriptor {
	skills := map[string]bool{}
	for name, spec := range cat.Skills {
		skills[name] = spec.Core
	}
	// No doc carries Core any longer (ADR-0043): every name is a non-core option.
	docs := map[string]bool{}
	for name := range cat.Docs {
		docs[name] = false
	}
	out := make([]catalog.VarDescriptor, len(cat.Vars))
	for i, d := range cat.Vars {
		switch d.Target {
		case "catalog-skills":
			d.Options, d.Default = namesAndCore(skills)
		case "catalog-docs":
			d.Options, d.Default = namesAndCore(docs)
		}
		out[i] = d
	}
	return out
}

// namesAndCore returns every name (sorted) and the comma-joined subset whose value
// is true (the core, pre-selected set).
func namesAndCore(core map[string]bool) ([]string, string) {
	all := slices.Sorted(maps.Keys(core))
	var coreNames []string
	for _, n := range all {
		if core[n] {
			coreNames = append(coreNames, n)
		}
	}
	return all, strings.Join(coreNames, ",")
}

// ParseAnswersFile parses a flat key→value answer map from JSON or YAML bytes.
func ParseAnswersFile(b []byte) (map[string]string, error) {
	m := map[string]string{}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("initspec: parse answers: %w", err)
	}
	return m, nil
}

// MergeSetFlags overlays "key=value" strings onto base (later wins).
func MergeSetFlags(base map[string]string, sets []string) error {
	for _, s := range sets {
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" {
			return fmt.Errorf("initspec: --set %q is not key=value", s)
		}
		base[k] = v
	}
	return nil
}

// Resolve maps descriptors + answers to a vars map, an optional invariants config,
// an optional catalog trim, and the resolved commit-scope list. For a string/enum
// descriptor the value is: the
// explicit answer if present; otherwise an interactive prompt (when interactive);
// otherwise empty. A multiselect descriptor resolves to a verbatim selection (see
// resolveMultiselect) routed to the catalog-skills/catalog-docs trim dimension. The
// invariants-marker/globs targets are collected into a *config.InvariantConfig:
// both non-empty → enabled config; exactly one → error; neither → nil.
func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool) (map[string]string, *config.InvariantConfig, *config.CatalogTrim, []string, error) {
	vars := map[string]string{}
	var marker, globs, scopesRaw string
	var skillsSel, docsSel *[]string
	r := bufio.NewReader(in)
	for _, d := range descs {
		if d.Kind == "multiselect" {
			sel, selected, err := resolveMultiselect(r, out, d, answers, interactive)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if selected {
				switch d.Target {
				case "catalog-skills":
					chosen := sel
					skillsSel = &chosen
				case "catalog-docs":
					chosen := sel
					docsSel = &chosen
				}
			}
			continue
		}
		val, ok := answers[d.Key]
		if !ok {
			if interactive {
				p, err := prompt(r, out, d)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				val = p
			} else {
				val = ""
			}
		}
		switch d.Target {
		case "invariants-marker":
			marker = val
		case "invariants-globs":
			globs = val
		case "audit-scopes":
			scopesRaw = val
		default:
			vars[d.Key] = val
		}
	}

	var gs []string
	for _, g := range strings.Split(globs, ",") {
		if g = strings.TrimSpace(g); g != "" {
			gs = append(gs, g)
		}
	}
	var inv *config.InvariantConfig
	switch {
	case marker == "" && len(gs) == 0:
		// inv stays nil: no invariants config supplied (decide on parsed globs, so
		// a whitespace-only globs value counts as unset).
	case marker == "" || len(gs) == 0:
		return nil, nil, nil, nil, errors.New("initspec: invariantsMarker and invariantsGlobs must be set together")
	default:
		inv = &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: gs, Marker: marker}}}
	}

	var trim *config.CatalogTrim
	if skillsSel != nil || docsSel != nil {
		trim = &config.CatalogTrim{Skills: skillsSel, Docs: docsSel}
	}
	return vars, inv, trim, splitNames(scopesRaw), nil
}

// resolveMultiselect resolves one multiselect descriptor to a selection plus a
// "selected" flag: the explicit answer (comma-separated names, each validated
// against the descriptor's options) if present; an interactive prompt (1-based
// option numbers for the complete desired set) when interactive; otherwise not
// selected. selected=false means "no selection: keep the scaffold's curated-core
// default"; selected=true carries the verbatim set (possibly empty = deselect all).
func resolveMultiselect(r *bufio.Reader, out io.Writer, d catalog.VarDescriptor, answers map[string]string, interactive bool) ([]string, bool, error) {
	if raw, ok := answers[d.Key]; ok {
		sel := splitNames(raw)
		for _, n := range sel {
			if !slices.Contains(d.Options, n) {
				return nil, false, fmt.Errorf("initspec: %s: unknown option %q", d.Key, n)
			}
		}
		return sel, true, nil
	}
	if !interactive {
		return nil, false, nil
	}
	return promptMultiselect(r, out, d)
}

// promptMultiselect renders the numbered option list (core marked [x]) and reads a
// complete selection as comma-separated 1-based numbers. Empty input keeps the core
// default (selected=false); an out-of-range or non-numeric token errors.
func promptMultiselect(r *bufio.Reader, out io.Writer, d catalog.VarDescriptor) ([]string, bool, error) {
	core := map[string]bool{}
	for _, n := range splitNames(d.Default) {
		core[n] = true
	}
	fmt.Fprintf(out, "%s — %s\n", d.Key, d.Description)
	for i, o := range d.Options {
		mark := " "
		if core[o] {
			mark = "x"
		}
		fmt.Fprintf(out, "  %d) [%s] %s\n", i+1, mark, o)
	}
	fmt.Fprint(out, "  enter full selection (comma-sep numbers), empty=keep: ")
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, false, fmt.Errorf("initspec: read input: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, false, nil
	}
	var sel []string
	for _, tok := range strings.Split(line, ",") {
		if tok = strings.TrimSpace(tok); tok == "" {
			continue
		}
		n, e := strconv.Atoi(tok)
		if e != nil || n < 1 || n > len(d.Options) {
			return nil, false, fmt.Errorf("initspec: %s: invalid option %q", d.Key, tok)
		}
		sel = append(sel, d.Options[n-1])
	}
	return sel, true, nil
}

// splitNames trims and drops empties from a comma-separated string.
func splitNames(s string) []string {
	var out []string
	for _, n := range strings.Split(s, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// prompt reads one line for descriptor d, returning d.Default on empty input.
// For an enum, a numeric reply selects the option at that 1-based index.
func prompt(r *bufio.Reader, out io.Writer, d catalog.VarDescriptor) (string, error) {
	fmt.Fprintf(out, "%s — %s\n", d.Key, d.Description)
	if d.Kind == "enum" {
		for i, o := range d.Options {
			fmt.Fprintf(out, "  %d) %s\n", i+1, o)
		}
	} else if len(d.Options) > 0 {
		fmt.Fprintf(out, "  e.g. %s\n", strings.Join(d.Options, ", "))
	}
	fmt.Fprintf(out, "  [%s]: ", d.Default)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("initspec: read input: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return d.Default, nil
	}
	if d.Kind == "enum" {
		var n int
		if _, e := fmt.Sscanf(line, "%d", &n); e == nil && n >= 1 && n <= len(d.Options) {
			return d.Options[n-1], nil
		}
	}
	return line, nil
}
