package project

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// ContextResult is the read-only context awf holds for a set of repo-relative
// paths: their owning domains (each with the rendered current-state pointer),
// the invariant slugs backed under those paths, the ADRs related via the owning
// domains, and any queried path matching no configured domain.
type ContextResult struct {
	Paths      []string    `json:"paths"`
	Domains    []DomainRef `json:"domains"`
	Invariants []string    `json:"invariants"`
	ADRs       []ADRRef    `json:"adrs"`
	Unowned    []string    `json:"unowned"`
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
// invariant: context-read-only
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
