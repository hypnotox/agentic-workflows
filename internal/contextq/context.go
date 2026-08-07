// Package contextq answers context and coverage questions over one assembled
// context state: path classification, request assembly, universe assembly,
// topic and claim and pending projection, artifact records, the context and
// uncovered result vocabulary, and the human rendering of those results
// (ADR-0195).
package contextq

import (
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// domainRef is an owning domain and its rendered current-state doc path.
type domainRef struct{ Name, CurrentState string }

type pendingChange struct {
	ADR, Title, Status string
	Applied, Declared  int
	Op, Claim          string
	Progress           string
}

// Query answers context questions over one assembled context state. It is the
// package's only construction path: every entry point is a method, so a query
// can never run against a partially-assembled universe.
type Query struct{ state project.ContextState }

// New binds a query to one assembled context state. The state is produced by
// one of the core's two constructors - Project.ContextState for the working
// tree, project.StagedContextState for the index - and is read, never written.
func New(state project.ContextState) *Query { return &Query{state: state} }

// ContextForOptions assembles the full context report for the queried paths.
// It writes nothing and cannot fail: every fallible step already happened while
// the state was loaded.
func (q *Query) ContextForOptions(queries []string, options ContextOptions) ContextResult {
	state := q.state
	if options.Selection == "" {
		options.Selection = SelectionExplicit
	}
	outputs := map[string]bool{}
	for _, d := range state.Declarations {
		outputs[d.Path] = true
	}
	nested := []string{}
	for _, f := range state.Tree.List() {
		if !resident.IsResidentPath(f.Path) && f.Scannable() && strings.HasSuffix(f.Path, "/"+config.DirName+"/config.yaml") {
			nested = append(nested, strings.TrimSuffix(f.Path, "/"+config.DirName+"/config.yaml"))
		}
	}
	slices.Sort(nested)
	set := contextPathSet{tree: state.Tree, nested: nested, outputs: outputs, ignores: state.Cfg.ContextIgnore, domainPaths: state.Loaded.Topics.DomainPaths, impacts: map[string]contextPathImpact{}}
	selectedADRs := state.Loaded.Corpus
	lay := state.Layout
	markerSitesByPath := map[string][]topic.MarkerSite{}
	for _, site := range state.Loaded.Topics.Markers.All() {
		markerSitesByPath[site.Path] = append(markerSitesByPath[site.Path], site)
	}
	makeImpact := func(filePath string, explicit bool) contextPathImpact {
		class, nestedRoot, targetInside := classifyContextPath(filePath, set)
		records := artifactRecords(filePath, state.Declarations, artifactAuthorities{Layout: lay, ADRs: selectedADRs})
		applyArtifactSnapshots(records, filePath, state.Tree, state.Lock)
		impact := contextPathImpact{Classification: class, NestedRoot: nestedRoot, TargetInsideRepository: targetInside, Provenance: []contextProvenance{}, Domains: []domainRef{}, Topics: []contextPathTopic{}, Relationships: emptyContextRelationships(), Warnings: []contextWarning{}}
		for _, record := range records {
			impact.Provenance = append(impact.Provenance, contextProvenance{Role: string(record.Role), Identity: record.Identity, Sources: cloneArtifactLinks(record.Sources), Outputs: cloneArtifactLinks(record.Outputs), Navigation: cloneArtifactLinks(record.Navigation)})
		}
		literalGlob := explicit && (class == pathNotFound || class == pathContextIgnored) && globLiteralQuery(filePath)
		safe := class != pathOutsideRepository && class != pathNestedAdopter && class != pathSymlink
		if safe && !literalGlob {
			impact.Relationships = contextRelationshipsForPath(markerSitesByPath, filePath)
			for _, d := range slices.Sorted(maps.Keys(state.Loaded.Topics.DomainPaths)) {
				if pathglob.MatchAny(state.Loaded.Topics.DomainPaths[d], filePath) {
					impact.Domains = append(impact.Domains, domainRef{Name: d, CurrentState: lay.DocsDir + "/domains/" + d + ".md"})
				}
			}
			for _, t := range topic.TopicsForPath(state.Loaded.Topics, filePath) {
				impact.Topics = append(impact.Topics, contextPathTopic{ID: t.ID.String()})
			}
		}
		if literalGlob {
			impact.Warnings = append(impact.Warnings, warningGlobLiteral)
		}
		if class == pathEligibleUnowned {
			impact.Warnings = append(impact.Warnings, warningEligibleUnowned)
		}
		if explicit {
			impact.ADR = projectADRArtifact(filePath, lay.ADRDir, selectedADRs, state.Loaded.Topics, options.Facets)
		}
		return impact
	}
	for _, f := range state.Tree.List() {
		set.impacts[f.Path] = makeImpact(f.Path, false)
	}
	for _, raw := range queries {
		if strings.TrimSpace(raw) != "" {
			lookup := filepath.ToSlash(filepath.Clean(raw))
			set.impacts[lookup] = makeImpact(lookup, options.Selection == SelectionExplicit)
		}
	}
	requests := buildContextRequests(queries, set, options)
	directSources := map[string]map[int]map[string]bool{}
	applicable := map[string]topic.Topic{}
	for _, request := range requests {
		impacts := []contextPathImpact{}
		if request.Exact != nil {
			impacts = append(impacts, request.Exact.Context)
			addContextRelationshipSources(directSources, request.Index, request.Exact.Context.Relationships)
		}
		if request.Directory != nil {
			for _, group := range request.Directory.Groups {
				impacts = append(impacts, group.Context)
			}
			if slices.Contains(options.Facets, FacetRelationships) {
				addContextRelationshipSources(directSources, request.Index, request.Directory.Relationships)
			}
		}
		for _, impact := range impacts {
			for _, ref := range impact.Topics {
				if t, ok := state.Loaded.Topics.ByTopicID(ref.ID); ok {
					applicable[ref.ID] = t
				}
			}
		}
	}
	result := ContextResult{Selection: options.Selection, Range: options.Range, Requests: requests, Topics: []topicImpact{}}
	currentPaths := safelyMatchablePaths(state.Tree)
	projectedSources := contextRelationshipSources(directSources)
	globallyVisible := contextVisibleClaimIDs(applicable, projectedSources, options.Facets)
	referencedSeen := map[string]bool{}
	for _, id := range slices.Sorted(maps.Keys(applicable)) {
		result.Topics = append(result.Topics, projectTopicImpact(applicable[id], state.Loaded.Topics, projectedSources, globallyVisible, referencedSeen, currentPaths, pendingChanges(state.Loaded.Corpus, map[string]bool{id: true}), options.Facets))
	}
	return result
}

func contextVisibleClaimIDs(applicable map[string]topic.Topic, directSources map[string][]contextRelationshipSource, facets []ContextFacet) map[string]bool {
	visible := map[string]bool{}
	for _, t := range applicable {
		for _, claim := range t.Claims {
			if len(directSources[claim.ID]) > 0 || claim.Type == topic.Invariant && slices.Contains(facets, FacetInvariants) || claim.Type != topic.Invariant && slices.Contains(facets, FacetAllRules) {
				visible[claim.ID] = true
			}
		}
	}
	return visible
}

func addContextRelationshipSources(dst map[string]map[int]map[string]bool, requestIndex int, relationships contextRelationships) {
	for _, kind := range []struct {
		label string
		ids   []string
	}{
		{label: "State", ids: relationships.State},
		{label: "Touches", ids: relationships.Touches},
		{label: "Proofs", ids: relationships.Proofs},
	} {
		for _, id := range kind.ids {
			byRequest := dst[id]
			if byRequest == nil {
				byRequest = map[int]map[string]bool{}
				dst[id] = byRequest
			}
			kinds := byRequest[requestIndex]
			if kinds == nil {
				kinds = map[string]bool{}
				byRequest[requestIndex] = kinds
			}
			kinds[kind.label] = true
		}
	}
}

func contextRelationshipSources(in map[string]map[int]map[string]bool) map[string][]contextRelationshipSource {
	out := map[string][]contextRelationshipSource{}
	for id, byRequest := range in {
		for _, requestIndex := range slices.Sorted(maps.Keys(byRequest)) {
			kinds := []string{}
			for _, kind := range []string{"State", "Touches", "Proofs"} {
				if byRequest[requestIndex][kind] {
					kinds = append(kinds, kind)
				}
			}
			out[id] = append(out[id], contextRelationshipSource{RequestIndex: requestIndex, Kinds: kinds})
		}
	}
	return out
}

func pendingChanges(corpus adr.Corpus, matchedTopics map[string]bool) []pendingChange {
	var out []pendingChange
	ordered := slices.Clone(corpus.All())
	// A pending V3 record answers to its slug, so every lookup and every
	// presented reference here is the identity, not the number: keying on the
	// number would resolve nothing and sort the record before 0001 (ADR-0202
	// item 10 places a pending record after every numbered one).
	sort.SliceStable(ordered, func(i, j int) bool {
		return adr.IdentityOrder(ordered[i].Identity()) < adr.IdentityOrder(ordered[j].Identity())
	})
	for _, a := range ordered {
		if !a.IsAccepted() && !a.IsImplementing() {
			continue
		}
		identity := a.Identity()
		progress, _, err := corpus.OperationProgress(identity)
		if err != nil {
			continue
		}
		declared := len(progress.Applied) + len(progress.Remaining) + len(progress.Canceled)
		for _, op := range progress.Remaining {
			if matchedTopics[topicOfClaim(op.ID)] {
				out = append(out, pendingChange{ADR: identity, Title: strings.TrimPrefix(a.Title, "ADR-"+identity+": "), Status: a.Status, Applied: len(progress.Applied), Declared: declared, Op: string(op.Verb), Claim: op.ID, Progress: "remaining"})
			}
		}
	}
	return out
}
func topicOfClaim(id string) string {
	if i := strings.Index(id, ":"); i >= 0 {
		return id[:i]
	}
	return id
}

// UncoveredResult is the read-only coverage report for a set of scan roots: the
// eligible paths owned by no domain (collapsed to a topmost trailing-slash node)
// and the domain-owned paths with no claim-bearing scoped topic (ADR-0134).
// ScanRoots echoes the requested roots (empty = whole repository).
type UncoveredResult struct {
	ScanRoots []string
	Unowned   []unownedEntry
	Uncovered []uncoveredTopic
}

// unownedEntry is one collapsed unowned node: UnownedCount is the in-scope
// eligible unowned paths it covers, ExcludedCount the in-scope scannable paths
// beneath it that coverage excludes (generated, context-ignored, resident,
// or nested-adopter). Plain file entries keep ExcludedCount zero.
type unownedEntry struct {
	Path          string
	UnownedCount  int
	ExcludedCount int
}

// uncoveredTopic is one domain-owned path lacking a scoped topic that covers it.
type uncoveredTopic struct {
	Path   string
	Domain string
}

// Uncovered assembles the coverage report over the state's eligible paths:
// those neither generated nor contextIgnore-matched (ADR-0134). scanRoots
// restrict the report to paths at or beneath them on slash-separated segment
// boundaries; empty scanRoots scans everything. It writes nothing.
func (q *Query) Uncovered(scanRoots []string) UncoveredResult {
	return assembleUncovered(q.state.Loaded.Topics, q.state.Eligible, safelyMatchablePaths(q.state.Tree), scanRoots)
}

func assembleUncovered(corpus topic.Corpus, eligible, all, scanRoots []string) UncoveredResult {
	roots := NormalizeContextPaths(scanRoots)
	res := UncoveredResult{ScanRoots: roots}
	inScope := func(path string) bool {
		if len(roots) == 0 {
			return true
		}
		for _, r := range roots {
			if r == "." || path == r || strings.HasPrefix(path, r+"/") {
				return true
			}
		}
		return false
	}

	// Owned-but-uncovered: the report wants coverage gaps and nothing else, so it
	// requests the coverage check alone. Fan-out is a budget concern the gate
	// evaluates; asking for it here and discarding the result would be the magic
	// rank this replaced (ADR-0183). There is deliberately no kind filter below:
	// filtering an unrequested class is that discarded answer, and without it a
	// later edit that requests fan-out here fails TestUncovered rather than being
	// silently swallowed (ADR-0184 item 5).
	var scoped []string
	for _, path := range eligible {
		if inScope(path) {
			scoped = append(scoped, path)
		}
	}
	for _, f := range topic.EvaluateCoverage(corpus, scoped, topic.CoveragePolicy{Coverage: true, Fanout: false}) {
		res.Uncovered = append(res.Uncovered, uncoveredTopic{Path: f.Path, Domain: f.Domain})
	}

	// Unowned: eligible paths matched by no domain glob, collapsed to the topmost
	// node with no owned descendant in scope.
	owned := func(path string) bool {
		for _, d := range corpus.DomainPaths {
			if pathglob.MatchAny(d, path) {
				return true
			}
		}
		return false
	}
	coveredDirs := map[string]bool{}
	for _, r := range roots {
		for _, a := range ancestors(r) {
			coveredDirs[a] = true
		}
	}
	var unowned []string
	for _, path := range eligible {
		if !inScope(path) {
			continue
		}
		if owned(path) {
			for _, a := range ancestors(path) {
				coveredDirs[a] = true
			}
			continue
		}
		unowned = append(unowned, path)
	}
	entries := map[string]*unownedEntry{}
	for _, u := range unowned {
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
		e := entries[pick]
		if e == nil {
			e = &unownedEntry{Path: pick}
			entries[pick] = e
		}
		e.UnownedCount++
	}
	// Directory entries additionally count the in-scope scannable paths beneath
	// them that coverage excludes, so a mostly-generated directory never reads
	// as wholly unowned. Only counts render; excluded paths are never listed.
	eligibleSet := map[string]bool{}
	for _, p := range scoped {
		eligibleSet[p] = true
	}
	for _, e := range entries {
		if e.Path != "." && !strings.HasSuffix(e.Path, "/") {
			continue
		}
		prefix := strings.TrimSuffix(e.Path, "/") + "/"
		for _, p := range all {
			if !inScope(p) || eligibleSet[p] {
				continue
			}
			if e.Path == "." || strings.HasPrefix(p, prefix) {
				e.ExcludedCount++
			}
		}
	}
	for _, e := range entries {
		res.Unowned = append(res.Unowned, *e)
	}
	sort.Slice(res.Unowned, func(i, j int) bool { return res.Unowned[i].Path < res.Unowned[j].Path })
	return res
}

// ancestors returns path's directory ancestors from the top down - "." then each
// strict directory prefix - excluding path itself.
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
		if p == "" {
			continue
		}
		c := filepath.ToSlash(filepath.Clean(p))
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
