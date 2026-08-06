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

func newReplayGraph(ctx context.Context, commits []replayCommit) (replayGraph, error) {
	byRevision := make(map[string]replayCommit, len(commits))
	for _, commit := range commits {
		if err := ctx.Err(); err != nil {
			return replayGraph{}, err
		}
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
		if err := ctx.Err(); err != nil {
			return replayGraph{}, err
		}
		for _, parent := range commit.Parents {
			if err := ctx.Err(); err != nil {
				return replayGraph{}, err
			}
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
		if err := ctx.Err(); err != nil {
			return replayGraph{}, err
		}
		if remainingChildren[revision] == 0 {
			ready = append(ready, revision)
		}
	}
	slices.Sort(ready)
	schedule := make([]replayCommit, 0, len(commits))
	for len(ready) > 0 {
		if err := ctx.Err(); err != nil {
			return replayGraph{}, err
		}
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
}

type revisionResult struct {
	state *revisionState
	err   error
}

type revisionEntry struct {
	result       revisionResult
	keys         int
	lightUses    int
	sourceUses   int
	universeUses int
	heavyLive    bool
}

type revisionKey struct {
	entry        *revisionEntry
	lightUses    int
	sourceUses   int
	universeUses int
}

// revisionStore separates revision keys from canonical entries resolved before
// replay. Counters model logical ownership rather than garbage-collector timing.
type revisionStore struct {
	keys           map[string]*revisionKey
	entries        map[*revisionEntry]bool
	currentHeavy   int
	highWaterHeavy int
}

func newRevisionStore() revisionStore {
	return revisionStore{
		keys:    map[string]*revisionKey{},
		entries: map[*revisionEntry]bool{},
	}
}

func (s *revisionStore) addDistinct(revision string, result revisionResult) *revisionEntry {
	entry := &revisionEntry{result: result}
	s.entries[entry] = true
	s.addKey(revision, entry)
	return entry
}

func (s *revisionStore) addKey(revision string, entry *revisionEntry) {
	s.keys[revision] = &revisionKey{entry: entry}
	entry.keys++
}

func (s *revisionStore) retainHeavy(entry *revisionEntry) {
	if entry.heavyLive {
		return
	}
	entry.heavyLive = true
	s.currentHeavy++
	if s.currentHeavy > s.highWaterHeavy {
		s.highWaterHeavy = s.currentHeavy
	}
}

func (s *revisionStore) releaseHeavy(entry *revisionEntry) {
	if !entry.heavyLive {
		return
	}
	entry.heavyLive = false
	s.currentHeavy--
	if state := entry.result.state; state != nil {
		state.heavyLive = false
		state.heavyMaterialized = false
		state.universeReady = false
		state.universe = currentstate.Universe{}
		state.universeErr = nil
		state.loadUniverse = nil
	}
}

func (s *revisionStore) releaseEntry(entry *revisionEntry) {
	s.releaseHeavy(entry)
	state := entry.result.state
	if state != nil {
		state.loadLock = nil
		state.loadConfig = nil
		state.loadUniverse = nil
		state.lockReady = false
		state.lock = nil
		state.lockFound = false
		state.lockErr = nil
		state.configReady = false
		state.config = nil
		state.configErr = nil
		state.universeReady = false
		state.universe = currentstate.Universe{}
		state.universeErr = nil
	}
	entry.result = revisionResult{}
	delete(s.entries, entry)
}

func (s *revisionStore) releaseAll() {
	for revision := range s.keys {
		delete(s.keys, revision)
	}
	for entry := range s.entries {
		entry.keys = 0
		entry.lightUses = 0
		entry.sourceUses = 0
		entry.universeUses = 0
		s.releaseEntry(entry)
	}
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
	graph, err := newReplayGraph(ctx, h.commits)
	if err != nil {
		return nil, fmt.Errorf("build historical replay graph: %w", err)
	}
	defer h.store.releaseAll()
	if err := h.planRevisionOwnership(ctx, graph.schedule); err != nil {
		return nil, err
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
	if key, ok := h.store.keys[revision]; ok {
		return key.entry.result.state, key.entry.result.err
	}
	entry := h.loadDistinctRevision(ctx, revision)
	return entry.result.state, entry.result.err
}

func (h *historyOperation) loadDistinctRevision(ctx context.Context, revision string) *revisionEntry {
	state, err := h.loadRevision(ctx, revision)
	if err == nil && state == nil {
		err = errors.New("revision loader returned no state")
	}
	return h.store.addDistinct(revision, revisionResult{state: state, err: err})
}

func (h *historyOperation) planRevisionOwnership(ctx context.Context, schedule []replayCommit) error {
	byRevision := make(map[string]replayCommit, len(schedule))
	for _, commit := range schedule {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("plan revision ownership: %w", err)
		}
		byRevision[commit.Revision] = commit
	}
	resolving := map[string]bool{}
	for _, commit := range schedule {
		entry, err := h.resolveRevision(ctx, commit.Revision, byRevision, resolving)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("resolve revision %s: %w", commit.Hash, err)
		}
		if contextTermination(entry.result.err) {
			return fmt.Errorf("resolve revision %s: %w", commit.Hash, entry.result.err)
		}
		for _, parent := range commit.Parents {
			entry, err := h.resolveRevision(ctx, parent, byRevision, resolving)
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("resolve parent of %s: %w", commit.Hash, err)
			}
			if contextTermination(entry.result.err) {
				return fmt.Errorf("resolve parent of %s: %w", commit.Hash, entry.result.err)
			}
		}
	}
	return nil
}

