package audit

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

const currentStateTransitionRule = "current-state-transition"

type rangeWalker func(context.Context, string, string, func(awfgit.Commit) error) (int, error)
type revisionLoader func(context.Context, string) (*revisionState, error)

// replayCommit is the compact evidence historical replay needs after ordinary
// rules have observed a rich commit.
type replayCommit struct {
	Ordinal                 int
	Hash, Revision, Subject string
	Parents                 []string
	IsMerge                 bool
	Message                 string
	Paths                   []string
}

func compactReplayCommit(ordinal int, commit awfgit.Commit) replayCommit {
	paths := map[string]bool{}
	for _, change := range commit.Changes {
		if change.OldPath != "" {
			paths[change.OldPath] = true
		}
		if change.Path != "" {
			paths[change.Path] = true
		}
	}
	message := ""
	if commit.IsMerge {
		message = commit.Message
	}
	return replayCommit{Ordinal: ordinal, Hash: commit.Hash, Revision: commit.Revision, Subject: strings.Clone(commit.Subject), Parents: slices.Clone(commit.Parents), IsMerge: commit.IsMerge, Message: message, Paths: slices.Sorted(maps.Keys(paths))}
}

type firstParentPaths func(context.Context, string) ([]string, error)
type liveEvaluator func(context.Context) ([]Finding, error)

// replayGraph validates compact history evidence and schedules each selected
// child before its selected parents. Parents outside the selected range are
// retained as direct boundary dependencies and are never expanded.
type replayGraph struct {
	schedule   []replayCommit
	boundaries map[string]bool
}

func newReplayGraph(commits []replayCommit) (replayGraph, error) {
	byRevision := make(map[string]replayCommit, len(commits))
	for _, commit := range commits {
		if commit.Revision == "" {
			return replayGraph{}, errors.New("replay commit has no revision")
		}
		if _, exists := byRevision[commit.Revision]; exists {
			return replayGraph{}, fmt.Errorf("duplicate replay revision %s", commit.Revision)
		}
		byRevision[commit.Revision] = commit
	}
	remainingChildren := make(map[string]int, len(commits))
	boundaries := map[string]bool{}
	for _, commit := range commits {
		for _, parent := range commit.Parents {
			if parent == "" {
				return replayGraph{}, fmt.Errorf("replay revision %s has an empty parent", commit.Revision)
			}
			if parent == commit.Revision {
				return replayGraph{}, fmt.Errorf("replay revision %s names itself as a parent", parent)
			}
			if _, selected := byRevision[parent]; selected {
				remainingChildren[parent]++
			} else {
				boundaries[parent] = true
			}
		}
	}
	ready := make([]string, 0, len(commits))
	for revision := range byRevision {
		if remainingChildren[revision] == 0 {
			ready = append(ready, revision)
		}
	}
	slices.Sort(ready)
	schedule := make([]replayCommit, 0, len(commits))
	for len(ready) > 0 {
		revision := ready[0]
		ready = ready[1:]
		commit := byRevision[revision]
		schedule = append(schedule, commit)
		for _, parent := range commit.Parents {
			if _, selected := byRevision[parent]; !selected {
				continue
			}
			remainingChildren[parent]--
			if remainingChildren[parent] == 0 {
				ready = append(ready, parent)
				slices.Sort(ready)
			}
		}
	}
	if len(schedule) != len(commits) {
		return replayGraph{}, errors.New("cycle among selected replay revisions")
	}
	return replayGraph{schedule: schedule, boundaries: boundaries}, nil
}

// historyOperation owns every historical input and derived revision state for
// one audit invocation. Nothing on it is retained by Project or shared with a
// later invocation.
type historyOperation struct {
	commits          []replayCommit
	ordinary         []Finding
	visited          int
	loadRevision     revisionLoader
	firstParentPaths firstParentPaths
	live             liveEvaluator
	store            revisionStore
	universeUses     map[string]int
	sourceUses       map[string]int
}

type revisionResult struct {
	state *revisionState
	err   error
}

