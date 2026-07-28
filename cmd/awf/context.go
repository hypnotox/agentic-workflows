package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/contextdelivery"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

var deliverContext = contextdelivery.Deliver

func runContext(cwd string, paths []string, staged bool, rng string, uncovered, full bool, shows []string, stdout io.Writer) error {
	facets, err := project.ParseContextFacets(shows, full)
	if err != nil {
		return &usageErr{"awf context: " + err.Error()}
	}
	if uncovered && (full || len(shows) > 0) {
		return &usageErr{"awf context: --show and --full cannot be combined with --uncovered"}
	}
	if uncovered {
		return runUncovered(cwd, paths, staged, rng, stdout)
	}
	selection := project.SelectionExplicit
	if staged {
		selection = project.SelectionStaged
	} else if rng != "" {
		selection = project.SelectionRange
	}
	if len(paths) == 0 {
		if !staged && rng == "" {
			return &usageErr{"usage: awf context <path>... [--show <facet>] [--full] [--staged] [--range <a>..<b>]"}
		}
		resolved, e := awfgit.ChangedPaths(cwd, staged, rng)
		if e != nil {
			return e
		}
		if len(resolved) == 0 {
			return &usageErr{"awf context: no changed paths for the given selector"}
		}
		paths = resolved
	}
	options := project.ContextOptions{Selection: selection, Range: rng, Facets: facets}
	var result project.ContextResult
	header := "context: live state for this project"
	if staged {
		if err := gateStaged(cwd); err != nil {
			return err
		}
		result, err = project.StagedContextRootOptions(cwd, paths, options)
		header = "context: staged state for this project"
	} else if _, statErr := os.Stat(config.ConfigPath(cwd)); statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			return statErr
		}
		result = project.ContextResult{Selection: selection, Range: rng, Requests: []project.ContextRequestReport{}, Topics: []project.TopicImpact{}}
		header = "context (static: not inside an awf project; live classification and authority require an adopted project)"
	} else {
		if err := gate(cwd); err != nil {
			return err
		}
		p, e := project.Open(cwd)
		if e != nil { // coverage-ignore: gate just loaded the same config and project presence; failure requires a concurrent filesystem race
			return e
		}
		result, err = p.ContextForOptions(paths, options)
	}
	if err != nil {
		return err
	}
	var out bytes.Buffer
	renderContext(&out, result, header, facets)
	return deliverContext(out.Bytes(), cwd, stdout)
}

func runUncovered(cwd string, roots []string, staged bool, rng string, stdout io.Writer) error {
	if rng != "" {
		return &usageErr{"awf context --uncovered takes optional scan-root paths, not --range"}
	}
	var result project.UncoveredResult
	var err error
	header := "context --uncovered: coverage gaps for this project"
	if staged {
		if err = gateStaged(cwd); err == nil {
			result, err = project.StagedUncoveredRoot(cwd, roots)
		}
		header = "context --uncovered: staged coverage gaps for this project"
	} else if _, statErr := os.Stat(config.ConfigPath(cwd)); statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			return statErr
		}
		result = project.UncoveredResult{ScanRoots: project.NormalizeContextPaths(roots)}
		header = "context --uncovered (static: not inside an awf project; live coverage appears inside one)"
	} else {
		if err = gate(cwd); err == nil {
			var p *project.Project
			p, err = project.Open(cwd)
			if err == nil { // coverage-ignore: gate just loaded the same project; an Open failure requires a concurrent filesystem race
				result, err = p.Uncovered(roots)
			}
		}
	}
	if err != nil {
		return err
	}
	var out bytes.Buffer
	renderUncovered(&out, result, header)
	return deliverContext(out.Bytes(), cwd, stdout)
}

