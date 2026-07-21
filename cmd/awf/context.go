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

// runContext prints the read-only current-state context for the given
// repo-relative paths: their owning domains, the applicable topics with their
// current claims, any Accepted-ADR pending changes on those topics, and each
// unowned path. Explicit paths and --range query the working universe; --staged
// queries the index universe. When no explicit paths are given, --staged/--range
// resolve them from git first (a bad selector still errors, an empty selector is
// a usage error). It then mirrors runConfig's gate + static-fallback shape: a
// genuinely absent config prints the pre-adoption notice; any other stat fault
// is an error; inside a tree the binary-version gate runs before Open.
func runContext(cwd string, paths []string, staged bool, rng string, asJSON, uncovered bool, stdout io.Writer) error {
	if uncovered {
		return runUncovered(cwd, paths, staged, rng, asJSON, stdout)
	}
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
	if staged {
		if err := gateStaged(cwd); err != nil {
			return err
		}
		res, err := project.StagedContextRoot(cwd, paths)
		if err != nil {
			return err
		}
		return printContext(stdout, res, asJSON, "context: staged state for this project")
	}
	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return printContext(stdout, project.ContextResult{Paths: paths}, asJSON,
			"context (static: not inside an awf project; live context appears inside one)")
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
	return printContext(stdout, res, asJSON, "context: live state for this project")
}

// runUncovered serves `awf context --uncovered`: the whole-tree coverage report.
// Positional args are optional scan roots; --range is rejected. With --staged,
// every input comes from the immutable index universe.
func runUncovered(cwd string, scanRoots []string, staged bool, rng string, asJSON bool, stdout io.Writer) error {
	if rng != "" {
		return &usageErr{"awf context --uncovered takes optional scan-root paths, not --range"}
	}
	if staged {
		if err := gateStaged(cwd); err != nil {
			return err
		}
		res, err := project.StagedUncoveredRoot(cwd, scanRoots)
		if err != nil {
			return err
		}
		return printUncovered(stdout, res, asJSON, "context --uncovered: staged coverage gaps for this project")
	}
	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return printUncovered(stdout, project.UncoveredResult{ScanRoots: project.NormalizeContextPaths(scanRoots)}, asJSON,
			"context --uncovered (static: not inside an awf project; live coverage appears inside one)")
	}
	if err := gate(cwd); err != nil {
		return err
	}
	p, err := project.Open(cwd)
	if err != nil {
		return err
	}
	res, err := p.Uncovered(scanRoots)
	if err != nil {
		return err
	}
	return printUncovered(stdout, res, asJSON, "context --uncovered: coverage gaps for this project")
}

// printUncovered renders res as JSON or human-readable text. Both modes read the
// same assembled res, so they cannot diverge.
func printUncovered(stdout io.Writer, res project.UncoveredResult, asJSON bool, header string) error {
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	fmt.Fprintln(stdout, header)
	if len(res.ScanRoots) > 0 {
		fmt.Fprintf(stdout, "\nscan roots: %v\n", res.ScanRoots)
	}
	if len(res.Uncovered) == 0 && len(res.Unowned) == 0 {
		fmt.Fprintln(stdout, "\nall scanned paths are owned and covered by a scoped topic")
		return nil
	}
	if len(res.Uncovered) > 0 {
		fmt.Fprintln(stdout, "\n## Uncovered (owned by a domain, no scoped topic)")
		for _, u := range res.Uncovered {
			fmt.Fprintf(stdout, "  %s (%s)\n", u.Path, u.Domain)
		}
	}
	if len(res.Unowned) > 0 {
		fmt.Fprintln(stdout, "\n## Unowned (configure a domain to own these)")
		for _, u := range res.Unowned {
			fmt.Fprintf(stdout, "  %s\n", u)
		}
	}
	return nil
}

// printContext renders res as JSON or human-readable text. Both modes read the
// same assembled res, so they cannot diverge.
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
		fmt.Fprintf(stdout, "  %s: %s\n", d.Name, d.CurrentState)
	}
	fmt.Fprintln(stdout, "\n## Topics")
	for _, t := range res.Topics {
		label := t.ID
		if t.Global {
			label += " (global)"
		}
		if t.Title != "" {
			fmt.Fprintf(stdout, "  %s - %s\n", label, t.Title)
		} else {
			fmt.Fprintf(stdout, "  %s\n", label)
		}
		fmt.Fprintf(stdout, "    %s\n", t.Summary)
		for _, c := range t.Claims {
			fmt.Fprintf(stdout, "    [%s] %s: %s\n", c.Type, c.ID, c.Prose)
			if c.Backing != "" {
				fmt.Fprintf(stdout, "      Backing: %s\n", c.Backing)
			}
			if c.Verify != "" {
				fmt.Fprintf(stdout, "      Verify: %s\n", c.Verify)
			}
		}
	}
	if len(res.Pending) > 0 {
		fmt.Fprintln(stdout, "\n## Pending accepted changes (not yet current)")
		for _, pc := range res.Pending {
			fmt.Fprintf(stdout, "  ADR-%s (%s) %s %s\n", pc.ADR, pc.Title, pc.Op, pc.Claim)
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