// revisionStore separates revision keys from the canonical immutable state they
// resolve to. Its counters are logical ownership instrumentation, not heap
// measurements: they make the heavy frontier deterministic in tests.
type revisionStore struct {
	keys           map[string]revisionResult
	currentHeavy   int
	highWaterHeavy int
}

func newRevisionStore() revisionStore { return revisionStore{keys: map[string]revisionResult{}} }

func (s *revisionStore) retainHeavy(state *revisionState) {
	if state.heavyLive {
		return
	}
	state.heavyLive = true
	s.currentHeavy++
	if s.currentHeavy > s.highWaterHeavy {
		s.highWaterHeavy = s.currentHeavy
	}
}

func (s *revisionStore) releaseHeavy(state *revisionState) {
	if !state.heavyLive {
		return
	}
	state.heavyLive = false
	s.currentHeavy--
	state.universe = currentstate.Universe{}
	state.universeErr = nil
}

// revisionState is one lazily parsed committed revision. The committed snapshot
// is loaded once by the operation; lock and current-state parsing are separately
// lazy so a pre-policy merge can inspect only its schema boundary.
type revisionState struct {
	loadLock          func() (*manifest.Lock, bool, error)
	loadUniverse      func() (currentstate.Universe, error)
	heavyMaterialized bool
	heavyLive         bool

	lockReady bool
	lock      *manifest.Lock
	lockFound bool
	lockErr   error

	universeReady bool
	universe      currentstate.Universe
	universeErr   error

	loadConfig  func() (*config.Config, error)
	configReady bool
	config      *config.Config
	configErr   error
}

func newStreamingHistoryOperation(ctx context.Context, base, head string, in Inputs, walk rangeWalker, load revisionLoader, paths firstParentPaths, live liveEvaluator) (*historyOperation, error) {
	evaluator := newRangeEvaluator(in)
	var commits []replayCommit
	count, err := walk(ctx, base, head, func(commit awfgit.Commit) error {
		evaluator.observe(commit)
		commits = append(commits, compactReplayCommit(len(commits), commit))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect audit range: %w", err)
	}
	return newHistoryOperationFromCompact(commits, evaluator.findings(), count, load, paths, live), nil
}

func newHistoryOperationFromCompact(commits []replayCommit, ordinary []Finding, visited int, load revisionLoader, paths firstParentPaths, live liveEvaluator) *historyOperation {
	return &historyOperation{
		commits:          slices.Clone(commits),
		ordinary:         slices.Clone(ordinary),
		visited:          visited,
		loadRevision:     load,
		firstParentPaths: paths,
		live:             live,
		store:            newRevisionStore(),
		universeUses:     map[string]int{},
		sourceUses:       map[string]int{},
	}
}

type scheduledFindings struct {
	ordinal  int
	findings []Finding
}

func findingsByStreamOrdinal(groups []scheduledFindings) []Finding {
	slices.SortStableFunc(groups, func(a, b scheduledFindings) int { return a.ordinal - b.ordinal })
	var findings []Finding
	for _, group := range groups {
		findings = append(findings, group.findings...)
	}
	return findings
}

func replayContext(ctx context.Context, consumer string, commit replayCommit) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s for %s: %w", consumer, commit.Hash, err)
	}
	return nil
}

