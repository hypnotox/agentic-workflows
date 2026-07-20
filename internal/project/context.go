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

// ContextResult is the read-only current-state context awf holds for a set of
// repo-relative paths (ADR-0134): their owning domains (each with the rendered
// current-state pointer), the applicable topics with their current claims, a
// separate section of Accepted-ADR pending changes targeting those topics, and
// any queried path matching no configured domain. Authority is the topic claim
// set alone: no ADR, tag, relation, plan, or pitfall is expanded here.
type ContextResult struct {
	Paths   []string        `json:"paths"`
	Domains []DomainRef     `json:"domains"`
	Topics  []TopicContext  `json:"topics"`
	Pending []PendingChange `json:"pending"`
	Unowned []string        `json:"unowned"`
}

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
	ID     string     `json:"id"`
	Title  string     `json:"title"`
	Global bool       `json:"global,omitempty"`
	Claims []ClaimRef `json:"claims"`
}

// ClaimRef is one current-state claim: its full `<domain>/<topic>:<slug>` ID,
// type (rule or invariant), and prose.
type ClaimRef struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Prose string `json:"prose"`
}

// PendingChange is one Accepted-ADR State-changes operation targeting a matched
// topic (ADR-0135). It describes a change that is not yet current: an Accepted
// ADR is normative only for executing its pending change and never overrides the
// topic claims describing current reality, so it renders in its own section.
type PendingChange struct {
	ADR   string `json:"adr"`
	Title string `json:"title"`
	Op    string `json:"op"`
	Claim string `json:"claim"`
}

// ContextFor assembles the read-only current-state context for paths over the
// working-tree universe (ADR-0134, ADR-0135). It loads exactly one working Tree,
// so the selection never mixes a working and an index universe, and it writes
// nothing.
func (p *Project) ContextFor(paths []string) (ContextResult, error) {
	ws, err := p.workingCurrentState()
	if err != nil {
		return ContextResult{}, err
	}
	expanded := expandContextPaths(paths, eligiblePaths(ws.Tree, ws.Lock, p.Cfg.ContextIgnore), ws.Tree)
	return p.assembleContext(ws.Loaded, expanded), nil
}

// StagedContextRoot assembles context without opening working configuration.
// Config, lock, topics, markers, path eligibility, and project presence all
// come from one immutable index snapshot.
func StagedContextRoot(root string, paths []string) (ContextResult, error) {
	p := &Project{Root: root}
	state, err := p.indexCurrentState()
	if err != nil {
		return ContextResult{}, err
	}
	p.Cfg = state.Cfg
	expanded := expandContextPaths(paths, eligiblePaths(state.Tree, state.Lock, state.Cfg.ContextIgnore), state.Tree)
	return p.assembleContext(state.Loaded, expanded), nil
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
	cutoff, gaps := attestationCutoff(lock)
	loaded, cfg, err := loadTreeCurrentState(p.Root, tree, cutoff, gaps)
	if err != nil {
		return indexState{}, err
	}
	if cfg == nil {
		return indexState{}, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	return indexState{Loaded: loaded, Tree: tree, Lock: lock, Cfg: cfg}, nil
}

func expandContextPaths(paths, eligible []string, tree *snapshot.Tree) []string {
	clean := NormalizeContextPaths(paths)
	files := tree.List()
	var expanded []string
	for _, query := range clean {
		if _, exists := tree.Lookup(query); exists {
			expanded = append(expanded, query)
			continue
		}
		prefix := query + "/"
		if query == "." {
			prefix = ""
		}
		isDir := query == "."
		if !isDir {
			for _, file := range files {
				if strings.HasPrefix(file.Path, prefix) {
					isDir = true
					break
				}
			}
		}
		if !isDir {
			expanded = append(expanded, query)
			continue
		}
		for _, path := range eligible {
			if strings.HasPrefix(path, prefix) {
				expanded = append(expanded, path)
			}
		}
	}
	return NormalizeContextPaths(expanded)
}

// assembleContext performs the pure topic-centric selection over one loaded
// universe: owning domains, applicable topics with state-marker-narrowed claims,
// Accepted pending changes on matched topics, and unowned queried paths.
func (p *Project) assembleContext(loaded currentstate.Loaded, paths []string) ContextResult {
	clean := NormalizeContextPaths(paths)
	lay := p.layout()
	res := ContextResult{Paths: clean}
	corpus := loaded.Topics
	domains := slices.Sorted(maps.Keys(corpus.DomainPaths))

	// Owning domains and the unowned queried paths.
	owners := map[string]bool{}
	matched := map[string]bool{}
	for _, d := range domains {
		for _, path := range clean {
			if pathMatchesAny(corpus.DomainPaths[d], path) {
				owners[d] = true
				matched[path] = true
			}
		}
	}
	for _, d := range domains {
		if owners[d] {
			res.Domains = append(res.Domains, DomainRef{Name: d, CurrentState: lay.DocsDir + "/domains/" + d + ".md"})
		}
	}
	for _, path := range clean {
		if !matched[path] {
			res.Unowned = append(res.Unowned, path)
		}
	}

	// Applicable topics with narrowed claims, unioned across the queried files.
	type acc struct {
		topic  topic.Topic
		claims map[string]bool
	}
	accs := map[string]*acc{}
	for _, path := range clean {
		for _, t := range topic.TopicsForPath(corpus, path) {
			id := t.ID.String()
			a := accs[id]
			if a == nil {
				a = &acc{topic: t, claims: map[string]bool{}}
				accs[id] = a
			}
			for _, cl := range applicableClaims(t, corpus.Markers, path) {
				a.claims[cl] = true
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(accs)) {
		a := accs[id]
		tc := TopicContext{ID: id, Title: a.topic.Metadata.Title, Global: a.topic.Metadata.Applies == "global"}
		for _, cl := range a.topic.Claims {
			if a.claims[cl.ID] {
				tc.Claims = append(tc.Claims, ClaimRef{ID: cl.ID, Type: string(cl.Type), Prose: cl.Prose})
			}
		}
		res.Topics = append(res.Topics, tc)
	}

	// Accepted pending changes targeting a matched topic.
	matchedTopics := map[string]bool{}
	for id := range accs {
		matchedTopics[id] = true
	}
	res.Pending = pendingChanges(loaded.ADRs, matchedTopics)
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

// pendingChanges returns the Accepted-ADR State-changes operations whose claim
// targets a matched topic, sorted by ADR number then claim ID.
func pendingChanges(adrs []adr.ADR, matchedTopics map[string]bool) []PendingChange {
	var out []PendingChange
	for _, a := range adrs {
		if !a.IsAccepted() {
			continue
		}
		for _, op := range a.Operations {
			if !matchedTopics[topicOfClaim(op.ID)] {
				continue
			}
			out = append(out, PendingChange{
				ADR:   a.Number,
				Title: strings.TrimPrefix(a.Title, "ADR-"+a.Number+": "),
				Op:    string(op.Verb),
				Claim: op.ID,
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
