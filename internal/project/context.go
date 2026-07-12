package project

import (
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/plan"
)

// ContextResult is the read-only context awf holds for a set of repo-relative
// paths: their owning domains (each with the rendered current-state pointer),
// the invariant slugs backed under those paths, the ADRs related via the owning
// domains, and any queried path matching no configured domain.
type ContextResult struct {
	Paths      []string     `json:"paths"`
	Domains    []DomainRef  `json:"domains"`
	Invariants []string     `json:"invariants"`
	ADRs       []ADRRef     `json:"adrs"`
	Plans      []PlanRef    `json:"plans"`
	Pitfalls   []PitfallRef `json:"pitfalls"`
	Unowned    []string     `json:"unowned"`
}

// PitfallRef is a pitfall surfaced because one of its domains: owns a queried
// path. Path is the docsDir-rooted pitfalls doc; Domains are the entry's own tags.
type PitfallRef struct {
	Title   string   `json:"title"`
	Domains []string `json:"domains"`
	Path    string   `json:"path"`
}

// PlanRef is a plan surfaced because its adrs: links an ADR reported for the
// query. Path is docsDir-rooted; ADRs are the linked ADR numbers.
type PlanRef struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Status   string `json:"status"`
	ADRs     []int  `json:"adrs"`
}

// DomainRef is an owning domain and its rendered current-state doc path, derived
// by convention (never a sidecar field — ADR-0086).
type DomainRef struct {
	Name         string `json:"name"`
	CurrentState string `json:"currentState"`
}

// ADRRef is an ADR related to the query via an owning domain. Title is the human
// title with the "ADR-NNNN: " prefix stripped (Number carries it). Invariants
// are the inv: slugs this ADR declares (its Invariants section), the ADR-side
// half of the backing-invariants join (ADR-0092 D4).
type ADRRef struct {
	Number     string   `json:"number"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Path       string   `json:"path"`
	Invariants []string `json:"invariants"`
}

// ContextFor assembles the read-only context for paths. It reads only committed
// state (domain sidecars, ADR files, source markers) and writes nothing.
func (p *Project) ContextFor(paths []string) (ContextResult, error) {
	clean := normalizeContextPaths(paths)
	lay := p.layout()
	res := ContextResult{Paths: clean}

	owners := map[string]bool{}
	matched := map[string]bool{}
	for _, d := range p.Cfg.Domains {
		sc, err := p.Cfg.Sidecar("domains", d)
		if err != nil {
			return ContextResult{}, err
		}
		for _, g := range sc.Paths {
			for _, path := range clean {
				if pathglob.Match(g, path) {
					owners[d] = true
					matched[path] = true
				}
			}
		}
	}
	for d := range owners {
		res.Domains = append(res.Domains, DomainRef{
			Name:         d,
			CurrentState: lay.DocsDir + "/domains/" + d + ".md",
		})
	}
	sort.Slice(res.Domains, func(i, j int) bool { return res.Domains[i].Name < res.Domains[j].Name })
	for _, path := range clean {
		if !matched[path] {
			res.Unowned = append(res.Unowned, path)
		}
	}

	if p.Cfg.Invariants != nil && !p.Cfg.Invariants.Disabled {
		slugs, err := invariants.MarkersUnder(p.Root, p.Cfg.Invariants.Sources, clean)
		if err != nil {
			return ContextResult{}, err
		}
		res.Invariants = slugs
	}

	adrs, err := adr.ParseDir(p.decisionsDir())
	if err != nil {
		return ContextResult{}, err
	}
	for _, a := range adrs {
		for _, dm := range a.Domains {
			if owners[dm] {
				res.ADRs = append(res.ADRs, ADRRef{
					Number:     a.Number,
					Title:      strings.TrimPrefix(a.Title, "ADR-"+a.Number+": "),
					Status:     a.Status,
					Path:       lay.DocsDir + "/decisions/" + a.Filename,
					Invariants: invariants.DeclaredSlugs(a.Sections["Invariants"]),
				})
				break
			}
		}
	}
	sort.Slice(res.ADRs, func(i, j int) bool { return res.ADRs[i].Number < res.ADRs[j].Number })

	// Surface plans transitively: a plan whose adrs: links any ADR reported above.
	// Plans declare adrs: not paths:, so this ADR join is the only clean link.
	// invariant: context-surfaces-linked-plans
	surfaced := map[int]bool{}
	for _, a := range res.ADRs {
		if n, err := strconv.Atoi(a.Number); err == nil { // coverage-ignore: a.Number is always a 4-digit numeral from FilenameRe
			surfaced[n] = true
		}
	}
	plans, err := plan.ParseDir(filepath.Join(p.Root, p.Cfg.DocsDir, "plans"))
	if err != nil {
		return ContextResult{}, err
	}
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		for _, n := range pl.ADRs {
			if surfaced[n] {
				res.Plans = append(res.Plans, PlanRef{
					Filename: pl.Filename, Path: lay.PlansDir + "/" + pl.Filename,
					Status: pl.Status, ADRs: pl.ADRs,
				})
				break
			}
		}
	}
	sort.Slice(res.Plans, func(i, j int) bool { return res.Plans[i].Filename < res.Plans[j].Filename })

	// Surface pitfalls whose own domains: owns a queried path (like ADRs, not
	// transitively like plans). Only when the toggleable pitfalls doc is enabled.
	// invariant: context-surfaces-pitfalls
	if slices.Contains(p.Cfg.Docs, "pitfalls") {
		sc, err := p.Cfg.Sidecar("docs", "pitfalls")
		if err != nil {
			return ContextResult{}, err
		}
		entries, err := pitfallEntries(sc.Data["pitfalls"])
		if err != nil {
			return ContextResult{}, err
		}
		for _, e := range entries {
			for _, d := range e.Domains {
				if owners[d] {
					res.Pitfalls = append(res.Pitfalls, PitfallRef{
						Title: e.Title, Domains: e.Domains,
						Path: lay.DocsDir + "/pitfalls.md",
					})
					break
				}
			}
		}
		sort.Slice(res.Pitfalls, func(i, j int) bool { return res.Pitfalls[i].Title < res.Pitfalls[j].Title })
	}
	return res, nil
}

// normalizeContextPaths slash-normalizes, path-cleans, de-duplicates, and sorts
// the queried paths so the assembly is deterministic.
func normalizeContextPaths(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		c := filepath.ToSlash(filepath.Clean(p))
		if c == "" || c == "." || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