func (h *historyOperation) run(ctx context.Context) ([]Finding, error) {
	graph, err := newReplayGraph(h.commits)
	if err != nil {
		return nil, fmt.Errorf("build historical replay graph: %w", err)
	}
	h.reserveConsumers(graph.schedule)
	var stale, transitions []scheduledFindings
	for _, commit := range graph.schedule {
		if err := replayContext(ctx, "replay stale-merge evidence", commit); err != nil {
			return nil, err
		}
		stepStale, err := h.replayStale(ctx, commit)
		if err != nil {
			return nil, err
		}
		if len(stepStale) > 0 {
			stale = append(stale, scheduledFindings{ordinal: commit.Ordinal, findings: stepStale})
		}
		if err := replayContext(ctx, "replay transition evidence", commit); err != nil {
			return nil, err
		}
		stepTransitions, err := h.replayTransition(ctx, commit)
		if err != nil {
			return nil, err
		}
		if len(stepTransitions) > 0 {
			transitions = append(transitions, scheduledFindings{ordinal: commit.Ordinal, findings: stepTransitions})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("evaluate live cleanliness: %w", err)
	}
	live, err := h.live(ctx)
	if err != nil {
		return nil, fmt.Errorf("evaluate live cleanliness: %w", err)
	}
	findings := slices.Clone(h.ordinary)
	findings = append(findings, findingsByStreamOrdinal(stale)...)
	findings = append(findings, live...)
	return append(findings, findingsByStreamOrdinal(transitions)...), nil
}

func (h *historyOperation) state(ctx context.Context, revision string) (*revisionState, error) {
	if cached, ok := h.store.keys[revision]; ok {
		return cached.state, cached.err
	}
	state, err := h.loadRevision(ctx, revision)
	if err == nil && state == nil {
		err = errors.New("revision loader returned no state")
	}
	h.store.keys[revision] = revisionResult{state: state, err: err}
	return state, err
}

// stateForCommit reuses the first-parent state only after committed path
// evidence proves this revision cannot affect the reduced policy authority.
func (h *historyOperation) stateForCommit(ctx context.Context, commit replayCommit) (*revisionState, error) {
	if cached, ok := h.store.keys[commit.Revision]; ok {
		return cached.state, cached.err
	}
	if len(commit.Parents) == 0 || h.firstParentPaths == nil {
		return h.state(ctx, commit.Revision)
	}
	parent, parentErr := h.state(ctx, commit.Parents[0])
	if parentErr != nil {
		if contextTermination(parentErr) {
			return nil, fmt.Errorf("derive first-parent state for %s: %w", commit.Hash, parentErr)
		}
		return h.state(ctx, commit.Revision)
	}
	docsDir, err := parent.committedDocsDir()
	if err != nil {
		if contextTermination(err) {
			return nil, fmt.Errorf("derive first-parent configuration for %s: %w", commit.Hash, err)
		}
		return h.state(ctx, commit.Revision)
	}
	var paths []string
	if commit.IsMerge {
		paths, err = h.firstParentPaths(ctx, commit.Revision)
	} else {
		paths = commit.Paths
	}
	if err != nil {
		if contextTermination(err) {
			return nil, fmt.Errorf("derive first-parent changed paths for %s: %w", commit.Hash, err)
		}
		return h.state(ctx, commit.Revision)
	}
	if policyRelevant(paths, docsDir) {
		return h.state(ctx, commit.Revision)
	}
	// Alias the immutable parent result, including a previously cached error;
	// neither the value nor its slices/maps are mutated by this operation.
	h.store.keys[commit.Revision] = h.store.keys[commit.Parents[0]]
	return parent, nil
}

func policyRelevant(paths []string, docsDir string) bool {
	if docsDir == "" {
		return true
	}
	decisionsPrefix := path.Join(filepath.ToSlash(docsDir), "decisions") + "/"
	for _, changed := range paths {
		if changed == "" || changed == config.DirName || strings.HasPrefix(changed, config.DirName+"/") {
			return true
		}
		if strings.HasPrefix(changed, decisionsPrefix) {
			rel := strings.TrimPrefix(changed, decisionsPrefix)
			if rel != "" && !strings.Contains(rel, "/") && strings.HasSuffix(rel, ".md") {
				return true
			}
		}
	}
	return false
}

func (s *revisionState) lockEvidence() (*manifest.Lock, bool, error) {
	if !s.lockReady {
		s.lock, s.lockFound, s.lockErr = s.loadLock()
		s.lockReady = true
	}
	return s.lock, s.lockFound, s.lockErr
}

func (s *revisionState) currentState() (currentstate.Universe, error) {
	if !s.universeReady {
		load := s.loadUniverse
		s.loadUniverse = nil // release the exact selected blobs captured by the loader
		if load != nil {
			s.universe, s.universeErr = load()
			s.heavyMaterialized = true
		}
		s.universeReady = true
	}
	return s.universe, s.universeErr
}

func (h *historyOperation) currentState(state *revisionState) (currentstate.Universe, error) {
	universe, err := state.currentState()
	if state.heavyMaterialized {
		h.store.retainHeavy(state)
	}
	return universe, err
}

// reserveConsumers counts fixed replay roles without reading heavy authority.
// They attach to revision keys until relevance has resolved aliases.
func (h *historyOperation) reserveConsumers(schedule []replayCommit) {
	for _, commit := range schedule {
		h.universeUses[commit.Revision]++ // transition result
		if len(commit.Parents) > 0 {
			h.universeUses[commit.Parents[0]]++ // transition first parent
		}
		if commit.IsMerge {
			for _, revision := range append([]string{commit.Revision}, commit.Parents...) {
				h.universeUses[revision]++
				h.sourceUses[revision]++
			}
		}
	}
}

func (h *historyOperation) remaining(uses map[string]int, state *revisionState) int {
	remaining := 0
	for revision, result := range h.store.keys {
		if result.state == state {
			remaining += uses[revision]
		}
	}
	return remaining
}

func (h *historyOperation) consumeUniverse(revision string, state *revisionState) {
	if state == nil || h.universeUses[revision] == 0 {
		return
	}
	h.universeUses[revision]--
	if h.remaining(h.universeUses, state) == 0 {
		h.store.releaseHeavy(state)
	}
}

func (h *historyOperation) consumeSources(revision string, state *revisionState) {
	if state == nil || h.sourceUses[revision] == 0 {
		return
	}
	h.sourceUses[revision]--
	if h.remaining(h.sourceUses, state) == 0 {
		state.universe.Sources = nil
	}
}

func (s *revisionState) committedConfig() (*config.Config, error) {
	if !s.configReady {
		if s.loadConfig != nil {
			s.config, s.configErr = s.loadConfig()
		}
		s.configReady = true
	}
	return s.config, s.configErr
}

func (s *revisionState) committedDocsDir() (string, error) {
	cfg, err := s.committedConfig()
	if err != nil || cfg == nil {
		return "", err
	}
	return cfg.DocsDir, nil
}

// loadSelectedRevision reads only the committed authority needed by historical
// policy. Metadata first discovers the optional controls; config then determines
// the exact ADR directory before the remaining authority blobs are read.
func loadSelectedRevision(ctx context.Context, root, revision string, entryRead func(context.Context, string) ([]awfgit.TreeEntry, error), blobRead func(context.Context, string, []string) ([]awfgit.IndexBlob, error)) (*revisionState, error) {
	entries, err := entryRead(ctx, revision)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]awfgit.TreeEntry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	configPath := config.DirName + "/config.yaml"
	configEntry, found := byPath[configPath]
	if !found {
		return emptyRevisionState(), nil
	}
	controls := []string{configPath}
	if _, ok := byPath[config.DirName+"/awf.lock"]; ok {
		controls = append(controls, config.DirName+"/awf.lock")
	}
	blobs, err := blobRead(ctx, revision, controls)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	controlSelection, err := snapshot.NewSelectionFromBlobs(blobs)
	if err != nil {
		return nil, err
	}
	configFile, ok := controlSelection.Lookup(configPath)
	if !ok || !configFile.Scannable() || configEntry.Mode == awfgit.BlobSymlink {
		return revisionStateWithConfigError(fmt.Errorf("%s/config.yaml is not a scannable file", config.DirName)), nil
	}
	lock, lockFound, lockErr := auditLockFromSelection(controlSelection)
	if lockErr != nil {
		return revisionStateFromControls(root, controlSelection), nil //nolint:nilerr // preserve malformed committed controls as lazy policy warnings, not fatal audit failure
	}
	cfg, configErr := auditConfig(root, controlSelection, lock)
	if configErr != nil {
		return revisionStateFromControls(root, controlSelection), nil //nolint:nilerr // preserve malformed committed controls as lazy policy warnings, not fatal audit failure
	}
	authorityPaths := selectedAuthorityPaths(entries, cfg.DocsDir)
	// Authority bytes are deliberately not read with controls. Keep only the
	// exact path selection in this light state; the one-shot heavy loader drops
	// it immediately after materialization.
	return revisionStateFromAuthority(cfg, lock, lockFound, func() (currentstate.Universe, error) {
		blobs, err := blobRead(ctx, revision, authorityPaths)
		if err != nil {
			return currentstate.Universe{}, err
		}
		if err := ctx.Err(); err != nil {
			return currentstate.Universe{}, err
		}
		authority, err := snapshot.NewSelectionFromBlobs(blobs)
		if err != nil {
			return currentstate.Universe{}, err
		}
		allowed := make(map[string]bool, len(authorityPaths))
		for _, selected := range authorityPaths {
			allowed[selected] = true
		}
		for _, file := range authority.List() {
			if !allowed[file.Path] || !file.Scannable() {
				return currentstate.Universe{}, fmt.Errorf("selected authority %q is not a scannable file", file.Path)
			}
		}
		return currentstate.LoadUniverseFromSelection(authority, cfg)
	}), nil
}

