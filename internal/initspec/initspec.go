// Package initspec resolves awf init answers against the catalog's value
// descriptors and emits the descriptor schema (ADR-0029). It bridges the
// catalog's VarDescriptor set to a resolved (vars, invariants-config) pair via
// explicit answers, an optional line-based prompter, or the silent default.
package initspec

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// Resolve maps descriptors + answers to a vars map and an optional invariants
// config. For each descriptor the value is: the explicit answer if present;
// otherwise an interactive prompt (when interactive); otherwise empty. The
// invariants-marker/globs targets are collected into a *config.InvariantConfig:
// both non-empty → enabled config; exactly one → error; neither → nil.
func Resolve(descs []catalog.VarDescriptor, answers map[string]string, in io.Reader, out io.Writer, interactive bool) (map[string]string, *config.InvariantConfig, error) {
	vars := map[string]string{}
	var marker, globs string
	r := bufio.NewReader(in)
	for _, d := range descs {
		val, ok := answers[d.Key]
		if !ok {
			if interactive {
				p, err := prompt(r, out, d)
				if err != nil {
					return nil, nil, err
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
		default:
			vars[d.Key] = val
		}
	}

	var inv *config.InvariantConfig
	switch {
	case marker == "" && globs == "":
		// inv stays nil: no invariants config supplied.
	case marker == "" || globs == "":
		return nil, nil, errors.New("initspec: invariantsMarker and invariantsGlobs must be set together")
	default:
		var gs []string
		for _, g := range strings.Split(globs, ",") {
			if g = strings.TrimSpace(g); g != "" {
				gs = append(gs, g)
			}
		}
		inv = &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: gs, Marker: marker}}}
	}
	return vars, inv, nil
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
