package audit

import (
	"context"
	"errors"
	"fmt"
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

type rangeCollector func(context.Context, string, string) ([]awfgit.Commit, error)
type revisionLoader func(context.Context, string) (*revisionState, error)
type firstParentPaths func(context.Context, string) ([]string, error)
type liveEvaluator func(context.Context) ([]Finding, error)

// historyOperation owns every historical input and derived revision state for
// one audit invocation. Nothing on it is retained by Project or shared with a
// later invocation.
type historyOperation struct {
	commits          []awfgit.Commit
	inputs           Inputs
	loadRevision     revisionLoader
	firstParentPaths firstParentPaths
	live             liveEvaluator
	states           map[string]revisionResult
}

type revisionResult struct {
	state *revisionState
	err   error
}

// revisionState is one lazily parsed committed revision. The committed snapshot
// is loaded once by the operation; lock and current-state parsing are separately
// lazy so a pre-policy merge can inspect only its schema boundary.
type revisionState struct {
	loadLock     func() (*manifest.Lock, bool, error)
	loadUniverse func() (currentstate.Universe, error)

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

func newHistoryOperation(ctx context.Context, base, head string, in Inputs, collect rangeCollector, load revisionLoader, paths firstParentPaths, live liveEvaluator) (*historyOperation, error) {
	commits, err := collect(ctx, base, head)
	if err != nil {
		return nil, fmt.Errorf("collect audit range: %w", err)
	}
	return newHistoryOperationWithRelevance(commits, in, load, paths, live), nil
}

func newHistoryOperationWithRelevance(commits []awfgit.Commit, in Inputs, load revisionLoader, paths firstParentPaths, live liveEvaluator) *historyOperation {
	return &historyOperation{
		commits:          slices.Clone(commits),
		inputs:           in,
		loadRevision:     load,
		firstParentPaths: paths,
		live:             live,
		states:           map[string]revisionResult{},
	}
}

func (h *historyOperation) run(ctx context.Context) ([]Finding, error) {
	findings := evaluate(h.commits, h.inputs)
	stale, err := h.staleMergeFindings(ctx)
	if err != nil {
		return nil, err
	}
	findings = append(findings, stale...)
	live, err := h.live(ctx)
	if err != nil {
		return nil, fmt.Errorf("evaluate live cleanliness: %w", err)
	}
	findings = append(findings, live...)
	transitions, err := h.transitionFindings(ctx)
	if err != nil {
		return nil, err
	}
	return append(findings, transitions...), nil
}

func (h *historyOperation) state(ctx context.Context, revision string) (*revisionState, error) {
	if cached, ok := h.states[revision]; ok {
		return cached.state, cached.err
	}
	state, err := h.loadRevision(ctx, revision)
	if err == nil && state == nil {
		err = errors.New("revision loader returned no state")
	}
	h.states[revision] = revisionResult{state: state, err: err}
	return state, err
}

// stateForCommit reuses the first-parent state only after committed path
// evidence proves this revision cannot affect the reduced policy authority.
func (h *historyOperation) stateForCommit(ctx context.Context, commit awfgit.Commit) (*revisionState, error) {
	if cached, ok := h.states[commit.Revision]; ok {
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
		paths = changedPaths(commit.Changes)
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
	h.states[commit.Revision] = h.states[commit.Parents[0]]
	return parent, nil
}

func changedPaths(changes []awfgit.FileChange) []string {
	paths := make([]string, 0, len(changes)*2)
	for _, change := range changes {
		if change.OldPath != "" {
			paths = append(paths, change.OldPath)
		}
		if change.Path != "" {
			paths = append(paths, change.Path)
		}
	}
	return paths
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
		s.universe, s.universeErr = s.loadUniverse()
		s.universeReady = true
	}
	return s.universe, s.universeErr
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

func loadCompleteRevision(ctx context.Context, root string, repo *awfgit.Repo, revision string) (*revisionState, error) {
	tree, err := snapshot.CommitTree(ctx, repo, revision)
	if err != nil {
		return nil, err
	}
	return revisionStateFromTree(root, tree), nil
}

func revisionStateFromTree(root string, tree *snapshot.Tree) *revisionState {
	state := &revisionState{}
	state.loadLock = func() (*manifest.Lock, bool, error) { return auditLockFromTree(tree) }
	state.loadConfig = func() (*config.Config, error) {
		lock, _, err := state.lockEvidence()
		if err != nil {
			return nil, err
		}
		return auditConfig(root, tree, lock)
	}
	state.loadUniverse = func() (currentstate.Universe, error) {
		cfg, err := state.committedConfig()
		if err != nil || cfg == nil {
			return currentstate.Universe{}, err
		}
		return currentstate.LoadUniverseFromTree(tree, cfg)
	}
	return state
}

func (h *historyOperation) transitionFindings(ctx context.Context) ([]Finding, error) {
	var out []Finding
	for _, commit := range h.commits {
		afterState, err := h.stateForCommit(ctx, commit)
		if err != nil {
			if contextTermination(err) {
				return nil, fmt.Errorf("derive transition result %s: %w", commit.Hash, err)
			}
			out = append(out, transitionLoadWarning(commit, err))
			continue
		}
		after, err := afterState.currentState()
		if err != nil {
			if contextTermination(err) {
				return nil, fmt.Errorf("derive transition result current state %s: %w", commit.Hash, err)
			}
			out = append(out, transitionLoadWarning(commit, err))
			continue
		}
		before := currentstate.Universe{}
		if len(commit.Parents) > 0 {
			beforeState, loadErr := h.state(ctx, commit.Parents[0])
			if loadErr != nil {
				if contextTermination(loadErr) {
					return nil, fmt.Errorf("derive transition first parent %s: %w", commit.Hash, loadErr)
				}
				out = append(out, transitionLoadWarning(commit, loadErr))
				continue
			}
			before, err = beforeState.currentState()
			if err != nil {
				if contextTermination(err) {
					return nil, fmt.Errorf("derive transition first-parent current state %s: %w", commit.Hash, err)
				}
				out = append(out, transitionLoadWarning(commit, err))
				continue
			}
		}
		mode := currentstate.AuthoredCommit
		if commit.IsMerge {
			mode = currentstate.MergeAggregate
		}
		for _, transition := range currentstate.CheckPair(before, after, mode) {
			out = append(out, finding(severity.Error, currentStateTransitionRule, commit, transition.Message))
		}
	}
	return out, nil
}

func contextTermination(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func transitionLoadWarning(commit awfgit.Commit, err error) Finding {
	return finding(severity.Warn, currentStateTransitionRule, commit,
		"could not load the current-state universes for this commit: "+err.Error())
}

// staleMergeFindings applies the live stale-merge evidence policy to historical
// merge commits once their result lock reaches schema generation 31.
func (h *historyOperation) staleMergeFindings(ctx context.Context) ([]Finding, error) {
	var findings []Finding
	for _, commit := range h.commits {
		if !commit.IsMerge {
			continue
		}
		resultState, err := h.stateForCommit(ctx, commit)
		if err != nil {
			return nil, fmt.Errorf("load merge result %s: %w", commit.Hash, err)
		}
		lock, found, err := resultState.lockEvidence()
		if err != nil {
			return nil, fmt.Errorf("load merge result lock %s: %w", commit.Hash, err)
		}
		if !found || lock.SchemaVersion < 31 {
			continue
		}
		current, _ := adr.FormatAtGeneration(lock.SchemaVersion)
		if len(commit.Parents) < 2 {
			return nil, fmt.Errorf("merge %s has fewer than two parents", commit.Hash)
		}
		result, err := resultState.currentState()
		if err != nil {
			return nil, fmt.Errorf("load merge result current state %s: %w", commit.Hash, err)
		}
		firstState, err := h.state(ctx, commit.Parents[0])
		if err != nil {
			return nil, fmt.Errorf("load merge first parent %s: %w", commit.Hash, err)
		}
		first, err := firstState.currentState()
		if err != nil {
			return nil, fmt.Errorf("load merge first parent current state %s: %w", commit.Hash, err)
		}
		incoming := make([]currentstate.Universe, len(commit.Parents)-1)
		for i, revision := range commit.Parents[1:] {
			incomingState, loadErr := h.state(ctx, revision)
			if loadErr != nil {
				return nil, fmt.Errorf("load merge incoming parent %s: %w", commit.Hash, loadErr)
			}
			incoming[i], err = incomingState.currentState()
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
			findings = append(findings, finding(severity.Error, "stale-merge-authorization", commit,
				fmt.Sprintf("malformed reserved trailer at cleaned line %d: %s", syntax.Line, syntax.Reason)))
			continue
		}
		allowed := map[string]bool{}
		for _, authorization := range authorizations {
			allowed[authorization.Version] = true
		}
		for _, qualification := range currentstate.QualifyIncoming(first, result, incoming, current) {
			identity := "ADR-" + qualification.Introduction.Identity
			if !qualification.Qualified {
				findings = append(findings, finding(severity.Error, "stale-merge-authorization", commit,
					"unqualified incoming-parent record "+identity))
				continue
			}
			version := adr.FormatMarker(qualification.Introduction.Format)
			if version == "" {
				version = "legacy"
			}
			if !allowed[version] {
				findings = append(findings, finding(severity.Error, "stale-merge-authorization", commit,
					"missing authorization version "+version+" for "+identity))
			}
		}
	}
	return findings, nil
}

func staleAuthorizationSyntax(err error) (*commitmsg.SyntaxError, error) {
	var syntax *commitmsg.SyntaxError
	if !errors.As(err, &syntax) {
		return nil, fmt.Errorf("parse stale merge authorizations: %w", err)
	}
	return syntax, nil
}

func auditLockFromTree(tree *snapshot.Tree) (*manifest.Lock, bool, error) {
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, false, nil
	}
	if !file.Scannable() {
		return nil, true, fmt.Errorf("%s/awf.lock is not a scannable file", config.DirName)
	}
	lock, err := manifest.Parse(file.Bytes)
	return lock, true, err
}

func auditConfig(root string, tree *snapshot.Tree, lock *manifest.Lock) (*config.Config, error) {
	file, ok := tree.Lookup(config.DirName + "/config.yaml")
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
	cfg, err := config.ParseTree(config.RootDir(root), data, auditSnapshotReader{tree})
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

type auditSnapshotReader struct{ tree *snapshot.Tree }

func (r auditSnapshotReader) ReadFile(path string) ([]byte, bool) {
	file, ok := r.tree.Lookup(config.DirName + "/" + filepath.ToSlash(path))
	if !ok || !file.Scannable() {
		return nil, false
	}
	return slices.Clone(file.Bytes), true
}

func (r auditSnapshotReader) Paths(prefix string) []string {
	full := config.DirName + "/" + filepath.ToSlash(prefix)
	var paths []string
	for _, file := range r.tree.List() {
		if file.Scannable() && strings.HasPrefix(file.Path, full) {
			paths = append(paths, strings.TrimPrefix(file.Path, config.DirName+"/"))
		}
	}
	return paths
}
