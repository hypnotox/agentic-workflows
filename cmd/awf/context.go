package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

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
	return runContextProjection(cwd, paths, staged, rng, asJSON, uncovered, false, stdout)
}

func runContextProjection(cwd string, paths []string, staged bool, rng string, asJSON, uncovered, full bool, stdout io.Writer) error {
	if full && uncovered {
		return &usageErr{"awf context: --full cannot be combined with --uncovered"}
	}
	if uncovered {
		return runUncovered(cwd, paths, staged, rng, asJSON, stdout)
	}
	gitSelected := len(paths) == 0
	if len(paths) == 0 {
		if !staged && rng == "" {
			return &usageErr{"usage: awf context <path>... [--json] [--full] [--staged] [--range <a>..<b>]"}
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
		switch {
		case full && gitSelected:
			res, err = project.StagedContextRootFullGitSelection(cwd, paths)
		case full:
			res, err = project.StagedContextRootFull(cwd, paths)
		case gitSelected:
			res, err = project.StagedContextRootGitSelection(cwd, paths)
		default:
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
		projection := project.ContextConcise
		if full {
			projection = project.ContextFull
		}
		return printContext(stdout, project.ContextResult{Projection: projection, Requests: []project.ContextRequest{}, Topics: []project.InvocationTopicContext{}, Paths: []project.ContextPath{}}, asJSON,
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
	switch {
	case full && gitSelected:
		res, err = p.ContextForFullGitSelection(paths)
	case full:
		res, err = p.ContextForFull(paths)
	case gitSelected:
		res, err = p.ContextForGitSelection(paths)
	default:
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
			if u.Path != "." && !strings.HasSuffix(u.Path, "/") {
				fmt.Fprintf(stdout, "  %s\n", u.Path)
				continue
			}
			fmt.Fprintf(stdout, "  %s (%s", u.Path, countNoun(u.UnownedCount, "unowned file"))
			if u.ExcludedCount > 0 {
				fmt.Fprintf(stdout, "; %s excluded from coverage beneath", countNoun(u.ExcludedCount, "file"))
			}
			fmt.Fprintln(stdout, ")")
		}
	}
	return nil
}

// countNoun renders "1 <noun>" or "<n> <noun>s" for the uncovered annotations.
func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// printContext renders res as JSON or human-readable text. Both modes read the
// same assembled res, so they cannot diverge.
func printContext(stdout io.Writer, res project.ContextResult, asJSON bool, header string) error {
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return fmt.Errorf("write context JSON: %w", err)
		}
		return nil
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, header)
	fmt.Fprintf(&out, "Projection: %s\n", res.Projection)
	fmt.Fprintln(&out, "\n## Requests")
	for _, r := range res.Requests {
		fmt.Fprintf(&out, "  %s [%s]: %v\n", r.Query, r.Status, r.EffectivePaths)
	}
	fmt.Fprintln(&out, "\n## Topics")
	for i := 0; i < len(res.Topics); {
		domain := topicDomain(res.Topics[i].ID)
		j := i
		for j < len(res.Topics) && topicDomain(res.Topics[j].ID) == domain {
			j++
		}
		printTopicGroup(&out, domain, res.Topics[i:j])
		i = j
	}
	fmt.Fprintln(&out, "\n## Effective paths")
	for _, p := range res.Paths {
		fmt.Fprintf(&out, "\n%s [%s] (requests: %v)\n", p.Path, p.Classification, p.Requests)
		if p.NestedRoot != "" {
			fmt.Fprintf(&out, "  Nested root: %s\n", p.NestedRoot)
		}
		if p.TargetInsideRepository != nil {
			fmt.Fprintf(&out, "  Symlink target inside repository: %t\n", *p.TargetInsideRepository)
		}
		if p.GlobLiteral {
			fmt.Fprintln(&out, "  globs are not expanded; pass a directory or an exact file")
		}
		if p.Classification == project.PathEligibleUnowned {
			fmt.Fprintln(&out, "  No domain owns this path; add a domain glob to a configured domain to own it (see: awf context --uncovered)")
		}
		for _, d := range p.Domains {
			fmt.Fprintf(&out, "  Domain: %s (%s)\n", d.Name, d.CurrentState)
		}
		for _, t := range p.Topics {
			fmt.Fprintf(&out, "  Topic: %s\n", t.ID)
			if len(t.DirectClaimIDs) > 0 {
				fmt.Fprintf(&out, "    Direct claims: %s\n", strings.Join(t.DirectClaimIDs, ", "))
			}
		}
		for _, a := range p.Artifacts {
			fmt.Fprintf(&out, "  Artifact: %s %s (navigation)\n", a.Role, a.Identity)
			for _, link := range a.Sources {
				fmt.Fprintf(&out, "    Source: %s (%s)\n", link.Path, link.Label)
			}
			for _, link := range a.Outputs {
				fmt.Fprintf(&out, "    Output: %s (%s)\n", link.Path, link.Label)
			}
			for _, link := range a.Navigation {
				fmt.Fprintf(&out, "    Navigate: %s (%s)\n", link.Path, link.Label)
			}
		}
		if p.ADR != nil {
			fmt.Fprintf(&out, "  ADR navigation: ADR-%s %s [%s, %s]\n", p.ADR.Number, p.ADR.Title, p.ADR.Status, p.ADR.Mutability)
			fmt.Fprintf(&out, "    Authority role: %s\n", p.ADR.AuthorityRole)
			for _, operation := range p.ADR.Operations {
				fmt.Fprintf(&out, "    %s %s [%s, %s]", operation.Operation, operation.Claim, operation.Progress, operation.ClaimState)
				if operation.StateSequence != 0 {
					fmt.Fprintf(&out, " state-sequence %d", operation.StateSequence)
				}
				fmt.Fprintln(&out)
				if operation.Detail != nil && operation.Detail.Current != nil {
					printClaimDetail(&out, "Current", *operation.Detail.Current)
				}
			}
		}
	}
	if _, err := stdout.Write(out.Bytes()); err != nil {
		return fmt.Errorf("write context: %w", err)
	}
	return nil
}

// topicDomain returns the domain segment of a domain-qualified topic ID.
func topicDomain(id string) string {
	if i := strings.Index(id, "/"); i >= 0 {
		return id[:i]
	}
	return id // coverage-ignore: every topic ID is a validated domain-qualified ID
}

// printTopicGroup renders one domain's consecutive topics: the domain-selector
// block prints once per group (never for an all-global group), each topic keeps
// its own selector, matched, claim, and detail lines.
func printTopicGroup(out io.Writer, domain string, group []project.InvocationTopicContext) {
	for _, t := range group {
		if !t.Applicability.DeclaredGlobal {
			fmt.Fprintf(out, "\nDomain %s paths: %v\n  Both domain and topic selectors must match.\n", domain, t.Applicability.DomainPaths)
			break
		}
	}
	for _, t := range group {
		fmt.Fprintf(out, "\n%s - %s\n", t.ID, t.Title)
		if t.Applicability.DeclaredGlobal {
			fmt.Fprintf(out, "  Global topic within owning domain selectors: %v\n", t.Applicability.DomainPaths)
		} else {
			fmt.Fprintf(out, "  Topic paths: %v\n", t.Applicability.TopicPaths)
		}
		fmt.Fprintf(out, "  Matched paths: %d (drill down: %s)\n", t.Applicability.MatchedPathCount, t.CoverageCommand)
		fmt.Fprintf(out, "  Claims (%d): %s\n", len(t.ClaimIDs), strings.Join(t.ClaimIDs, ", "))
		if len(t.DirectClaims) > 0 {
			fmt.Fprintln(out, "  Direct claims:")
			for _, claim := range t.DirectClaims {
				printClaimDetail(out, "Direct claim", claim)
			}
		}
		if t.OmittedDetailCount > 0 {
			fmt.Fprintf(out, "  Details omitted for %d claim(s); drill down: %s\n", t.OmittedDetailCount, t.TopicCommand)
		}
		if t.Full != nil {
			fmt.Fprintln(out, "  Full authority:")
			for _, claim := range t.Full.Claims {
				printClaimDetail(out, "Claim", claim)
			}
			for _, pending := range t.Full.Pending {
				fmt.Fprintf(out, "      Pending: ADR-%s %s %s\n", pending.ADR, pending.Op, pending.Claim)
			}
		}
	}
}

func printClaimDetail(out io.Writer, label string, claim project.ClaimDetail) {
	fmt.Fprintf(out, "      %s: %s [%s] %s\n", label, claim.ID, claim.Type, claim.Prose)
	if claim.Backing != "" {
		fmt.Fprintf(out, "        Backing: %s\n", claim.Backing)
	}
	if claim.Verify != "" {
		fmt.Fprintf(out, "        Verify: %s\n", claim.Verify)
	}
	for _, site := range claim.Sites {
		fmt.Fprintf(out, "        Site: %s:%d [%s]\n", site.Path, site.Line, site.Kind)
	}
	if len(claim.References.Incoming) > 0 || len(claim.References.Outgoing) > 0 {
		fmt.Fprintf(out, "        References: incoming %v; outgoing %v\n", claim.References.Incoming, claim.References.Outgoing)
	}
}
