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
	Paths      []string       `json:"paths"`
	Domains    []DomainRef    `json:"domains"`
	Invariants []InvariantRef `json:"invariants"`
	Governing  []ADRRef       `json:"governing"`  // Tier 1: invariants backed under the query
	Related    []ADRRef       `json:"related"`    // Tier 2: precise-tag or related: linked
	Pitfalls   []PitfallRef   `json:"pitfalls"`   // Tier 2: precise-tag match
	Plans      []PlanRef      `json:"plans"`      // linked to a Tier-1/Tier-2 ADR
	Background int            `json:"background"` // Tier 3: collapsed domain-ADR count
	Unowned    []string       `json:"unowned"`
}

// InvariantRef is an invariant slug surfaced as present under a queried path
// (ADR-0106). Class labels a governing (declared) invariant backed or unbacked;
// it is empty for a slug present under the path but declared by no Implemented
// ADR. Verify carries an unbacked governing invariant's `Verify:` guidance;
// Touches carries the site notes from any `touches-invariant:` marker under the
// query. Both renderings derive from this one value (context-output-parity).
type InvariantRef struct {
	Slug    string   `json:"slug"`
	Class   string   `json:"class,omitempty"`
	Verify  string   `json:"verify,omitempty"`
	Touches []string `json:"touches,omitempty"`
}

// PitfallRef is a pitfall surfaced because it shares a precise tag with the
// query (ADR-0104 Tier 2). Path is the docsDir-rooted pitfalls doc; Tags are the
// entry's own tags.
type PitfallRef struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
	Path  string   `json:"path"`
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

