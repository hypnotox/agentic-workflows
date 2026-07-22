package project

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// DomainRef is an owning domain and its rendered current-state doc path, derived
// by convention (never a sidecar field - ADR-0086).
type DomainRef struct {
	Name         string `json:"name"`
	CurrentState string `json:"currentState"`
}

// TopicContext is one topic applicable to the queried paths with the claims that
// apply: a topic's whole claim set unless a state marker under a queried path
// narrows the selection to the marked claims (ADR-0134). Global is set for an
// `applies: global` topic, which is always selected.
type TopicContext struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Summary string     `json:"summary"`
	Global  bool       `json:"global,omitempty"`
	Claims  []ClaimRef `json:"claims"`
}

// ClaimRef is one current-state claim: its full `<domain>/<topic>:<slug>` ID,
// type (rule or invariant), prose, and invariant backing contract.
type ClaimRef struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Prose   string `json:"prose"`
	Backing string `json:"backing,omitempty"`
	Verify  string `json:"verify,omitempty"`
}

// PendingChange is one remaining governed ADR operation targeting a matched
// topic. It is not yet current and renders separately from current claims.
type PendingChange struct {
	ADR      string `json:"adr"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Applied  int    `json:"applied"`
	Declared int    `json:"declared"`
	Op       string `json:"op"`
	Claim    string `json:"claim"`
}

// ContextFor assembles the read-only current-state context for paths over the
// working-tree universe (ADR-0134, ADR-0135). It loads exactly one working Tree,
// so the selection never mixes a working and an index universe, and it writes
// nothing.
func (p *Project) ContextFor(paths []string) (ContextResult, error) {
	return p.contextFor(paths, false, ContextConcise)
}

// ContextForFull returns the complete authority projection from the same model.
func (p *Project) ContextForFull(paths []string) (ContextResult, error) {
	return p.contextFor(paths, false, ContextFull)
}

// ContextForGitSelection preserves Git-selected request status while resolving
// authority against the same single working universe.
func (p *Project) ContextForGitSelection(paths []string) (ContextResult, error) {
	return p.contextFor(paths, true, ContextConcise)
}

// ContextForFullGitSelection combines Git request attribution with full authority.
func (p *Project) ContextForFullGitSelection(paths []string) (ContextResult, error) {
	return p.contextFor(paths, true, ContextFull)
}

func (p *Project) contextFor(paths []string, gitSelected bool, projection ContextProjection) (ContextResult, error) {
	ws, err := p.workingCurrentState()
	if err != nil {
		return ContextResult{}, err
	}
	universe := &Project{Root: p.Root, Cfg: ws.Cfg}
	universe.Targets, err = resolveTargets(ws.Cfg.Targets)
	if err != nil {
		return ContextResult{}, err
	}
	universe.Cat, err = universe.effectiveCatalog()
	if err != nil {
		return ContextResult{}, err
	}
	selectedADRs := adr.NewCorpus(ws.Loaded.ADRs)
	decls, err := BuildOutputDeclarations(ws.Cfg, universe.Cat, universe.Targets, snapshotTreeReader{tree: ws.Tree}, selectedADRs)
	if err != nil {
		return ContextResult{}, err
	}
	return universe.assembleContextUniverse(ws.Loaded, ws.Tree, ws.Lock, ws.Cfg, decls, paths, gitSelected, projection), nil
}

// StagedContextRoot assembles every result from one immutable index universe.
func StagedContextRoot(root string, paths []string) (ContextResult, error) {
	return stagedContextRoot(root, paths, false, ContextConcise)
}

// StagedContextRootFull returns the full projection from the immutable index.
func StagedContextRootFull(root string, paths []string) (ContextResult, error) {
	return stagedContextRoot(root, paths, false, ContextFull)
}

// StagedContextRootGitSelection preserves Git-selected request status in the
// immutable index universe.
func StagedContextRootGitSelection(root string, paths []string) (ContextResult, error) {
	return stagedContextRoot(root, paths, true, ContextConcise)
}

// StagedContextRootFullGitSelection combines Git attribution and full authority.
func StagedContextRootFullGitSelection(root string, paths []string) (ContextResult, error) {
	return stagedContextRoot(root, paths, true, ContextFull)
}

func stagedContextRoot(root string, paths []string, gitSelected bool, projection ContextProjection) (ContextResult, error) {
	p := &Project{Root: root}
	state, err := p.indexCurrentState()
	if err != nil {
		return ContextResult{}, err
	}
	p.Cfg = state.Cfg
	p.Targets, err = resolveTargets(state.Cfg.Targets)
	if err != nil {
		return ContextResult{}, err
	}
	p.Cat, err = p.effectiveCatalog()
	if err != nil {
		return ContextResult{}, err
	}
	selectedADRs := adr.NewCorpus(state.Loaded.ADRs)
	decls, err := BuildOutputDeclarations(p.Cfg, p.Cat, p.Targets, snapshotTreeReader{tree: state.Tree}, selectedADRs)
	if err != nil { // coverage-ignore: effectiveCatalog just parsed the same selected sidecars and declaration enumeration has no other error source
		return ContextResult{}, err
	}
	return p.assembleContextUniverse(state.Loaded, state.Tree, state.Lock, state.Cfg, decls, paths, gitSelected, projection), nil
}

type indexState struct {
	Loaded currentstate.Loaded
	Tree   *snapshot.Tree
	Lock   *manifest.Lock
	Cfg    *config.Config
}

func (p *Project) indexCurrentState() (indexState, error) {
	tree, err := snapshot.IndexTree(p.Root)
	if err != nil {
		return indexState{}, err
	}
	lock, err := lockFromTree(tree)
	if err != nil {
		return indexState{}, err
	}
	boundaries, gaps := attestationBoundaries(lock)
	loaded, cfg, err := loadTreeCurrentState(p.Root, tree, boundaries, gaps)
	if err != nil {
		return indexState{}, err
	}
	if cfg == nil {
		return indexState{}, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	return indexState{Loaded: loaded, Tree: tree, Lock: lock, Cfg: cfg}, nil
}

// assembleContextUniverse classifies requests and resolves authority from one
// selected snapshot independently of coverage eligibility.
func (p *Project) assembleContextUniverse(loaded currentstate.Loaded, tree *snapshot.Tree, lock *manifest.Lock, cfg *config.Config, declarations []OutputDeclaration, queries []string, gitSelected bool, projection ContextProjection) ContextResult {
	outputs := map[string]bool{}
	for _, d := range declarations {
		if !d.Reservation {
			outputs[d.Path] = true
		}
	}
	files := tree.List()
	nested := []string{}
	for _, f := range files {
		if !isMetricsResidentPath(f.Path) && f.Scannable() && strings.HasSuffix(f.Path, "/"+config.DirName+"/config.yaml") {
			nested = append(nested, strings.TrimSuffix(f.Path, "/"+config.DirName+"/config.yaml"))
		}
	}
	slices.Sort(nested)
	set := contextPathSet{tree: tree, eligible: eligiblePaths(tree, lock, cfg.ContextIgnore), nested: nested, outputs: outputs, ignores: cfg.ContextIgnore, domainPaths: loaded.Topics.DomainPaths}
	requests, attribution := buildContextRequests(queries, gitSelected, set)
	res := ContextResult{Projection: projection, Requests: requests, Paths: []ContextPath{}}
	lay := p.layout()
	currentPaths := safelyMatchablePaths(tree)
	for _, path := range slices.Sorted(maps.Keys(attribution)) {
		class, nestedRoot, targetInside := classifyContextPath(path, set)
		selectedADRs := adr.NewCorpus(loaded.ADRs)
		cp := ContextPath{Path: path, Requests: slices.Clone(attribution[path]), Classification: class, TargetInsideRepository: targetInside, NestedRoot: nestedRoot, Domains: []DomainRef{}, Topics: []PathTopicContext{}, Pending: []PendingChange{}, Artifacts: artifactRecords(path, declarations, artifactAuthorities{Layout: lay, ADRs: selectedADRs})}
		applyArtifactSnapshots(cp.Artifacts, path, tree, lock)
		safe := class != PathOutsideRepository && class != PathNestedAdopter && class != PathSymlink
		matchedTopics := map[string]bool{}
		if safe {
			for _, d := range slices.Sorted(maps.Keys(loaded.Topics.DomainPaths)) {
				if pathMatchesAny(loaded.Topics.DomainPaths[d], path) {
					cp.Domains = append(cp.Domains, DomainRef{Name: d, CurrentState: lay.DocsDir + "/domains/" + d + ".md"})
				}
			}
			applicable := topic.TopicsForPath(loaded.Topics, path)
			for _, t := range applicable {
				matchedTopics[t.ID.String()] = true
			}
			cp.Pending = pendingChanges(loaded.ADRs, matchedTopics)
			for _, t := range applicable {
				cp.Topics = append(cp.Topics, projectPathTopic(t, loaded.Topics, path, currentPaths, cp.Pending, projection))
			}
		}
		if explicitContextPath(requests, path) {
			cp.ADR = projectADRArtifact(path, lay.ADRDir, adr.NewCorpus(loaded.ADRs), loaded.Topics, projection)
		}
		res.Paths = append(res.Paths, cp)
		for _, d := range cp.Domains {
			if !slices.Contains(res.Domains, d) {
				res.Domains = append(res.Domains, d)
			}
		}
		if class == PathEligibleUnowned || class == PathNotFound {
			res.Unowned = append(res.Unowned, path)
		}
		for _, pt := range cp.Topics {
			idx := -1
			for i := range res.Topics {
				if res.Topics[i].ID == pt.ID {
					idx = i
					break
				}
			}
			if idx < 0 {
				res.Topics = append(res.Topics, TopicContext{ID: pt.ID, Title: pt.Title, Summary: pt.Summary})
				idx = len(res.Topics) - 1
			}
			if t, ok := loaded.Topics.ByTopicID(pt.ID); ok {
				res.Topics[idx].Global = t.Metadata.Applies == "global"
				selected := applicableClaims(t, loaded.Topics.Markers, path)
				for _, claim := range t.Claims {
					if slices.Contains(selected, claim.ID) {
						ref := ClaimRef{ID: claim.ID, Type: string(claim.Type), Prose: claim.Prose, Backing: string(claim.Backing), Verify: claim.Verify}
						if !slices.Contains(res.Topics[idx].Claims, ref) {
							res.Topics[idx].Claims = append(res.Topics[idx].Claims, ref)
						}
					}
				}
			}
		}
		for _, pending := range cp.Pending {
			if !slices.Contains(res.Pending, pending) {
				res.Pending = append(res.Pending, pending)
			}
		}
	}
	return res
}

// applicableClaims returns the IDs of the claims of t that apply at path. A state
// marker under path for one of t's claims narrows the selection to the marked
// claims (never expanding beyond the topic - the topic already matched path);
// absent any such marker every claim of t applies.
func applicableClaims(t topic.Topic, markers topic.MarkerIndex, path string) []string {
	var narrowed []string
	for _, cl := range t.Claims {
		for _, s := range markers.ForClaim(cl.ID) {
			if s.Kind == topic.StateMarker && s.Path == path {
				narrowed = append(narrowed, cl.ID)
				break
			}
		}
	}
	if len(narrowed) > 0 {
		return narrowed
	}
	all := make([]string, len(t.Claims))
	for i, cl := range t.Claims {
		all[i] = cl.ID
	}
	return all
}

// pendingChanges returns remaining Accepted and Implementing operations whose
// claims target a matched topic, sorted by ADR number then claim ID.
func pendingChanges(adrs []adr.ADR, matchedTopics map[string]bool) []PendingChange {
	var out []PendingChange
	corpus := adr.NewCorpus(adrs)
	for _, a := range corpus.All() {
		if !a.IsAccepted() && !a.IsImplementing() {
			continue
		}
		progress, _, err := corpus.OperationProgress(a.Number)
		if err != nil {
			continue
		}
		declared := len(progress.Applied) + len(progress.Remaining) + len(progress.Canceled)
		for _, op := range progress.Remaining {
			if !matchedTopics[topicOfClaim(op.ID)] {
				continue
			}
			out = append(out, PendingChange{
				ADR:      a.Number,
				Title:    strings.TrimPrefix(a.Title, "ADR-"+a.Number+": "),
				Status:   a.Status,
				Applied:  len(progress.Applied),
				Declared: declared,
				Op:       string(op.Verb),
				Claim:    op.ID,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ADR != out[j].ADR {
			return out[i].ADR < out[j].ADR
		}
		return out[i].Claim < out[j].Claim
	})
	return out
}

// topicOfClaim returns the `<domain>/<topic>` prefix of a qualified claim ID.
func topicOfClaim(claimID string) string {
	if i := strings.Index(claimID, ":"); i >= 0 {
		return claimID[:i]
	}
	return claimID // coverage-ignore: every ADR operation ID is a validated qualified claim ID
}

// pathMatchesAny reports whether path matches any of globs.
func pathMatchesAny(globs []string, path string) bool {
	for _, g := range globs {
		if pathglob.Match(g, path) {
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
	ScanRoots []string         `json:"scanRoots"`
	Unowned   []string         `json:"unowned"`
	Uncovered []UncoveredTopic `json:"uncovered"`
}

// UncoveredTopic is one domain-owned path lacking a scoped topic that covers it.
type UncoveredTopic struct {
	Path   string `json:"path"`
	Domain string `json:"domain"`
}

// Uncovered assembles the coverage report over the working-tree eligible paths:
// those neither generated nor contextIgnore-matched (ADR-0134). scanRoots
// restrict the report to paths at or beneath them on slash-separated segment
// boundaries; empty scanRoots scans everything. It writes nothing.
func (p *Project) Uncovered(scanRoots []string) (UncoveredResult, error) {
	ws, err := p.workingCurrentState()
	if err != nil {
		return UncoveredResult{}, err
	}
	return assembleUncovered(ws.Loaded.Topics, p.eligibleCoveragePaths(ws.Tree, ws.Lock), scanRoots), nil
}

// StagedUncoveredRoot reports coverage entirely from the index universe.
func StagedUncoveredRoot(root string, scanRoots []string) (UncoveredResult, error) {
	state, err := (&Project{Root: root}).indexCurrentState()
	if err != nil {
		return UncoveredResult{}, err
	}
	return assembleUncovered(state.Loaded.Topics, eligiblePaths(state.Tree, state.Lock, state.Cfg.ContextIgnore), scanRoots), nil
}

func assembleUncovered(corpus topic.Corpus, eligible, scanRoots []string) UncoveredResult {
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

	// Owned-but-uncovered: force the coverage severity so every domain-owned path
	// with no claim-bearing scoped topic is reported regardless of the project's
	// configured strictness (the report shows gaps; the gate applies severity).
	var scoped []string
	for _, path := range eligible {
		if inScope(path) {
			scoped = append(scoped, path)
		}
	}
	for _, f := range topic.EvaluateCoverage(corpus, scoped, topic.CoveragePolicy{Coverage: topic.CoverageError, Fanout: topic.CoverageOff}) {
		if f.Kind == topic.Uncovered {
			res.Uncovered = append(res.Uncovered, UncoveredTopic{Path: f.Path, Domain: f.Domain})
		}
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
	entries := map[string]bool{}
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
		entries[pick] = true
	}
	for e := range entries {
		res.Unowned = append(res.Unowned, e)
	}
	sort.Strings(res.Unowned)
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
