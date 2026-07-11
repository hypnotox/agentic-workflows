package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runContext prints the read-only context for the given repo-relative paths:
// owning domains, backed invariants, related ADRs, and each domain's rendered
// current-state pointer. It mirrors runConfig's gate + static-fallback shape: a
// genuinely absent config prints the pre-adoption notice; any other stat fault
// is an error; inside a tree the binary-version gate runs before Open.
func runContext(cwd string, paths []string, asJSON bool, stdout io.Writer) error {
	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		// invariant: context-static-fallback
		return printContext(stdout, project.ContextResult{Paths: paths}, asJSON,
			"context (static — not inside an awf project; live context appears inside one)")
	}
	if err := gate(cwd); err != nil {
		return err
	}
	p, err := project.Open(cwd)
	if err != nil {
		return err
	}
	res, err := p.ContextFor(paths)
	if err != nil {
		return err
	}
	return printContext(stdout, res, asJSON, "context — live state for this project")
}

// printContext renders res as JSON or human-readable text. Both modes read the
// same assembled res, so they cannot diverge.
// invariant: context-output-parity
func printContext(stdout io.Writer, res project.ContextResult, asJSON bool, header string) error {
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	fmt.Fprintln(stdout, header)
	fmt.Fprintf(stdout, "\npaths: %v\n", res.Paths)
	fmt.Fprintln(stdout, "\n## Domains")
	for _, d := range res.Domains {
		fmt.Fprintf(stdout, "  %s — %s\n", d.Name, d.CurrentState)
	}
	fmt.Fprintln(stdout, "\n## Invariants")
	for _, s := range res.Invariants {
		fmt.Fprintf(stdout, "  %s\n", s)
	}
	fmt.Fprintln(stdout, "\n## Related ADRs")
	for _, a := range res.ADRs {
		fmt.Fprintf(stdout, "  ADR-%s (%s) %s — %s\n", a.Number, a.Status, a.Title, a.Path)
		if len(a.Invariants) > 0 {
			fmt.Fprintf(stdout, "    invariants: %v\n", a.Invariants)
		}
	}
	if len(res.Unowned) > 0 {
		fmt.Fprintln(stdout, "\n## Unowned paths (no configured domain)")
		for _, u := range res.Unowned {
			fmt.Fprintf(stdout, "  %s\n", u)
		}
	}
	return nil
}