func (h *historyOperation) resolveRevision(ctx context.Context, revision string, byRevision map[string]replayCommit, resolving map[string]bool) (*revisionEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("resolve revision %s: %w", revision, err)
	}
	if key, ok := h.store.keys[revision]; ok {
		return key.entry, nil
	}
	commit, selected := byRevision[revision]
	if !selected {
		return h.loadDistinctRevision(ctx, revision), nil
	}
	if resolving[revision] { // coverage-ignore: newReplayGraph rejects selected cycles before ownership planning
		return nil, fmt.Errorf("cycle while resolving replay revision %s", revision)
	}
	resolving[revision] = true
	defer delete(resolving, revision)
	return h.resolveCommit(ctx, commit, byRevision, resolving)
}

// resolveCommit reuses the recursively resolved first-parent entry only after
// committed light controls and changed paths prove irrelevance.
func (h *historyOperation) resolveCommit(ctx context.Context, commit replayCommit, byRevision map[string]replayCommit, resolving map[string]bool) (*revisionEntry, error) {
	if len(commit.Parents) == 0 || h.firstParentPaths == nil {
		return h.loadDistinctRevision(ctx, commit.Revision), nil
	}
	parentEntry, err := h.resolveRevision(ctx, commit.Parents[0], byRevision, resolving)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("resolve first parent of %s: %w", commit.Hash, err)
	}
	parent, parentErr := parentEntry.result.state, parentEntry.result.err
	if parentErr != nil || parent == nil {
		if contextTermination(parentErr) {
			return nil, fmt.Errorf("derive first-parent state for %s: %w", commit.Hash, parentErr)
		}
		return h.loadDistinctRevision(ctx, commit.Revision), nil
	}
	docsDir, err := parent.committedDocsDir()
	if err != nil {
		if contextTermination(err) {
			return nil, fmt.Errorf("derive first-parent configuration for %s: %w", commit.Hash, err)
		}
		return h.loadDistinctRevision(ctx, commit.Revision), nil
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
		return h.loadDistinctRevision(ctx, commit.Revision), nil
	}
	if policyRelevant(paths, docsDir) {
		return h.loadDistinctRevision(ctx, commit.Revision), nil
	}
	h.store.addKey(commit.Revision, parentEntry)
	return parentEntry, nil
}

