package project

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// DomainRef is an owning domain and its rendered current-state doc path.
type DomainRef struct{ Name, CurrentState string }

type PendingChange struct {
	ADR, Title, Status string
	Applied, Declared  int
	Op, Claim          string
	Progress           string
}

type contextAssemblyState struct {
	Loaded       currentstate.Loaded
	Tree         *snapshot.Tree
	Lock         *manifest.Lock
	Config       *config.Config
	Declarations []OutputDeclaration
}

func (p *Project) ContextForOptions(ctx context.Context, paths []string, options ContextOptions) (ContextResult, error) {
	ws, err := p.workingCurrentState(ctx)
	if err != nil {
		return ContextResult{}, err
	}
	universe := &Project{Root: p.Root, Cfg: ws.Cfg, standard: p.standard, repo: p.repo}
	universe.Targets, err = resolveTargets(ws.Cfg.Targets)
	if err != nil {
		return ContextResult{}, err
	}
	universe.Cat, err = universe.effectiveCatalog()
	if err != nil {
		return ContextResult{}, err
	}
	declarations, err := BuildOutputDeclarations(ws.Cfg, universe.Cat, universe.Targets, snapshotTreeReader{tree: ws.Tree}, adr.NewCorpus(ws.Loaded.ADRs))
	if err != nil { // coverage-ignore: the snapshot-local catalog and every declaration input were already parsed from this immutable tree
		return ContextResult{}, err
	}
	return universe.assembleContextUniverse(contextAssemblyState{Loaded: ws.Loaded, Tree: ws.Tree, Lock: ws.Lock, Config: ws.Cfg, Declarations: declarations}, paths, options)
}

func StagedContextRootOptions(ctx context.Context, root string, paths []string, options ContextOptions) (ContextResult, error) {
	p, err := openRootProject(root)
	if err != nil {
		return ContextResult{}, err
	}
	state, err := p.indexCurrentState(ctx)
	if err != nil {
		return ContextResult{}, err
	}
	targets, err := resolveTargets(state.Cfg.Targets)
	if err != nil {
		return ContextResult{}, err
	}
	universe := &Project{Root: root, Cfg: state.Cfg, Targets: targets, standard: catalog.Standard, repo: p.repo}
	universe.Cat, err = universe.effectiveCatalog()
	if err != nil {
		return ContextResult{}, err
	}
	declarations, err := BuildOutputDeclarations(state.Cfg, universe.Cat, universe.Targets, snapshotTreeReader{tree: state.Tree}, adr.NewCorpus(state.Loaded.ADRs))
	if err != nil { // coverage-ignore: the staged snapshot-local catalog and every declaration input were already parsed from this immutable tree
		return ContextResult{}, err
	}
	return universe.assembleContextUniverse(contextAssemblyState{Loaded: state.Loaded, Tree: state.Tree, Lock: state.Lock, Config: state.Cfg, Declarations: declarations}, paths, options)
}

type indexState struct {
	Loaded currentstate.Loaded
	Tree   *snapshot.Tree
	Lock   *manifest.Lock
	Cfg    *config.Config
}

