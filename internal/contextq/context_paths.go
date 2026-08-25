package contextq

import (
	"encoding/binary"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type ContextFacet string

const (
	FacetRelationships ContextFacet = "relationships"
	FacetInvariants    ContextFacet = "invariants"
	FacetAllRules      ContextFacet = "all-rules"
	FacetEvidence      ContextFacet = "evidence"
	FacetSelectors     ContextFacet = "selectors"
	FacetReferences    ContextFacet = "references"
	FacetPending       ContextFacet = "pending"
	FacetArtifacts     ContextFacet = "artifacts"
)

var allContextFacets = []ContextFacet{FacetRelationships, FacetInvariants, FacetAllRules, FacetEvidence, FacetSelectors, FacetReferences, FacetPending, FacetArtifacts}

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

type requestStatus string

const (
	requestLiteral           requestStatus = "literal"
	requestDirectoryExpanded requestStatus = "directory"
	requestDirectoryEmpty    requestStatus = "directory"
	requestGitSelected       requestStatus = "git-selected"
)

type pathClassification string

const (
	pathCovered           pathClassification = "covered"
	pathEligibleUnowned   pathClassification = "eligible-unowned"
	pathContextIgnored    pathClassification = "context-ignored"
	pathGeneratedOutput   pathClassification = "generated-output"
	pathNestedAdopter     pathClassification = "nested-adopter"
	pathSymlink           pathClassification = "symlink"
	pathNotFound          pathClassification = "not-found"
	pathOutsideRepository pathClassification = "outside-repository"
)

// ContextResult is the assembled context report. Its request and topic rows are
// projection detail the command binary renders through this package rather than
// reads, so only the report itself and its selection are exported vocabulary.
type ContextResult struct {
	Selection ContextSelection
	Range     string
	Requests  []contextRequestReport
	Topics    []topicImpact
}

type contextRequestReport struct {
	Index            int
	Argument, Lookup string
	Kind             requestStatus
	Exact            *contextExactEntry
	Directory        *contextDirectory
}

type contextExactEntry struct {
	Path    string
	Context contextPathImpact
}

type contextDirectory struct {
	Included      int
	Excluded      []contextClassificationCount
	Groups        []contextGroup
	Relationships contextRelationships
}

type contextClassificationCount struct {
	Classification pathClassification
	Count          int
}

type contextGroup struct {
	Count   int
	Members []string
	Context contextPathImpact
}

type contextRelationships struct {
	State   []string
	Touches []string
	Proofs  []string
}

type contextPathImpact struct {
	Classification         pathClassification
	NestedRoot             string
	TargetInsideRepository *bool
	Provenance             []contextProvenance
	Domains                []domainRef
	Topics                 []contextPathTopic
	Relationships          contextRelationships
	Warnings               []contextWarning
	ADR                    *adrArtifactContext
}

type contextProvenance struct {
	Role, Identity               string
	Sources, Outputs, Navigation []artifactLink
}

type contextPathTopic struct {
	ID string
}

type contextWarning string

const (
	warningGlobLiteral     contextWarning = "globs are not expanded; pass a directory or an exact file"
	warningEligibleUnowned contextWarning = "no domain owns this path; add a domain selector"
)

type contextAuthorityCounts struct {
	Invariants int
	Rules      int
}

type topicImpact struct {
	ID, Title, Summary                         string
	Counts                                     contextAuthorityCounts
	Direct, Invariants, Additional, Referenced []contextClaimImpact
	Pending                                    contextPendingImpact
	Selectors                                  *contextSelectorImpact
}

type contextRelationshipSource struct {
	RequestIndex int
	Kinds        []string
}

type contextClaimImpact struct {
	ID, Type, Summary, Backing, Verify string
	Sources                            []contextRelationshipSource
	Evidence                           []contextEvidence
	Incoming, Outgoing                 []string
}

type contextEvidence struct {
	Kind  string
	Count int
	Sites []topic.MarkerSite
}

type contextPendingImpact struct {
	OperationCount     int
	ADRs               []string
	AdditionalADRCount int
	Operations         []pendingChange
}

type contextSelectorImpact struct {
	DomainPaths, TopicPaths []string
	DeclaredGlobal          bool
}

type contextPathSet struct {
	tree        *snapshot.Tree
	nested      []string
	outputs     map[string]bool
	ignores     []string
	domainPaths map[string][]string
	impacts     map[string]contextPathImpact
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

// safelyMatchablePaths returns every scannable snapshot path: the universe a
// selector may be matched against. This deliberate private copy uses the
// core's identical three-line snapshot-tree filter. Exporting that filter
// solely for reuse would widen the seam this package exists to narrow
// (ADR-0195).
func safelyMatchablePaths(tree *snapshot.Tree) []string {
	out := []string{}
	for _, f := range tree.List() {
		if f.Scannable() {
			out = append(out, f.Path)
		}
	}
	return out
}

func buildContextRequests(queries []string, set contextPathSet, options ContextOptions) []contextRequestReport {
	requests := []contextRequestReport{}
	files := set.tree.List()
	for _, raw := range queries {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		lookup := filepath.ToSlash(filepath.Clean(raw))
		report := contextRequestReport{Index: len(requests) + 1, Argument: raw, Lookup: lookup, Kind: requestLiteral}
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
			report.Kind = requestDirectoryExpanded
			dir := contextDirectory{Excluded: []contextClassificationCount{}, Groups: []contextGroup{}, Relationships: emptyContextRelationships()}
			counts := map[pathClassification]int{}
			groups := map[string]*contextGroup{}
			for _, f := range files {
				if !strings.HasPrefix(f.Path, prefix) {
					continue
				}
				// Nested roots and symlinks are census boundaries and count once.
				boundary := false
				for _, root := range set.nested {
					if f.Path == root+"/.awf/config.yaml" || strings.HasPrefix(f.Path, root+"/") {
						if f.Path == root+"/.awf/config.yaml" {
							counts[pathNestedAdopter]++
						}
						boundary = true
						break
					}
				}
				if boundary {
					continue
				}
				impact := set.impacts[f.Path]
				if impact.Classification == pathSymlink {
					counts[pathSymlink]++
					continue
				}
				if impact.Classification != pathCovered && impact.Classification != pathEligibleUnowned {
					counts[impact.Classification]++
					continue
				}
				dir.Included++
				dir.Relationships = unionContextRelationships(dir.Relationships, impact.Relationships)
				key := contextGroupKey(impact, options.Facets)
				g := groups[key]
				if g == nil {
					g = &contextGroup{Members: []string{}, Context: impact}
					groups[key] = g
				}
				g.Count++
				g.Members = append(g.Members, f.Path)
			}
			for _, class := range []pathClassification{pathContextIgnored, pathGeneratedOutput, pathNestedAdopter, pathSymlink, pathNotFound, pathOutsideRepository} {
				if counts[class] > 0 {
					dir.Excluded = append(dir.Excluded, contextClassificationCount{class, counts[class]})
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
				report.Kind = requestDirectoryEmpty
			}
			report.Directory = &dir
		} else {
			if options.Selection != SelectionExplicit {
				report.Kind = requestGitSelected
			}
			impact := set.impacts[lookup]
			report.Exact = &contextExactEntry{Path: lookup, Context: impact}
		}
		requests = append(requests, report)
	}
	return requests
}

func emptyContextRelationships() contextRelationships {
	return contextRelationships{State: []string{}, Touches: []string{}, Proofs: []string{}}
}

func contextRelationshipsForPath(sitesByPath map[string][]topic.MarkerSite, filePath string) contextRelationships {
	relationships := emptyContextRelationships()
	for _, site := range sitesByPath[filePath] {
		switch site.Kind {
		case topic.StateMarker:
			relationships.State = append(relationships.State, site.ClaimID)
		case topic.TouchesMarker:
			relationships.Touches = append(relationships.Touches, site.ClaimID)
		case topic.ProofMarker:
			relationships.Proofs = append(relationships.Proofs, site.ClaimID)
		}
	}
	return compactContextRelationships(relationships)
}

func unionContextRelationships(inputs ...contextRelationships) contextRelationships {
	out := emptyContextRelationships()
	for _, relationships := range inputs {
		out.State = append(out.State, relationships.State...)
		out.Touches = append(out.Touches, relationships.Touches...)
		out.Proofs = append(out.Proofs, relationships.Proofs...)
	}
	return compactContextRelationships(out)
}

func compactContextRelationships(relationships contextRelationships) contextRelationships {
	slices.Sort(relationships.State)
	relationships.State = slices.Compact(relationships.State)
	slices.Sort(relationships.Touches)
	relationships.Touches = slices.Compact(relationships.Touches)
	slices.Sort(relationships.Proofs)
	relationships.Proofs = slices.Compact(relationships.Proofs)
	return relationships
}

func contextGroupKey(impact contextPathImpact, facets []ContextFacet) string {
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
		if slices.Contains(facets, FacetArtifacts) {
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
	}
	for _, d := range impact.Domains {
		add(d.Name)
		add(d.CurrentState)
	}
	for _, t := range impact.Topics {
		add(t.ID)
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
		}
	}
	return b.String()
}

// outsideContextPath reads an already-slash-normalized path, so it asks both
// spaces whether the input is absolute. path.IsAbs alone misses a drive-rooted
// Windows input like "C:/x"; filepath.IsAbs alone answers false on Windows for
// a slash-rooted input like "/etc/passwd", which then fell through to
// pathNotFound instead of pathOutsideRepository. On Unix the two agree.
func outsideContextPath(p string) bool {
	return path.IsAbs(p) || filepath.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../")
}
func globLiteralQuery(p string) bool { return strings.ContainsAny(p, "*?[") }

func classifyContextPath(p string, set contextPathSet) (pathClassification, string, *bool) {
	if outsideContextPath(p) {
		return pathOutsideRepository, "", nil
	}
	for _, root := range set.nested {
		if p == root || strings.HasPrefix(p, root+"/") {
			return pathNestedAdopter, root + "/.awf/config.yaml", nil
		}
	}
	if set.outputs[p] {
		return pathGeneratedOutput, "", nil
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
		return pathSymlink, "", &inside
	}
	if pathglob.MatchAny(set.ignores, p) {
		return pathContextIgnored, "", nil
	}
	if _, ok := set.tree.Lookup(p); !ok {
		return pathNotFound, "", nil
	}
	for _, globs := range set.domainPaths {
		if pathglob.MatchAny(globs, p) {
			return pathCovered, "", nil
		}
	}
	return pathEligibleUnowned, "", nil
}