// stateForCommit retains the focused test seam while production run resolves
// the complete alias graph before reserving or executing replay consumers.
func (h *historyOperation) stateForCommit(ctx context.Context, commit replayCommit) (*revisionState, error) {
	byRevision := make(map[string]replayCommit, len(h.commits)+1)
	for _, candidate := range h.commits {
		byRevision[candidate.Revision] = candidate
	}
	byRevision[commit.Revision] = commit
	entry, err := h.resolveRevision(ctx, commit.Revision, byRevision, map[string]bool{})
	if err != nil { // coverage-ignore: production validates the graph before this focused compatibility seam runs
		return nil, err
	}
	return entry.result.state, entry.result.err
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
		if s.loadLock != nil {
			s.lock, s.lockFound, s.lockErr = s.loadLock()
		}
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

func (h *historyOperation) currentState(revision string, state *revisionState) (currentstate.Universe, error) {
	universe, err := state.currentState()
	if state.heavyMaterialized {
		if key := h.store.keys[revision]; key != nil {
			h.store.retainHeavy(key.entry)
			state.heavyLive = key.entry.heavyLive
			if key.entry.sourceUses == 0 {
				state.universe.Sources = nil
				universe.Sources = nil
			}
		}
	}
	return universe, err
}

// reserveConsumers attaches every fixed replay role to its already-resolved
// canonical entry before heavy authority is materialized.
func (h *historyOperation) reserveConsumers(schedule []replayCommit) {
	for _, commit := range schedule {
		h.reserveUse(commit.Revision, false) // transition result
		if len(commit.Parents) > 0 {
			h.reserveUse(commit.Parents[0], false) // transition first parent
		}
		if commit.IsMerge {
			h.reserveUse(commit.Revision, true) // stale result
			for _, revision := range commit.Parents {
				h.reserveUse(revision, true) // ordered stale parent
			}
		}
	}
}

func (h *historyOperation) reserveUse(revision string, source bool) {
	key := h.store.keys[revision]
	key.lightUses++
	key.universeUses++
	key.entry.lightUses++
	key.entry.universeUses++
	if source {
		key.sourceUses++
		key.entry.sourceUses++
	}
}

func (h *historyOperation) consumeUse(revision string, source bool) {
	key := h.store.keys[revision]
	if key == nil || key.lightUses == 0 || key.universeUses == 0 {
		return
	}
	entry := key.entry
	key.lightUses--
	key.universeUses--
	entry.lightUses--
	entry.universeUses--
	if source && key.sourceUses > 0 {
		key.sourceUses--
		entry.sourceUses--
		if entry.sourceUses == 0 && entry.result.state != nil {
			entry.result.state.universe.Sources = nil
		}
	}
	if entry.universeUses == 0 {
		h.store.releaseHeavy(entry)
	}
	if key.lightUses == 0 {
		delete(h.store.keys, revision)
		entry.keys--
	}
	if entry.lightUses == 0 && entry.sourceUses == 0 && entry.universeUses == 0 && entry.keys == 0 {
		h.store.releaseEntry(entry)
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
	if err := ctx.Err(); err != nil {
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
	lock, lockFound, lockErr := auditLockFromSelection(controlSelection)
	configFile, ok := controlSelection.Lookup(configPath)
	if !ok || !configFile.Scannable() || configEntry.Mode == awfgit.BlobSymlink {
		configErr := fmt.Errorf("%s/config.yaml is not a scannable file", config.DirName)
		return revisionStateFromControlOutcome(lock, lockFound, lockErr, configErr), nil
	}
	if lockErr != nil {
		return revisionStateFromControlOutcome(lock, lockFound, lockErr, lockErr), nil
	}
	cfg, configErr := auditConfig(root, controlSelection, lock)
	if configErr != nil {
		return revisionStateFromControlOutcome(lock, lockFound, nil, configErr), nil
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

func revisionStateFromControlOutcome(lock *manifest.Lock, lockFound bool, lockErr, configErr error) *revisionState {
	return &revisionState{
		lockReady:     true,
		lock:          lock,
		lockFound:     lockFound,
		lockErr:       lockErr,
		configReady:   true,
		config:        nil,
		configErr:     configErr,
		universeReady: true,
		universeErr:   configErr,
	}
}

func revisionStateFromAuthority(cfg *config.Config, lock *manifest.Lock, lockFound bool, load func() (currentstate.Universe, error)) *revisionState {
	return &revisionState{lockReady: true, lock: lock, lockFound: lockFound, configReady: true, config: cfg, loadUniverse: load}
}

func (h *historyOperation) replayTransition(ctx context.Context, commit replayCommit) ([]Finding, error) {
	defer h.consumeUse(commit.Revision, false)
	if len(commit.Parents) > 0 {
		defer h.consumeUse(commit.Parents[0], false)
	}
	var out []Finding
	afterState, err := h.stateForCommit(ctx, commit)
	if err != nil {
		if contextTermination(err) { // coverage-ignore: planning rejects cached context errors before replay
			return nil, fmt.Errorf("derive transition result %s: %w", commit.Hash, err)
		}
		out = append(out, transitionLoadWarning(commit, err))
		return out, nil
	}
	after, err := h.currentState(commit.Revision, afterState)
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
			if contextTermination(loadErr) { // coverage-ignore: planning rejects cached context errors before replay
				return nil, fmt.Errorf("derive transition first parent %s: %w", commit.Hash, loadErr)
			}
			out = append(out, transitionLoadWarning(commit, loadErr))
			return out, nil
		}
		before, err = h.currentState(commit.Parents[0], beforeState)
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
	// when schema controls make stale qualification inapplicable or fails.
	for _, revision := range append([]string{commit.Revision}, commit.Parents...) {
		defer h.consumeUse(revision, true)
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
	result, err := h.currentState(commit.Revision, resultState)
	if err != nil {
		return nil, fmt.Errorf("load merge result current state %s: %w", commit.Hash, err)
	}
	firstState, err := h.state(ctx, commit.Parents[0])
	if err != nil {
		return nil, fmt.Errorf("load merge first parent %s: %w", commit.Hash, err)
	}
	first, err := h.currentState(commit.Parents[0], firstState)
	if err != nil {
		return nil, fmt.Errorf("load merge first parent current state %s: %w", commit.Hash, err)
	}
	incoming := make([]currentstate.Universe, len(commit.Parents)-1)
	for i, revision := range commit.Parents[1:] {
		incomingState, loadErr := h.state(ctx, revision)
		if loadErr != nil {
			return nil, fmt.Errorf("load merge incoming parent %s: %w", commit.Hash, loadErr)
		}
		incoming[i], err = h.currentState(revision, incomingState)
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
