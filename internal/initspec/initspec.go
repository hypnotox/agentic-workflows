// Package initspec resolves awf init answers against the catalog's value
// descriptors and emits the descriptor schema (ADR-0029). It bridges the
// catalog's VarDescriptor set to a resolved (vars, catalog-trim) pair plus the
// commit-scope list via explicit answers, an optional line-based prompter, or the
// silent default.
package initspec

import (
	"bufio"
	"encoding/json"
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
	// No doc carries Core any longer (ADR-0043); Mandatory singletons are excluded
	// from the toggleable pool (ADR-0061). Every remaining name is a non-core option.
	docs := map[string]bool{}
	for name, e := range cat.Docs {
		if !e.Mandatory {
			docs[name] = false
		}
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

// promptReader wraps the prompt input and latches EOF, so Resolve stops
// prompting (and stops emitting prompt text) once the input is exhausted -
// an init reading /dev/null or a closed stdin degrades to the silent path
// instead of streaming every remaining prompt to nobody.
type promptReader struct {
	r   *bufio.Reader
	eof bool
}

// line reads one line; EOF is latched, not returned - the partial line (or
// empty string) read alongside it is still the answer.
func (pr *promptReader) line() (string, error) {
	s, err := pr.r.ReadString('\n')
	if err == io.EOF {
		pr.eof = true
		return s, nil
	}
	if err != nil {
		return "", fmt.Errorf("initspec: read input: %w", err)
	}
	return s, nil
}

// Resolve maps descriptors + answers to a vars map, an optional catalog trim, and
// the resolved commit-scope list. Multiselect descriptors resolve first - the
// trim decides which var prompts are worth asking (ADR-0086 Decision 6), so
// artifact selection precedes var entry. For a string/enum descriptor the value
// is: the explicit answer if present; otherwise an interactive prompt (when
// interactive and, given a needed filter, the var is one the selection's
// templates reference); otherwise empty. A nil needed prompts for everything.
// A multiselect descriptor resolves to a verbatim selection (see
// resolveMultiselect) routed to the catalog-skills/catalog-docs trim dimension.
func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool, needed func(*config.CatalogTrim) (map[string]bool, error)) (map[string]string, *config.CatalogTrim, []string, error) {
	// An answer key matching no descriptor is a typo that would otherwise
	// no-op silently, leaving the intended var empty.
	known := map[string]bool{}
	for _, d := range descs {
		known[d.Key] = true
	}
	for _, k := range slices.Sorted(maps.Keys(answers)) {
		if !known[k] {
			return nil, nil, nil, fmt.Errorf("initspec: unknown answer key %q (see awf init --describe)", k)
		}
	}
	vars := map[string]string{}
	var scopesRaw string
	var skillsSel, docsSel *[]string
	r := &promptReader{r: bufio.NewReader(in)}
	for _, d := range descs {
		if d.Kind != "multiselect" {
			continue
		}
		sel, selected, err := resolveMultiselect(r, out, d, answers, interactive && !r.eof)
		if err != nil {
			return nil, nil, nil, err
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
	}
	var trim *config.CatalogTrim
	if skillsSel != nil || docsSel != nil {
		trim = &config.CatalogTrim{Skills: skillsSel, Docs: docsSel}
	}
	var neededVars map[string]bool
	if needed != nil {
		nv, err := needed(trim)
		if err != nil {
			return nil, nil, nil, err
		}
		neededVars = nv
	}
	for _, d := range descs {
		if d.Kind == "multiselect" {
			continue
		}
		val, ok := answers[d.Key]
		if !ok {
			// A var no template of the scaffolded enabled set references is
			// seeded empty, never prompted (ADR-0086 Decision 6): a typed
			// answer for it could only become unused-var drift. Explicit
			// answers (the ok branch above) stay honored.
			// invariant: init-prompts-enabled-vars
			skip := neededVars != nil && d.Target == "" && !neededVars[d.Key]
			if interactive && !r.eof && !skip {
				p, err := prompt(r, out, d)
				if err != nil {
					return nil, nil, nil, err
				}
				val = p
			} else {
				val = ""
			}
		}
		// Enum values are validated like multiselect options; a typed exact
		// option (interactive non-numeric input) passes, garbage errors.
		if d.Kind == "enum" && val != "" && !slices.Contains(d.Options, val) {
			return nil, nil, nil, fmt.Errorf("initspec: %s: invalid value %q (options: %s)", d.Key, val, strings.Join(d.Options, ", "))
		}
		switch d.Target {
		case "audit-scopes":
			scopesRaw = val
		default:
			vars[d.Key] = val
		}
	}
	return vars, trim, splitNames(scopesRaw), nil
}

// resolveMultiselect resolves one multiselect descriptor to a selection plus a
// "selected" flag: the explicit answer (comma-separated names, each validated
// against the descriptor's options) if present; an interactive prompt (1-based
// option numbers for the complete desired set) when interactive; otherwise not
// selected. selected=false means "no selection: keep the scaffold's curated-core
// default"; selected=true carries the verbatim set (possibly empty = deselect all).
func resolveMultiselect(r *promptReader, out io.Writer, d catalog.VarDescriptor, answers map[string]string, interactive bool) ([]string, bool, error) {
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
func promptMultiselect(r *promptReader, out io.Writer, d catalog.VarDescriptor) ([]string, bool, error) {
	core := map[string]bool{}
	for _, n := range splitNames(d.Default) {
		core[n] = true
	}
	fmt.Fprintf(out, "%s: %s\n", d.Key, d.Description)
	for i, o := range d.Options {
		mark := " "
		if core[o] {
			mark = "x"
		}
		fmt.Fprintf(out, "  %d) [%s] %s\n", i+1, mark, o)
	}
	fmt.Fprint(out, "  enter full selection (comma-sep numbers), empty=keep: ")
	line, err := r.line()
	if err != nil {
		return nil, false, err
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
func prompt(r *promptReader, out io.Writer, d catalog.VarDescriptor) (string, error) {
	fmt.Fprintf(out, "%s: %s\n", d.Key, d.Description)
	if d.Kind == "enum" {
		for i, o := range d.Options {
			fmt.Fprintf(out, "  %d) %s\n", i+1, o)
		}
	} else if len(d.Options) > 0 {
		fmt.Fprintf(out, "  e.g. %s\n", strings.Join(d.Options, ", "))
	}
	fmt.Fprintf(out, "  [%s]: ", d.Default)
	line, err := r.line()
	if err != nil {
		return "", err
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