// ADRRef is an ADR surfaced in a context tier. Title is the human title with the
// "ADR-NNNN: " prefix stripped (Number carries it). The per-ADR declared-slug
// echo was dropped by ADR-0104 (the flat ## Invariants block carries the
// path-present slugs; the per-ADR list only duplicated it).
type ADRRef struct {
	Number string `json:"number"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Path   string `json:"path"`
}

// ContextFor assembles the read-only context for paths. It reads only committed
// state (domain sidecars, ADR files, source markers) and writes nothing.
func (p *Project) ContextFor(paths []string) (ContextResult, error) {
	clean := NormalizeContextPaths(paths)
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

	var hits []invariants.MarkerHit
	if p.Cfg.Invariants != nil && !p.Cfg.Invariants.Disabled {
		h, err := invariants.MarkersUnder(p.Root, p.Cfg.Invariants, clean)
		if err != nil {
			return ContextResult{}, err
		}
		hits = h
	}

	adrs, err := adr.ParseDir(p.decisionsDir())
	if err != nil {
		return ContextResult{}, err
	}

	// Tier 1 — "governs this code": ADRs declaring an invariant slug present as a
	// marker under a queried path (one-to-one slug -> declaring Implemented ADR).
	// Each surfaced slug is labelled backed/unbacked from its declaring ADR's
	// ADR-0105 class, with an unbacked invariant's `Verify:` guidance and any
	// touches-marker site note carried on the InvariantRef (ADR-0106).
	// invariant: context-tier1-governs
	declaring, err := invariants.DeclaringADRs(adrs)
	if err != nil {
		return ContextResult{}, err
	}
	byFile := map[string]adr.ADR{}
	for _, a := range adrs {
		byFile[a.Filename] = a
	}
	tier1 := map[string]bool{}
	var t1 []adr.ADR
	for _, h := range hits {
		ref := InvariantRef{Slug: h.Slug, Touches: h.Notes}
		if decl, ok := declaring[h.Slug]; ok {
			ref.Class = string(decl.Class)
			if decl.Class == invariants.ClassUnbacked {
				ref.Verify = decl.Verify
			}
			a := byFile[decl.ADR]
			if !tier1[a.Number] {
				tier1[a.Number] = true
				t1 = append(t1, a)
				res.Governing = append(res.Governing, adrRefOf(a, lay))
			}
		}
		res.Invariants = append(res.Invariants, ref)
	}
	sort.Slice(res.Governing, func(i, j int) bool { return res.Governing[i].Number < res.Governing[j].Number })

	// Precise tag set: union of Tier-1 tags minus any tag naming a configured
	// domain (a domain-mirror tag is Tier-3 relatedness, not Tier-2 precision).
	domainName := map[string]bool{}
	for _, d := range p.Cfg.Domains {
		domainName[d] = true
	}
	precise := map[string]bool{}
	relatedNum := map[int]bool{}
	for _, a := range t1 {
		for _, tag := range a.Tags {
			if !domainName[tag] {
				precise[tag] = true
			}
		}
		for _, n := range a.Related {
			relatedNum[n] = true
		}
	}

	// Tier 2 — "topically related": non-Tier-1, non-Superseded ADRs sharing a
	// precise tag or named in a Tier-1 ADR's related:.
	// invariant: context-tier2-topical
	inTier2 := map[string]bool{}
	for _, a := range adrs {
		if tier1[a.Number] || strings.HasPrefix(a.Status, "Superseded") {
			continue
		}
		n, _ := strconv.Atoi(a.Number)
		if sharesTag(a.Tags, precise) || relatedNum[n] {
			inTier2[a.Number] = true
			res.Related = append(res.Related, adrRefOf(a, lay))
		}
	}
	sort.Slice(res.Related, func(i, j int) bool { return res.Related[i].Number < res.Related[j].Number })

	// Tier 2 pitfalls: share a precise tag (only when the pitfalls doc is enabled).
	// invariant: context-surfaces-tiered-pitfalls
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
			if sharesTag(e.Tags, precise) {
				res.Pitfalls = append(res.Pitfalls, PitfallRef{Title: e.Title, Tags: e.Tags, Path: lay.DocsDir + "/pitfalls.md"})
			}
		}
		sort.Slice(res.Pitfalls, func(i, j int) bool { return res.Pitfalls[i].Title < res.Pitfalls[j].Title })
	}

	// Tier 3 — "domain background": domain-membership ADRs in neither Tier 1 nor
	// Tier 2, reported only as a collapsed count.
	// invariant: context-tier3-collapsed
	for _, a := range adrs {
		if tier1[a.Number] || inTier2[a.Number] {
			continue
		}
		for _, dm := range a.Domains {
			if owners[dm] {
				res.Background++
				break
			}
		}
	}

	// Plans linked to a Tier-1 or Tier-2 ADR.
	// invariant: context-surfaces-tiered-plans
	surfaced := map[int]bool{}
	for _, a := range append(append([]ADRRef{}, res.Governing...), res.Related...) {
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
	return res, nil
}

// adrRefOf projects an ADR to its context reference (Title prefix stripped).
func adrRefOf(a adr.ADR, lay Layout) ADRRef {
	return ADRRef{
		Number: a.Number,
		Title:  strings.TrimPrefix(a.Title, "ADR-"+a.Number+": "),
		Status: a.Status,
		Path:   lay.DocsDir + "/decisions/" + a.Filename,
	}
}

// sharesTag reports whether any of tags is in set.
func sharesTag(tags []string, set map[string]bool) bool {
	for _, t := range tags {
		if set[t] {
			return true
		}
	}
	return false
}

// UncoveredResult is the read-only domain-coverage report for a set of scan roots:
// the git-tracked paths matched by no configured domain glob, with a fully-uncovered
// directory collapsed to its topmost node (a trailing-slash entry). ScanRoots echoes
// the requested roots (empty = whole repository).
type UncoveredResult struct {
	ScanRoots []string `json:"scanRoots"`
	Entries   []string `json:"entries"`
}

// Uncovered assembles the domain-coverage report over the tracked paths. It writes
// nothing and reads only the domain sidecars. scanRoots restrict the report to
// tracked paths at or beneath them, matched on slash-separated segment boundaries
// (a directory subtree), not raw string prefixes; empty scanRoots scans everything.
// touches-invariant: uncovered-lists-unowned-only — unowned-path reporting; proof in context_test.go
// touches-invariant: uncovered-collapses-directories — fully-uncovered directory collapse; proof in context_test.go
func (p *Project) Uncovered(tracked, scanRoots []string) (UncoveredResult, error) {
	roots := NormalizeContextPaths(scanRoots)
	res := UncoveredResult{ScanRoots: roots}

	// Domain glob set, once.
	var globs []string
	for _, d := range p.Cfg.Domains {
		sc, err := p.Cfg.Sidecar("domains", d)
		if err != nil {
			return UncoveredResult{}, err
		}
		globs = append(globs, sc.Paths...)
	}

	inScope := func(path string) bool {
		if len(roots) == 0 {
			return true
		}
		for _, r := range roots {
			if path == r || strings.HasPrefix(path, r+"/") {
				return true
			}
		}
		return false
	}
	covered := func(path string) bool {
		for _, g := range globs {
			if pathglob.Match(g, path) {
				return true
			}
		}
		return false
	}

	// coveredDirs: every ancestor directory (including "." root) of an in-scope
	// covered tracked path, plus each scan root's strict ancestors so a collapse
	// never climbs above a requested root. A directory absent here has no covered
	// descendant within scope.
	coveredDirs := map[string]bool{}
	for _, r := range roots {
		for _, a := range ancestors(r) {
			coveredDirs[a] = true
		}
	}
	var uncovered []string
	for _, path := range tracked {
		clean := filepath.ToSlash(filepath.Clean(path))
		if !inScope(clean) {
			continue
		}
		if covered(clean) {
			for _, a := range ancestors(clean) {
				coveredDirs[a] = true
			}
			continue
		}
		uncovered = append(uncovered, clean)
	}

	// Collapse each uncovered path to its topmost ancestor that has no covered
	// descendant; a path all of whose ancestors are covered-adjacent reports itself.
	entries := map[string]bool{}
	for _, u := range uncovered {
		pick := u
		for _, a := range ancestors(u) {
			if !coveredDirs[a] {
				if a == "." {
					pick = "."
				} else {
					pick = a + "/"
				}
				break
			}
		}
		entries[pick] = true
	}
	for e := range entries {
		res.Entries = append(res.Entries, e)
	}
	sort.Strings(res.Entries)
	return res, nil
}

// ancestors returns path's directory ancestors from the top down — "." then each
// strict directory prefix — excluding path itself.
func ancestors(path string) []string {
	out := []string{"."}
	segs := strings.Split(path, "/")
	for i := 1; i < len(segs); i++ {
		out = append(out, strings.Join(segs[:i], "/"))
	}
	return out
}

// NormalizeContextPaths slash-normalizes, path-cleans, de-duplicates, and sorts
// the queried paths so the assembly is deterministic.
func NormalizeContextPaths(paths []string) []string {
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