func selectedAuthorityPaths(entries []awfgit.TreeEntry, docsDir string) []string {
	decisions := path.Join(filepath.ToSlash(docsDir), "decisions") + "/"
	var paths []string
	for _, entry := range entries {
		if (strings.HasPrefix(entry.Path, config.DirName+"/topics/metadata/") && strings.HasSuffix(entry.Path, ".yaml")) ||
			(strings.HasPrefix(entry.Path, config.DirName+"/topics/parts/") && strings.HasSuffix(entry.Path, "/current-state.md")) {
			paths = append(paths, entry.Path)
			continue
		}
		rel, ok := strings.CutPrefix(entry.Path, decisions)
		if ok && !strings.Contains(rel, "/") && strings.HasSuffix(rel, ".md") && !adr.IsReservedBasename(rel) {
			paths = append(paths, entry.Path)
		}
	}
	slices.Sort(paths)
	return paths
}

func emptyRevisionState() *revisionState {
	return &revisionState{lockReady: true, universeReady: true}
}

func revisionStateWithConfigError(err error) *revisionState {
	state := &revisionState{configReady: true, configErr: err}
	state.loadUniverse = func() (currentstate.Universe, error) {
		_, err := state.committedConfig()
		return currentstate.Universe{}, err
	}
	return state
}

