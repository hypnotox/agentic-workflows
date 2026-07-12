package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runContext prints the read-only context for the given repo-relative paths:
// owning domains, backed invariants, related ADRs, and each domain's rendered
// current-state pointer. When no explicit paths are given, --staged/--range
// resolve them from git first (a bad selector still errors, an empty selector is
// a usage error) — placed before the static-fallback stat so the resolved paths
// carry into the outside-a-tree output. It then mirrors runConfig's gate +
// static-fallback shape: a genuinely absent config prints the pre-adoption
// notice; any other stat fault is an error; inside a tree the binary-version
// gate runs before Open. The command entry point holds no writer dependency —
// it only reads.
// invariant: context-read-only
func runContext(cwd string, paths []string, staged bool, rng string, asJSON bool, stdout io.Writer) error {
	if len(paths) == 0 {
		if !staged && rng == "" {
			return &usageErr{"usage: awf context <path>... [--json] [--staged] [--range <a>..<b>]"}
		}
		resolved, err := awfgit.ChangedPaths(cwd, staged, rng)
		if err != nil {
			return err
		}
		if len(resolved) == 0 {
			return &usageErr{"awf context: no changed paths for the given selector"}
		}
		paths = resolved
	}
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
	if len(res.Plans) > 0 {
		fmt.Fprintln(stdout, "\n## Related plans")
		for _, pl := range res.Plans {
			fmt.Fprintf(stdout, "  %s (%s) — %s\n", pl.Filename, pl.Status, pl.Path)
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
