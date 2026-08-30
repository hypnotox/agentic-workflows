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
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
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

// Resolve maps descriptors and answers to a vars map and the resolved
// commit-scope list. For a string or enum descriptor the value is the explicit
// answer if present; otherwise an interactive prompt when applicable; otherwise
// empty. A nil needed filter prompts for every descriptor.
func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool, needed func() (map[string]bool, error)) (map[string]string, []string, error) {
	// An answer key matching no descriptor is a typo that would otherwise
	// no-op silently, leaving the intended var empty.
	known := map[string]bool{}
	for _, d := range descs {
		known[d.Key] = true
	}
	for _, k := range slices.Sorted(maps.Keys(answers)) {
		if !known[k] {
			return nil, nil, fmt.Errorf("initspec: unknown answer key %q (see awf init --describe)", k)
		}
	}
	vars := map[string]string{}
	var scopesRaw string
	r := &promptReader{r: bufio.NewReader(in)}
	var neededVars map[string]bool
	if needed != nil {
		nv, err := needed()
		if err != nil {
			return nil, nil, err
		}
		neededVars = nv
	}
	for _, d := range descs {
		val, ok := answers[d.Key]
		if !ok {
			// A var no full-catalog template references is seeded empty, never
			// prompted. Explicit answers stay honored.
			skip := neededVars != nil && d.Target == "" && !neededVars[d.Key]
			if interactive && !r.eof && !skip {
				p, err := prompt(r, out, d)
				if err != nil {
					return nil, nil, err
				}
				val = p
			} else {
				val = ""
			}
		}
		if d.Kind == "enum" && val != "" && !slices.Contains(d.Options, val) {
			return nil, nil, fmt.Errorf("initspec: %s: invalid value %q (options: %s)", d.Key, val, strings.Join(d.Options, ", "))
		}
		switch d.Target {
		case "audit-scopes":
			scopesRaw = val
		default:
			vars[d.Key] = val
		}
	}
	return vars, splitNames(scopesRaw), nil
}

// writePrompt is the sole interactive init write. It validates and buffers the
// complete ordinary-tree prelude, then writes one flushed non-newline tail.
func writePrompt(out io.Writer, descriptor catalog.VarDescriptor, options []string, tail string) error {
	key, err := presentation.Literal(descriptor.Key)
	if err != nil {
		return err
	}
	variable, err := presentation.NewField("variable", key)
	if err != nil { // coverage-ignore: fixed label and validated literal cannot fail
		return err
	}
	descriptionText := descriptor.Description
	if descriptionText == "" {
		descriptionText = "configuration value"
	}
	descriptionValue, err := presentation.Prose(descriptionText)
	if err != nil {
		return err // coverage-ignore: empty descriptions become the nonempty fallback above
	}
	description, err := presentation.NewField("description", descriptionValue)
	if err != nil { // coverage-ignore: fixed label and validated prose cannot fail
		return err // coverage-ignore: the fixed label and validated value cannot fail
	}
	nodes := []presentation.Node{description}
	if len(options) > 0 {
		values := make([]presentation.Value, len(options))
		for i, option := range options {
			values[i], err = presentation.Prose(option)
			if err != nil {
				return err
			}
		}
		list, err := presentation.NewList("options", values...)
		if err != nil { // coverage-ignore: fixed label and validated values cannot fail
			return err // coverage-ignore: fixed label and validated option values cannot fail
		}
		nodes = append(nodes, list)
	}
	input, err := presentation.NewSection("input", nodes...)
	if err != nil { // coverage-ignore: fixed label and validated child nodes cannot fail
		return err // coverage-ignore: fixed label and validated child nodes cannot fail
	}
	prelude, err := presentation.NewDocument(variable, input)
	if err != nil { // coverage-ignore: validated root field and section cannot fail
		return err // coverage-ignore: validated root field and section cannot fail
	}
	tailValue, err := presentation.Prose(tail)
	if err != nil {
		return err
	}
	return presentation.Prompt(out, prelude, tailValue)
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
	options := []string(nil)
	if d.Kind == "enum" {
		options = make([]string, len(d.Options))
		for i, o := range d.Options {
			options[i] = fmt.Sprintf("%d %s", i+1, o)
		}
	} else if len(d.Options) > 0 {
		options = []string{"for example " + strings.Join(d.Options, ", ")}
	}
	tail := "default " + d.Default
	if d.Default == "" {
		tail = "default is empty"
	}
	if err := writePrompt(out, d, options, tail); err != nil {
		return "", err
	}
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