func renderUncovered(out io.Writer, res project.UncoveredResult, header string) {
	fmt.Fprintln(out, header)
	if len(res.ScanRoots) > 0 {
		fmt.Fprintf(out, "\nscan roots: %v\n", res.ScanRoots)
	}
	if len(res.Uncovered) == 0 && len(res.Unowned) == 0 {
		fmt.Fprintln(out, "\nall scanned paths are owned and covered by a scoped topic")
		return
	}
	if len(res.Uncovered) > 0 {
		fmt.Fprintln(out, "\n## Uncovered (owned by a domain, no scoped topic)")
		for _, u := range res.Uncovered {
			fmt.Fprintf(out, "  %s (%s)\n", u.Path, u.Domain)
		}
	}
	if len(res.Unowned) > 0 {
		fmt.Fprintln(out, "\n## Unowned (configure a domain to own these)")
		for _, u := range res.Unowned {
			if u.Path != "." && !strings.HasSuffix(u.Path, "/") {
				fmt.Fprintf(out, "  %s\n", u.Path)
				continue
			}
			fmt.Fprintf(out, "  %s (%s", u.Path, countNoun(u.UnownedCount, "unowned file"))
			if u.ExcludedCount > 0 {
				fmt.Fprintf(out, "; %s excluded from coverage beneath", countNoun(u.ExcludedCount, "file"))
			}
			fmt.Fprintln(out, ")")
		}
	}
}
func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func renderContext(out io.Writer, res project.ContextResult, header string, facets []project.ContextFacet) {
	fmt.Fprintln(out, header)
	if res.Selection == project.SelectionRange {
		fmt.Fprintf(out, "Selection: range %s\n", res.Range)
	} else {
		fmt.Fprintf(out, "Selection: %s\n", res.Selection)
	}
	fmt.Fprintln(out, "\n## Requests")
	if len(res.Requests) == 0 {
		fmt.Fprintln(out, "  none")
	}
	for _, request := range res.Requests {
		fmt.Fprintf(out, "[%d] %s\n", request.Index, request.Argument)
		if request.Directory != nil {
			excluded := 0
			for _, c := range request.Directory.Excluded {
				excluded += c.Count
			}
			fmt.Fprintf(out, "  Directory: %d included; %d excluded\n", request.Directory.Included, excluded)
			if len(request.Directory.Excluded) > 0 {
				parts := []string{}
				for _, c := range request.Directory.Excluded {
					parts = append(parts, fmt.Sprintf("%s=%d", c.Classification, c.Count))
				}
				fmt.Fprintf(out, "  Excluded: %s\n", strings.Join(parts, ", "))
			}
			for i, g := range request.Directory.Groups {
				fmt.Fprintf(out, "  Group %d: %d files\n", i+1, g.Count)
				if len(g.Members) > 0 {
					fmt.Fprintf(out, "    Members: %s\n", strings.Join(g.Members, ", "))
				}
				renderPathImpact(out, g.Context, "    ", facets)
			}
			if containsFacet(facets, project.FacetRelationships) {
				renderRelationships(out, request.Directory.Relationships, "  ")
			}
		} else if request.Exact != nil {
			fmt.Fprintf(out, "  File: %s\n", request.Exact.Path)
			renderPathImpact(out, request.Exact.Context, "  ", facets)
			renderRelationships(out, request.Exact.Context.Relationships, "  ")
		}
	}
	fmt.Fprintln(out, "\n## Authority")
	if len(res.Topics) == 0 {
		fmt.Fprintln(out, "  none")
	}
	for _, impact := range res.Topics {
		renderTopicImpact(out, impact)
	}
}

func renderPathImpact(out io.Writer, impact project.ContextPathImpact, indent string, facets []project.ContextFacet) {
	fmt.Fprintf(out, "%sClassification: %s\n", indent, impact.Classification)
	if impact.NestedRoot != "" {
		fmt.Fprintf(out, "%sNested root: %s\n", indent, impact.NestedRoot)
	}
	if impact.TargetInsideRepository != nil {
		fmt.Fprintf(out, "%sSymlink target inside repository: %t\n", indent, *impact.TargetInsideRepository)
	}
	if len(impact.Provenance) == 0 {
		fmt.Fprintf(out, "%sProvenance: none\n", indent)
	} else {
		for _, p := range impact.Provenance {
			fmt.Fprintf(out, "%sProvenance: %s %s\n", indent, p.Role, p.Identity)
			if containsFacet(facets, project.FacetArtifacts) {
				for _, e := range p.Sources {
					fmt.Fprintf(out, "%s  Source: %s (%s)\n", indent, e.Path, e.Label)
				}
				for _, e := range p.Outputs {
					fmt.Fprintf(out, "%s  Output: %s (%s)\n", indent, e.Path, e.Label)
				}
				for _, e := range p.Navigation {
					fmt.Fprintf(out, "%s  Navigate: %s (%s)\n", indent, e.Path, e.Label)
				}
			}
		}
	}
	domains := []string{}
	for _, d := range impact.Domains {
		domains = append(domains, d.Name)
	}
	topics := []string{}
	for _, t := range impact.Topics {
		topics = append(topics, t.ID)
	}
	fmt.Fprintf(out, "%sDomains: %s\n", indent, renderList(domains))
	fmt.Fprintf(out, "%sTopics: %s\n", indent, renderList(topics))
	for _, w := range impact.Warnings {
		fmt.Fprintf(out, "%sWarning: %s\n", indent, w)
	}
	if impact.ADR != nil {
		a := impact.ADR
		fmt.Fprintf(out, "%sADR: ADR-%s %s [%s, %s]\n", indent, a.Number, a.Title, a.Status, a.Mutability)
		fmt.Fprintf(out, "%sAuthority role: %s\n", indent, a.AuthorityRole)
		for _, op := range a.Operations {
			fmt.Fprintf(out, "%sOperation: %s %s [%s, %s", indent, op.Operation, op.Claim, op.Progress, op.ClaimState)
			if op.StateSequence != 0 {
				fmt.Fprintf(out, ", state-sequence %d", op.StateSequence)
			}
			fmt.Fprintln(out, "]")
			if op.Detail != nil {
				if op.Detail.Current != nil {
					fmt.Fprintf(out, "%s  Current claim: %s [%s] %s\n", indent, op.Detail.Current.ID, op.Detail.Current.Type, op.Detail.Current.Summary)
				}
				if op.Detail.History != nil && op.Detail.History.RemovedBy != nil {
					fmt.Fprintf(out, "%s  Removal history: removed by ADR-%s at state-sequence %d\n", indent, op.Detail.History.RemovedBy.Number, op.Detail.History.RemovedBy.StateSequence)
				}
				renderEvidence(out, op.Detail.Current, op.Detail.Evidence, indent+"  ")
			}
		}
	}
}