func (p *Project) indexCurrentState(ctx context.Context) (indexState, error) {
	tree, err := p.indexTree(ctx)
	if err != nil {
		return indexState{}, err
	}
	lock, err := lockFromTree(tree)
	if err != nil {
		return indexState{}, err
	}
	boundaries, gaps := attestationBoundaries(lock)
	loaded, cfg, err := loadTreeCurrentState(p.Root, tree, lock, boundaries, gaps)
	if err != nil {
		return indexState{}, err
	}
	if cfg == nil {
		return indexState{}, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	return indexState{Loaded: loaded, Tree: tree, Lock: lock, Cfg: cfg}, nil
}

func (p *Project) assembleContextUniverse(state contextAssemblyState, queries []string, options ContextOptions) (ContextResult, error) {
	if options.Selection == "" {
		options.Selection = SelectionExplicit
	}
	outputs := map[string]bool{}
	for _, d := range state.Declarations {
		if !d.Reservation {
			outputs[d.Path] = true
		}
	}
	nested := []string{}
	for _, f := range state.Tree.List() {
		if !isResidentPath(f.Path) && f.Scannable() && strings.HasSuffix(f.Path, "/"+config.DirName+"/config.yaml") {
			nested = append(nested, strings.TrimSuffix(f.Path, "/"+config.DirName+"/config.yaml"))
		}
	}
	slices.Sort(nested)
	set := contextPathSet{tree: state.Tree, eligible: eligiblePaths(state.Tree, state.Lock, state.Config.ContextIgnore), nested: nested, outputs: outputs, ignores: state.Config.ContextIgnore, domainPaths: state.Loaded.Topics.DomainPaths, impacts: map[string]ContextPathImpact{}}
	selectedADRs := adr.NewCorpus(state.Loaded.ADRs)
	lay := p.layout()
	markerSitesByPath := map[string][]topic.MarkerSite{}
	for _, site := range state.Loaded.Topics.Markers.All() {
		markerSitesByPath[site.Path] = append(markerSitesByPath[site.Path], site)
	}
	makeImpact := func(filePath string, explicit bool) ContextPathImpact {
		class, nestedRoot, targetInside := classifyContextPath(filePath, set)
		records := artifactRecords(filePath, state.Declarations, artifactAuthorities{Layout: lay, ADRs: selectedADRs})
		applyArtifactSnapshots(records, filePath, state.Tree, state.Lock)
		impact := ContextPathImpact{Classification: class, NestedRoot: nestedRoot, TargetInsideRepository: targetInside, Provenance: []ContextProvenance{}, Domains: []DomainRef{}, Topics: []ContextPathTopic{}, Relationships: emptyContextRelationships(), Warnings: []ContextWarning{}}
		for _, record := range records {
			impact.Provenance = append(impact.Provenance, ContextProvenance{Role: string(record.Role), Identity: record.Identity, Sources: cloneArtifactLinks(record.Sources), Outputs: cloneArtifactLinks(record.Outputs), Navigation: cloneArtifactLinks(record.Navigation)})
		}
		literalGlob := explicit && (class == PathNotFound || class == PathContextIgnored) && globLiteralQuery(filePath)
		safe := class != PathOutsideRepository && class != PathNestedAdopter && class != PathSymlink
		if safe && !literalGlob {
			impact.Relationships = contextRelationshipsForPath(markerSitesByPath, filePath)
			for _, d := range slices.Sorted(maps.Keys(state.Loaded.Topics.DomainPaths)) {
				if pathMatchesAny(state.Loaded.Topics.DomainPaths[d], filePath) {
					impact.Domains = append(impact.Domains, DomainRef{Name: d, CurrentState: lay.DocsDir + "/domains/" + d + ".md"})
				}
			}
			for _, t := range topic.TopicsForPath(state.Loaded.Topics, filePath) {
				impact.Topics = append(impact.Topics, ContextPathTopic{ID: t.ID.String()})
			}
		}
		if literalGlob {
			impact.Warnings = append(impact.Warnings, WarningGlobLiteral)
		}
		if class == PathEligibleUnowned {
			impact.Warnings = append(impact.Warnings, WarningEligibleUnowned)
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
			q := filepath.ToSlash(filepath.Clean(raw))
			set.impacts[q] = makeImpact(q, options.Selection == SelectionExplicit)
		}
	}
	requests := buildContextRequests(queries, set, options)
	directSources := map[string]map[int]map[string]bool{}
	applicable := map[string]topic.Topic{}
	for _, request := range requests {
		impacts := []ContextPathImpact{}
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
	result := ContextResult{Selection: options.Selection, Range: options.Range, Requests: requests, Topics: []TopicImpact{}}
	currentPaths := safelyMatchablePaths(state.Tree)
	projectedSources := contextRelationshipSources(directSources)
	globallyVisible := contextVisibleClaimIDs(applicable, projectedSources, options.Facets)
	referencedSeen := map[string]bool{}
	for _, id := range slices.Sorted(maps.Keys(applicable)) {
		result.Topics = append(result.Topics, projectTopicImpact(applicable[id], state.Loaded.Topics, projectedSources, globallyVisible, referencedSeen, currentPaths, pendingChanges(state.Loaded.ADRs, map[string]bool{id: true}), options.Facets))
	}
	return result, nil
}

func contextVisibleClaimIDs(applicable map[string]topic.Topic, directSources map[string][]ContextRelationshipSource, facets []ContextFacet) map[string]bool {
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

func addContextRelationshipSources(dst map[string]map[int]map[string]bool, requestIndex int, relationships ContextRelationships) {
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

func contextRelationshipSources(in map[string]map[int]map[string]bool) map[string][]ContextRelationshipSource {
	out := map[string][]ContextRelationshipSource{}
	for id, byRequest := range in {
		for _, requestIndex := range slices.Sorted(maps.Keys(byRequest)) {
			kinds := []string{}
			for _, kind := range []string{"State", "Touches", "Proofs"} {
				if byRequest[requestIndex][kind] {
					kinds = append(kinds, kind)
				}
			}
			out[id] = append(out[id], ContextRelationshipSource{RequestIndex: requestIndex, Kinds: kinds})
		}
	}
	return out
}

func pendingChanges(adrs []adr.ADR, matchedTopics map[string]bool) []PendingChange {
	var out []PendingChange
	corpus := adr.NewCorpus(adrs)
	ordered := slices.Clone(corpus.All())
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Number < ordered[j].Number })
	for _, a := range ordered {
		if !a.IsAccepted() && !a.IsImplementing() {
			continue
		}
		progress, _, err := corpus.OperationProgress(a.Number)
		if err != nil {
			continue
		}
		declared := len(progress.Applied) + len(progress.Remaining) + len(progress.Canceled)
		for _, op := range progress.Remaining {
			if matchedTopics[topicOfClaim(op.ID)] {
				out = append(out, PendingChange{ADR: a.Number, Title: strings.TrimPrefix(a.Title, "ADR-"+a.Number+": "), Status: a.Status, Applied: len(progress.Applied), Declared: declared, Op: string(op.Verb), Claim: op.ID, Progress: "remaining"})
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
func pathMatchesAny(globs []string, p string) bool {
	for _, g := range globs {
		if pathglob.Match(g, p) {
			return true
		}
	}
	return false
}

// UncoveredResult is the read-only coverage report for a set of scan roots: the
// eligible paths owned by no domain (collapsed to a topmost trailing-slash node)
// and the domain-owned paths with no claim-bearing scoped topic (ADR-0134).
// ScanRoots echoes the requested roots (empty = whole repository).
type UncoveredResult struct {
	ScanRoots []string
	Unowned   []UnownedEntry
	Uncovered []UncoveredTopic
}

// UnownedEntry is one collapsed unowned node: UnownedCount is the in-scope
// eligible unowned paths it covers, ExcludedCount the in-scope scannable paths
// beneath it that coverage excludes (generated, context-ignored, resident,
// or nested-adopter). Plain file entries keep ExcludedCount zero.
type UnownedEntry struct {
	Path          string
	UnownedCount  int
	ExcludedCount int
}

// UncoveredTopic is one domain-owned path lacking a scoped topic that covers it.
type UncoveredTopic struct {
	Path   string
	Domain string
}

// Uncovered assembles the coverage report over the working-tree eligible paths:
// those neither generated nor contextIgnore-matched (ADR-0134). scanRoots
// restrict the report to paths at or beneath them on slash-separated segment
// boundaries; empty scanRoots scans everything. It writes nothing.
func (p *Project) Uncovered(ctx context.Context, scanRoots []string) (UncoveredResult, error) {
	ws, err := p.workingCurrentState(ctx)
	if err != nil {
		return UncoveredResult{}, err
	}
	return assembleUncovered(ws.Loaded.Topics, p.eligibleCoveragePaths(ws.Tree, ws.Lock), safelyMatchablePaths(ws.Tree), scanRoots), nil
}

// StagedUncoveredRoot reports coverage entirely from the index universe.
func StagedUncoveredRoot(ctx context.Context, root string, scanRoots []string) (UncoveredResult, error) {
	p, err := openRootProject(root)
	if err != nil {
		return UncoveredResult{}, err
	}
	state, err := p.indexCurrentState(ctx)
	if err != nil {
		return UncoveredResult{}, err
	}
	return assembleUncovered(state.Loaded.Topics, eligiblePaths(state.Tree, state.Lock, state.Cfg.ContextIgnore), safelyMatchablePaths(state.Tree), scanRoots), nil
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
		res.Uncovered = append(res.Uncovered, UncoveredTopic{Path: f.Path, Domain: f.Domain})
	}

	// Unowned: eligible paths matched by no domain glob, collapsed to the topmost
	// node with no owned descendant in scope.
	owned := func(path string) bool {
		for _, d := range corpus.DomainPaths {
			if pathMatchesAny(d, path) {
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
	entries := map[string]*UnownedEntry{}
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
			e = &UnownedEntry{Path: pick}
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