func revisionStateFromControls(root string, selection *snapshot.Selection) *revisionState {
	state := &revisionState{}
	state.loadLock = func() (*manifest.Lock, bool, error) { return auditLockFromSelection(selection) }
	state.loadConfig = func() (*config.Config, error) {
		lock, _, err := state.lockEvidence()
		if err != nil {
			return nil, err
		}
		return auditConfig(root, selection, lock)
	}
	state.loadUniverse = func() (currentstate.Universe, error) {
		_, err := state.committedConfig()
		return currentstate.Universe{}, err
	}
	return state
}

func revisionStateFromAuthority(cfg *config.Config, lock *manifest.Lock, lockFound bool, load func() (currentstate.Universe, error)) *revisionState {
	return &revisionState{lockReady: true, lock: lock, lockFound: lockFound, configReady: true, config: cfg, loadUniverse: load}
}

func (h *historyOperation) replayTransition(ctx context.Context, commit replayCommit) ([]Finding, error) {
	var out []Finding
	afterState, err := h.stateForCommit(ctx, commit)
	if err != nil {
		if contextTermination(err) {
			return nil, fmt.Errorf("derive transition result %s: %w", commit.Hash, err)
		}
		out = append(out, transitionLoadWarning(commit, err))
		return out, nil
	}
	defer h.consumeUniverse(commit.Revision, afterState)
	after, err := h.currentState(afterState)
	if err != nil {
		if contextTermination(err) {
			return nil, fmt.Errorf("derive transition result current state %s: %w", commit.Hash, err)
		}
		out = append(out, transitionLoadWarning(commit, err))
		return out, nil
	}
	before := currentstate.Universe{}
	if len(commit.Parents) > 0 {
		beforeState, loadErr := h.state(ctx, commit.Parents[0])
		if loadErr != nil {
			if contextTermination(loadErr) {
				return nil, fmt.Errorf("derive transition first parent %s: %w", commit.Hash, loadErr)
			}
			out = append(out, transitionLoadWarning(commit, loadErr))
			return out, nil
		}
		defer h.consumeUniverse(commit.Parents[0], beforeState)
		before, err = h.currentState(beforeState)
		if err != nil {
			if contextTermination(err) {
				return nil, fmt.Errorf("derive transition first-parent current state %s: %w", commit.Hash, err)
			}
			out = append(out, transitionLoadWarning(commit, err))
			return out, nil
		}
	}
	mode := currentstate.AuthoredCommit
	if commit.IsMerge {
		mode = currentstate.MergeAggregate
	}
	for _, transition := range currentstate.CheckPair(before, after, mode) {
		out = append(out, replayFinding(severity.Error, currentStateTransitionRule, commit, transition.Message))
	}
	return out, nil
}