func renderRelationships(out io.Writer, relationships project.ContextRelationships, indent string) {
	for _, relationship := range []struct {
		label string
		ids   []string
	}{
		{label: "State", ids: relationships.State},
		{label: "Touches", ids: relationships.Touches},
		{label: "Proofs", ids: relationships.Proofs},
	} {
		if len(relationship.ids) > 0 {
			fmt.Fprintf(out, "%s%s: %s\n", indent, relationship.label, strings.Join(relationship.ids, ", "))
		}
	}
}

func renderTopicImpact(out io.Writer, t project.TopicImpact) {
	fmt.Fprintf(out, "%s - %s\n  Summary: %s\n", t.ID, t.Title, t.Summary)
	fmt.Fprintf(out, "  Authority counts: invariants=%d, rules=%d\n", t.Counts.Invariants, t.Counts.Rules)
	if t.Selectors != nil {
		domain := strings.Join(t.Selectors.DomainPaths, " ")
		topicPaths := strings.Join(t.Selectors.TopicPaths, " ")
		if t.Selectors.DeclaredGlobal {
			topicPaths = "global"
		}
		fmt.Fprintf(out, "  Selectors: domain=[%s]; topic=%s; both must match\n", domain, func() string {
			if topicPaths == "global" {
				return topicPaths
			}
			return "[" + topicPaths + "]"
		}())
	}
	renderClaimCategory(out, "Directly related", t.Direct)
	renderClaimCategory(out, "Applicable invariants", t.Invariants)
	renderClaimCategory(out, "Additional topic rules", t.Additional)
	renderClaimCategory(out, "Referenced context", t.Referenced)
	if len(t.Pending.Operations) > 0 {
		for _, p := range t.Pending.Operations {
			fmt.Fprintf(out, "  Pending operation: ADR-%s %s %s [%s]\n", p.ADR, p.Op, p.Claim, p.Progress)
		}
	} else if t.Pending.OperationCount > 0 {
		noun := "operations"
		if t.Pending.OperationCount == 1 {
			noun = "operation"
		}
		ids := []string{}
		for _, id := range t.Pending.ADRs {
			ids = append(ids, "ADR-"+id)
		}
		suffix := ""
		if t.Pending.AdditionalADRCount > 0 {
			suffix = fmt.Sprintf(" +%d ADRs", t.Pending.AdditionalADRCount)
		}
		fmt.Fprintf(out, "  Pending: %d %s from %s%s\n", t.Pending.OperationCount, noun, strings.Join(ids, ", "), suffix)
	}
}
func renderClaimCategory(out io.Writer, label string, claims []project.ContextClaimImpact) {
	if len(claims) == 0 {
		return
	}
	fmt.Fprintf(out, "  %s:\n", label)
	for _, claim := range claims {
		fmt.Fprintf(out, "    %s [%s] %s\n", claim.ID, claim.Type, claim.Summary)
		if len(claim.Sources) > 0 {
			entries := make([]string, 0, len(claim.Sources))
			for _, source := range claim.Sources {
				entries = append(entries, fmt.Sprintf("request %d [%s]", source.RequestIndex, strings.Join(source.Kinds, ", ")))
			}
			fmt.Fprintf(out, "      Sources: %s\n", strings.Join(entries, "; "))
		}
		if claim.Backing != "" {
			fmt.Fprintf(out, "      Backing: %s\n", claim.Backing)
		}
		if claim.Verify != "" {
			fmt.Fprintf(out, "      Verify: %s\n", claim.Verify)
		}
		renderEvidence(out, &claim, claim.Evidence, "      ")
		if len(claim.Incoming) > 0 {
			fmt.Fprintf(out, "      Incoming: %s\n", strings.Join(claim.Incoming, ", "))
		}
		if len(claim.Outgoing) > 0 {
			fmt.Fprintf(out, "      Outgoing: %s\n", strings.Join(claim.Outgoing, ", "))
		}
	}
}
func renderEvidence(out io.Writer, claim *project.ContextClaimImpact, evidence []project.ContextEvidence, indent string) {
	_ = claim
	for _, e := range evidence {
		if len(e.Sites) == 0 {
			fmt.Fprintf(out, "%sEvidence %s: %d sites\n", indent, e.Kind, e.Count)
		} else {
			for _, site := range e.Sites {
				fmt.Fprintf(out, "%sEvidence %s: %s:%d\n", indent, e.Kind, site.Path, site.Line)
			}
		}
	}
}
func renderList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
func containsFacet(facets []project.ContextFacet, want project.ContextFacet) bool {
	for _, f := range facets {
		if f == want {
			return true
		}
	}
	return false
}
