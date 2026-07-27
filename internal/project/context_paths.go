package project

import (
	"encoding/binary"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type ContextFacet string

const (
	FacetAllRules   ContextFacet = "all-rules"
	FacetEvidence   ContextFacet = "evidence"
	FacetSelectors  ContextFacet = "selectors"
	FacetReferences ContextFacet = "references"
	FacetPending    ContextFacet = "pending"
	FacetArtifacts  ContextFacet = "artifacts"
)

var allContextFacets = []ContextFacet{FacetAllRules, FacetEvidence, FacetSelectors, FacetReferences, FacetPending, FacetArtifacts}

type ContextSelection string

const (
	SelectionExplicit ContextSelection = "explicit"
	SelectionStaged   ContextSelection = "staged"
	SelectionRange    ContextSelection = "range"
)

type ContextOptions struct {
	Selection ContextSelection
	Range     string
	Facets    []ContextFacet
}

type RequestStatus string

const (
	RequestLiteral           RequestStatus = "literal"
	RequestDirectoryExpanded RequestStatus = "directory"
	RequestDirectoryEmpty    RequestStatus = "directory"
	RequestGitSelected       RequestStatus = "git-selected"
)

type PathClassification string

const (
	PathCovered           PathClassification = "covered"
	PathEligibleUnowned   PathClassification = "eligible-unowned"
	PathContextIgnored    PathClassification = "context-ignored"
	PathGeneratedOutput   PathClassification = "generated-output"
	PathNestedAdopter     PathClassification = "nested-adopter"
	PathSymlink           PathClassification = "symlink"
	PathNotFound          PathClassification = "not-found"
	PathOutsideRepository PathClassification = "outside-repository"
)

type ContextResult struct {
	Selection ContextSelection
	Range     string
	Requests  []ContextRequestReport
	Topics    []TopicImpact
}

type ContextRequestReport struct {
	Index            int
	Argument, Lookup string
	Kind             RequestStatus
	Exact            *ContextExactEntry
	Directory        *ContextDirectory
}

type ContextExactEntry struct {
	Path    string
	Context ContextPathImpact
}

type ContextDirectory struct {
	Included int
	Excluded []ContextClassificationCount
	Groups   []ContextGroup
}

type ContextClassificationCount struct {
	Classification PathClassification
	Count          int
}

type ContextGroup struct {
	Count   int
	Members []string
	Context ContextPathImpact
}

type ContextPathImpact struct {
	Classification         PathClassification
	NestedRoot             string
	TargetInsideRepository *bool
	Provenance             []ContextProvenance
	Domains                []DomainRef
	Topics                 []ContextPathTopic
	DirectRuleIDs          []string
	InvariantIDs           []string
	ProofIDs               []string
	Warnings               []ContextWarning
	ADR                    *ADRArtifactContext
}

type ContextProvenance struct {
	Role, Identity               string
	Sources, Outputs, Navigation []ArtifactLink
}

type ContextPathTopic struct {
	ID             string
	DirectClaimIDs []string
}

type ContextWarning string

const (
	WarningGlobLiteral     ContextWarning = "globs are not expanded; pass a directory or an exact file"
	WarningEligibleUnowned ContextWarning = "no domain owns this path; add a domain selector"
)

type TopicImpact struct {
	ID, Title, Summary                         string
	Direct, Invariants, Additional, Referenced []ContextClaimImpact
	Pending                                    ContextPendingImpact
	Selectors                                  *ContextSelectorImpact
}

type ContextClaimImpact struct {
	ID, Type, Summary, Backing, Verify string
	Evidence                           []ContextEvidence
	Incoming, Outgoing                 []string
}

type ContextEvidence struct {
	Kind  string
	Count int
	Sites []topic.MarkerSite
}

type ContextPendingImpact struct {
	OperationCount     int
	ADRs               []string
	AdditionalADRCount int
	Operations         []PendingChange
}

type ContextSelectorImpact struct {
	DomainPaths, TopicPaths []string
	DeclaredGlobal          bool
}

type contextPathSet struct {
	tree        *snapshot.Tree
	eligible    []string
	nested      []string
	outputs     map[string]bool
	ignores     []string
	domainPaths map[string][]string
	impacts     map[string]ContextPathImpact
}

func ParseContextFacets(values []string, full bool) ([]ContextFacet, error) {
	selected := map[ContextFacet]bool{}
	if full {
		for _, facet := range allContextFacets {
			selected[facet] = true
		}
	}
	for _, value := range values {
		facet := ContextFacet(value)
		if !slices.Contains(allContextFacets, facet) {
			return nil, &ContextFacetError{Value: value}
		}
		selected[facet] = true
	}
	out := []ContextFacet{}
	for _, facet := range allContextFacets {
		if selected[facet] {
			out = append(out, facet)
		}
	}
	return out, nil
}

type ContextFacetError struct{ Value string }

func (e *ContextFacetError) Error() string { return "unknown context facet " + strconv.Quote(e.Value) }

func safelyMatchablePaths(tree *snapshot.Tree) []string {
	out := []string{}
	for _, f := range tree.List() {
		if f.Scannable() {
			out = append(out, f.Path)
		}
	}
	return out
}

func buildContextRequests(queries []string, set contextPathSet, options ContextOptions) []ContextRequestReport {
	requests := []ContextRequestReport{}
	files := set.tree.List()
	for _, raw := range queries {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		lookup := filepath.ToSlash(filepath.Clean(raw))
		report := ContextRequestReport{Index: len(requests) + 1, Argument: raw, Lookup: lookup, Kind: RequestLiteral}
		directory := lookup == "."
		prefix := lookup + "/"
		if lookup == "." {
			prefix = ""
		}
		if _, ok := set.tree.Lookup(lookup); !ok && !outsideContextPath(lookup) {
			for _, f := range files {
				if strings.HasPrefix(f.Path, prefix) {
					directory = true
					break
				}
			}
		}
		if directory {
			report.Kind = RequestDirectoryExpanded
			dir := ContextDirectory{Excluded: []ContextClassificationCount{}, Groups: []ContextGroup{}}
			counts := map[PathClassification]int{}
			groups := map[string]*ContextGroup{}
			for _, f := range files {
				if !strings.HasPrefix(f.Path, prefix) {
					continue
				}
				// Nested roots and symlinks are census boundaries and count once.
				boundary := false
				for _, root := range set.nested {
					if f.Path == root+"/.awf/config.yaml" || strings.HasPrefix(f.Path, root+"/") {
						if f.Path == root+"/.awf/config.yaml" {
							counts[PathNestedAdopter]++
						}
						boundary = true
						break
					}
				}
				if boundary {
					continue
				}
				impact := set.impacts[f.Path]
				if impact.Classification == PathSymlink {
					counts[PathSymlink]++
					continue
				}
				if impact.Classification != PathCovered && impact.Classification != PathEligibleUnowned {
					counts[impact.Classification]++
					continue
				}
				dir.Included++
				key := contextGroupKey(impact)
				g := groups[key]
				if g == nil {
					g = &ContextGroup{Members: []string{}, Context: impact}
					groups[key] = g
				}
				g.Count++
				g.Members = append(g.Members, f.Path)
			}
			for _, class := range []PathClassification{PathContextIgnored, PathGeneratedOutput, PathNestedAdopter, PathSymlink, PathNotFound, PathOutsideRepository} {
				if counts[class] > 0 {
					dir.Excluded = append(dir.Excluded, ContextClassificationCount{class, counts[class]})
				}
			}
			keys := make([]string, 0, len(groups))
			for key := range groups {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, key := range keys {
				g := groups[key]
				slices.Sort(g.Members)
				if g.Count > 3 {
					g.Members = []string{}
				}
				dir.Groups = append(dir.Groups, *g)
			}
			if dir.Included == 0 {
				report.Kind = RequestDirectoryEmpty
			}
			report.Directory = &dir
		} else {
			if options.Selection != SelectionExplicit {
				report.Kind = RequestGitSelected
			}
			impact := set.impacts[lookup]
			report.Exact = &ContextExactEntry{Path: lookup, Context: impact}
		}
		requests = append(requests, report)
	}
	return requests
}

func contextGroupKey(impact ContextPathImpact) string {
	var b strings.Builder
	add := func(s string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		b.Write(n[:])
		b.WriteString(s)
	}
	add(string(impact.Classification))
	add(impact.NestedRoot)
	if impact.TargetInsideRepository == nil {
		add("nil")
	} else {
		add(strconv.FormatBool(*impact.TargetInsideRepository))
	}
	for _, p := range impact.Provenance {
		add(p.Role)
		add(p.Identity)
		for _, e := range p.Sources {
			add("s")
			add(e.Path)
			add(e.Label)
		}
		for _, e := range p.Outputs {
			add("o")
			add(e.Path)
			add(e.Label)
		}
		for _, e := range p.Navigation {
			add("n")
			add(e.Path)
			add(e.Label)
		}
	}
	for _, d := range impact.Domains {
		add(d.Name)
		add(d.CurrentState)
	}
	for _, t := range impact.Topics {
		add(t.ID)
		for _, id := range t.DirectClaimIDs {
			add(id)
		}
	}
	for _, ids := range [][]string{impact.DirectRuleIDs, impact.InvariantIDs, impact.ProofIDs} {
		for _, id := range ids {
			add(id)
		}
	}
	for _, w := range impact.Warnings {
		add(string(w))
	}
	if impact.ADR != nil {
		add(impact.ADR.Number)
		add(impact.ADR.Status)
		for _, op := range impact.ADR.Operations {
			add(op.Operation)
			add(op.Claim)
			add(op.Progress)
			add(op.ClaimState)
			add(strconv.Itoa(op.StateSequence))
		}
	}
	return b.String()
}

func outsideContextPath(p string) bool {
	return filepath.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../")
}
func globLiteralQuery(p string) bool { return strings.ContainsAny(p, "*?[") }

func classifyContextPath(p string, set contextPathSet) (PathClassification, string, *bool) {
	if outsideContextPath(p) {
		return PathOutsideRepository, "", nil
	}
	for _, root := range set.nested {
		if p == root || strings.HasPrefix(p, root+"/") {
			return PathNestedAdopter, root + "/.awf/config.yaml", nil
		}
	}
	if set.outputs[p] {
		return PathGeneratedOutput, "", nil
	}
	if f, ok := set.tree.Lookup(p); ok && f.Mode == snapshot.Symlink {
		target := string(f.Bytes)
		inside := true
		if path.IsAbs(target) {
			inside = false
		} else {
			joined := path.Clean(path.Join(path.Dir(p), target))
			inside = joined != ".." && !strings.HasPrefix(joined, "../")
		}
		return PathSymlink, "", &inside
	}
	if pathMatchesAny(set.ignores, p) {
		return PathContextIgnored, "", nil
	}
	if _, ok := set.tree.Lookup(p); !ok {
		return PathNotFound, "", nil
	}
	for _, globs := range set.domainPaths {
		if pathMatchesAny(globs, p) {
			return PathCovered, "", nil
		}
	}
	return PathEligibleUnowned, "", nil
}