func contextTermination(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func transitionLoadWarning(commit replayCommit, err error) Finding {
	return replayFinding(severity.Warn, currentStateTransitionRule, commit,
		"could not load the current-state universes for this commit: "+err.Error())
}

// staleMergeFindings applies the live stale-merge evidence policy to historical
// merge commits once their result lock reaches schema generation 31.
func (h *historyOperation) replayStale(ctx context.Context, commit replayCommit) ([]Finding, error) {
	if !commit.IsMerge {
		return nil, nil
	}
	// Every merge role was reserved during light planning. Discharge it even
	// when schema controls make stale qualification inapplicable.
	consume := func(revision string, state *revisionState) {
		h.consumeSources(revision, state)
		h.consumeUniverse(revision, state)
	}
	for _, revision := range append([]string{commit.Revision}, commit.Parents...) {
		defer func(revision string) {
			if result, ok := h.store.keys[revision]; ok {
				consume(revision, result.state)
			}
		}(revision)
	}
	var findings []Finding
	resultState, err := h.stateForCommit(ctx, commit)
	if err != nil {
		return nil, fmt.Errorf("load merge result %s: %w", commit.Hash, err)
	}
	lock, found, err := resultState.lockEvidence()
	if err != nil {
		return nil, fmt.Errorf("load merge result lock %s: %w", commit.Hash, err)
	}
	if !found || lock.SchemaVersion < 31 {
		return nil, nil
	}
	current, _ := adr.FormatAtGeneration(lock.SchemaVersion)
	if len(commit.Parents) < 2 {
		return nil, fmt.Errorf("merge %s has fewer than two parents", commit.Hash)
	}
	result, err := h.currentState(resultState)
	if err != nil {
		return nil, fmt.Errorf("load merge result current state %s: %w", commit.Hash, err)
	}
	firstState, err := h.state(ctx, commit.Parents[0])
	if err != nil {
		return nil, fmt.Errorf("load merge first parent %s: %w", commit.Hash, err)
	}
	first, err := h.currentState(firstState)
	if err != nil {
		return nil, fmt.Errorf("load merge first parent current state %s: %w", commit.Hash, err)
	}
	incoming := make([]currentstate.Universe, len(commit.Parents)-1)
	for i, revision := range commit.Parents[1:] {
		incomingState, loadErr := h.state(ctx, revision)
		if loadErr != nil {
			return nil, fmt.Errorf("load merge incoming parent %s: %w", commit.Hash, loadErr)
		}
		incoming[i], err = h.currentState(incomingState)
		if err != nil {
			return nil, fmt.Errorf("load merge incoming parent current state %s: %w", commit.Hash, err)
		}
	}
	authorizations, err := commitmsg.ParseAuthorizations(commitmsg.Clean([]byte(commit.Message)), func(value string) bool {
		return value == "legacy" || adr.KnownFormatMarker(value)
	})
	if err != nil {
		syntax, syntaxErr := staleAuthorizationSyntax(err)
		if syntaxErr != nil { // coverage-ignore: ParseAuthorizations returns only *SyntaxError; the checked fallback protects future implementations
			return nil, syntaxErr
		}
		findings = append(findings, replayFinding(severity.Error, "stale-merge-authorization", commit,
			fmt.Sprintf("malformed reserved trailer at cleaned line %d: %s", syntax.Line, syntax.Reason)))
		return findings, nil
	}
	allowed := map[string]bool{}
	for _, authorization := range authorizations {
		allowed[authorization.Version] = true
	}
	for _, qualification := range currentstate.QualifyIncoming(first, result, incoming, current) {
		identity := "ADR-" + qualification.Introduction.Identity
		if !qualification.Qualified {
			findings = append(findings, replayFinding(severity.Error, "stale-merge-authorization", commit,
				"unqualified incoming-parent record "+identity))
			continue
		}
		version := adr.FormatMarker(qualification.Introduction.Format)
		if version == "" {
			version = "legacy"
		}
		if !allowed[version] {
			findings = append(findings, replayFinding(severity.Error, "stale-merge-authorization", commit,
				"missing authorization version "+version+" for "+identity))
		}
	}
	return findings, nil
}

func replayFinding(rank severity.Rank, rule string, commit replayCommit, detail string) Finding {
	return Finding{Severity: rank, Rule: rule, Commit: commit.Hash, Subject: commit.Subject, Detail: detail}
}

func staleAuthorizationSyntax(err error) (*commitmsg.SyntaxError, error) {
	var syntax *commitmsg.SyntaxError
	if !errors.As(err, &syntax) {
		return nil, fmt.Errorf("parse stale merge authorizations: %w", err)
	}
	return syntax, nil
}

func auditLockFromSelection(selection *snapshot.Selection) (*manifest.Lock, bool, error) {
	file, ok := selection.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, false, nil
	}
	if !file.Scannable() {
		return nil, true, fmt.Errorf("%s/awf.lock is not a scannable file", config.DirName)
	}
	lock, err := manifest.Parse(file.Bytes)
	return lock, true, err
}

