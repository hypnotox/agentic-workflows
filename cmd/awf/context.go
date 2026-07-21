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
	gitSelected := len(paths) == 0
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
		var res project.ContextResult
		var err error
		if gitSelected {
			res, err = project.StagedContextRootGitSelection(cwd, paths)
		} else {
			res, err = project.StagedContextRoot(cwd, paths)
		}
		if err != nil {
			return err
		}
		return printContext(stdout, res, asJSON, "context: staged state for this project")
	}
	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return printContext(stdout, project.ContextResult{Projection: project.ContextConcise, Requests: []project.ContextRequest{}, Paths: []project.ContextPath{}}, asJSON,
			"context (static: not inside an awf project; live classification and authority require an adopted project)")
	}
	if err := gate(cwd); err != nil {
		return err
	}
	p, err := project.Open(cwd)
	if err != nil {
		return err
	}
	var res project.ContextResult
	if gitSelected {
		res, err = p.ContextForGitSelection(paths)
	} else {
		res, err = p.ContextFor(paths)
	}
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
	fmt.Fprintf(stdout, "Projection: %s\n", res.Projection)
	fmt.Fprintln(stdout, "\n## Requests")
	for _, r := range res.Requests {
		fmt.Fprintf(stdout, "  %s [%s]: %v\n", r.Query, r.Status, r.EffectivePaths)
	}
	fmt.Fprintln(stdout, "\n## Effective paths")
	for _, p := range res.Paths {
		fmt.Fprintf(stdout, "\n%s [%s] (requests: %v)\n", p.Path, p.Classification, p.Requests)
		if p.NestedRoot != "" {
			fmt.Fprintf(stdout, "  Nested root: %s/.awf/config.yaml\n", p.NestedRoot)
		}
		if p.TargetInsideRepository != nil {
			fmt.Fprintf(stdout, "  Symlink target inside repository: %t\n", *p.TargetInsideRepository)
		}
		for _, d := range p.Domains {
			fmt.Fprintf(stdout, "  Domain: %s (%s)\n", d.Name, d.CurrentState)
		}
		for _, t := range p.Topics {
			fmt.Fprintf(stdout, "  Topic: %s - %s\n", t.ID, t.Title)
			fmt.Fprintf(stdout, "    Domain paths: %v\n    Topic paths: %v\n    Both domain and topic selectors must match.\n    Matched paths: %v\n", t.Applicability.DomainPaths, t.Applicability.TopicPaths, t.Applicability.MatchedPaths)
			for _, site := range t.Applicability.MarkerSites {
				fmt.Fprintf(stdout, "    Marker: %s:%d [%s] %s\n", site.Path, site.Line, site.Kind, site.ClaimID)
			}
		}
		for _, a := range p.Artifacts {
			fmt.Fprintf(stdout, "  Artifact: %s %s\n", a.Role, a.Identity)
		}
	}
	return nil
}