func auditConfig(root string, selection *snapshot.Selection, lock *manifest.Lock) (*config.Config, error) {
	file, ok := selection.Lookup(config.DirName + "/config.yaml")
	if !ok {
		//nolint:nilnil // an absent committed config means an empty historical universe, not a load failure
		return nil, nil
	}
	if !file.Scannable() {
		return nil, fmt.Errorf("%s/config.yaml is not a scannable file", config.DirName)
	}
	schema := migrate.Current()
	if lock != nil {
		schema = lock.SchemaVersion
	}
	data, err := migrate.ConfigForCurrentSchema(file.Bytes, schema)
	if err != nil {
		return nil, err
	}
	cfg, err := config.ParseTree(config.RootDir(root), data, auditSelectionReader{selection})
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

type auditSelectionReader struct{ selection *snapshot.Selection }

func (r auditSelectionReader) ReadFile(path string) ([]byte, bool) {
	file, ok := r.selection.Lookup(config.DirName + "/" + filepath.ToSlash(path))
	if !ok || !file.Scannable() {
		return nil, false
	}
	return slices.Clone(file.Bytes), true
}

func (r auditSelectionReader) Paths(prefix string) []string {
	full := config.DirName + "/" + filepath.ToSlash(prefix)
	var paths []string
	for _, file := range r.selection.List() {
		if file.Scannable() && strings.HasPrefix(file.Path, full) {
			paths = append(paths, strings.TrimPrefix(file.Path, config.DirName+"/"))
		}
	}
	return paths
}
