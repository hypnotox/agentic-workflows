package project

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// currentStateTransitionRule names the range transition check in audit output.
const currentStateTransitionRule = "current-state-transition"

// CurrentStateReport is the routed outcome of a current-state check over one
// snapshot: the static ADR-to-claim handshake findings (all blocking), staged
// older-format introductions awaiting commit-message evidence, and the
// coverage/fan-out findings, which carry ranks fixed in code rather than
// configured - coverage at error, fan-out at warn (ADR-0183). Findings and Notes
// split the report into blocking lines and non-failing note lines so the command
// layer never re-derives the routing.
type CurrentStateReport struct {
	Static      []currentstate.Finding
	Provisional []currentstate.Introduction
	Coverage    []topic.CoverageFinding
}

// Findings returns the blocking lines: every static handshake finding and every
// coverage/fan-out finding at error severity.
func (r CurrentStateReport) Findings() []string {
	var out []string
	for _, f := range r.Static {
		out = append(out, f.Message)
	}
	for _, c := range r.Coverage {
		if c.Severity == severity.Error {
			out = append(out, coverageLine(c))
		}
	}
	return out
}

// Notes returns the non-failing lines: provisional older-format introductions
// and coverage/fan-out findings at warn. Provisional introductions are not
// findings because the staged boundary lacks definitive merge-parent and
// message evidence; every independently derivable finding remains blocking.
func (r CurrentStateReport) Notes() []string {
	out := make([]string, 0, len(r.Provisional))
	for _, introduction := range r.Provisional {
		marker := adr.FormatMarker(introduction.Format)
		if marker == "" {
			marker = "legacy"
		}
		out = append(out, fmt.Sprintf("provisional older-format ADR-%s (%s) requires commit-msg qualification", introduction.Identity, marker))
	}
	for _, c := range r.Coverage {
		if c.Severity == severity.Warn {
			out = append(out, coverageLine(c))
		}
	}
	return out
}

// coverageLine renders one coverage or fan-out finding as a stable one-line
// message shared by the blocking and note channels.
func coverageLine(c topic.CoverageFinding) string {
	if c.Kind == topic.Fanout {
		return fmt.Sprintf("fan-out: %s is matched by %d path-scoped topics", c.Path, c.Topics)
	}
	return fmt.Sprintf("uncovered: %s is owned by domain %s with no scoped topic", c.Path, c.Domain)
}

// workingState is one loaded working-tree current-state universe: the parsed
// ADR/topic view, the Tree it came from, and the lock.
// It is the shared substrate for CheckCurrentState and CurrentStateInvariants,
// which each read exactly one working Tree so a check and a report never mix a
// working and an index universe.
type workingState struct {
	Loaded currentstate.Loaded
	Tree   *snapshot.Tree
	Lock   *manifest.Lock
	Cfg    *config.Config
}

// workingCurrentState loads the working-tree ADR/topic view and recorded gaps.
func (p *Project) workingCurrentState(ctx context.Context) (workingState, error) {
	tree, err := p.workingTree(ctx)
	if err != nil {
		return workingState{}, err
	}
	lock, _, err := optionalLockFromTree(tree)
	if err != nil {
		return workingState{}, err
	}
	loaded, cfg, err := loadTreeCurrentState(p.Root, tree, lock)
	if err != nil {
		return workingState{}, err
	}
	if cfg == nil { // coverage-ignore: Project.Open already required config; only a concurrent deletion after path enumeration can remove it
		return workingState{}, fmt.Errorf("working snapshot has no %s/config.yaml", config.DirName)
	}
	return workingState{Loaded: loaded, Tree: tree, Lock: lock, Cfg: cfg}, nil
}

// CheckCurrentState loads the working-tree current-state view and runs the
// static ADR-to-claim handshake and the coverage/fan-out evaluator over it
// (ADR-0135, ADR-0134). It reads exactly one working Tree, so the two checks
// never mix a working and an index universe. Coverage and fan-out always
// evaluate, whether or not the project configures a currentState policy
// (ADR-0192).
func (p *Project) CheckCurrentState(ctx context.Context) (CurrentStateReport, error) {
	ws, err := p.workingCurrentState(ctx)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report := CurrentStateReport{
		Static: currentstate.Check(ws.Loaded.ADRs, ws.Loaded.Topics.All()),
	}
	report.Coverage = topic.EvaluateCoverage(ws.Loaded.Topics, eligiblePaths(ws.Tree, ws.Lock, ws.Cfg.ContextIgnore), coveragePolicy(ws.Cfg.CurrentState))
	return report, nil
}

// CheckStagedRoot validates the staged current-state transition without opening
// working-tree project configuration. The staged command must remain operable
// when a valid adopted index deliberately deletes or lacks the working config.
func CheckStagedRoot(ctx context.Context, root string) (CurrentStateReport, error) {
	p, err := openRootProject(root)
	if err != nil {
		return CurrentStateReport{}, err
	}
	return p.CheckStaged(ctx)
}

// CheckStaged loads the HEAD (before) and staged index (after) current-state
// universes and runs the snapshot-diff transition check between them plus the
// coverage/fan-out evaluator over the index (ADR-0135, ADR-0134). Both sides are
// committed or index universes, so a dirty working tree never affects the result.
// The before side is the empty universe on a repository with no commit yet, and
// the after config, policy, and eligible paths all come from the index tree so
// the staged check reads one universe. Coverage and fan-out always evaluate,
// whether or not the staged config declares a currentState policy (ADR-0192).
func (p *Project) CheckStaged(ctx context.Context) (CurrentStateReport, error) {
	afterTree, err := p.indexTree(ctx)
	if err != nil {
		return CurrentStateReport{}, err
	}
	afterLock, err := lockFromTree(afterTree)
	if err != nil {
		return CurrentStateReport{}, err
	}
	beforeTree, beforeLock, err := p.headTreeAndLock(ctx)
	if err != nil {
		return CurrentStateReport{}, err
	}
	if err := validateLockTransition(beforeTree, beforeLock, afterLock); err != nil {
		return CurrentStateReport{}, err
	}
	before, _, err := loadTreeCurrentState(p.Root, beforeTree, beforeLock)
	if err != nil {
		return CurrentStateReport{}, err
	}
	after, afterCfg, err := loadTreeCurrentState(p.Root, afterTree, afterLock)
	if err != nil {
		return CurrentStateReport{}, err
	}
	if afterCfg == nil {
		return CurrentStateReport{}, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	// A merge integrates a branch whose commits were each validated as they were
	// authored, so the pair carries several steps at once and takes the aggregate
	// contract (ADR-0182). Provenance decides this, not the shape of the diff.
	// A checkout whose control root cannot be safely resolved, a symlinked .git
	// being the reachable case, is treated as not merging rather than failing the
	// check. The seam's index read follows the symlink and succeeds, so propagating
	// the refusal here would break a staged check that worked before merge
	// detection existed. Falling back selects the stricter authored-commit
	// contract, which can refuse a legitimate merge but can never wrongly accept.
	merging, err := git.MergeInProgress(p.Root)
	if err != nil {
		merging = false
	}
	mode := currentstate.AuthoredCommit
	if merging {
		mode = currentstate.MergeAggregate
	}
	report := CurrentStateReport{
		Static:      currentstate.CheckPair(before.Universe(), after.Universe(), mode),
		Provisional: currentstate.OlderIntroductions(before.Universe(), after.Universe(), adr.CurrentFormat()),
	}
	report.Coverage = topic.EvaluateCoverage(after.Topics, eligiblePaths(afterTree, afterLock, afterCfg.ContextIgnore), coveragePolicy(afterCfg.CurrentState))
	return report, nil
}

func lockFromTree(tree *snapshot.Tree) (*manifest.Lock, error) {
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, fmt.Errorf("no staged %s/awf.lock", config.DirName)
	}
	if !file.Scannable() {
		return nil, fmt.Errorf("staged %s/awf.lock is not a scannable file", config.DirName)
	}
	lock, err := manifest.Parse(file.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse staged lock: %w", err)
	}
	return lock, nil
}

// headTreeAndLock loads HEAD and its own lock, or an empty tree and nil lock for
// an unborn or pre-adoption repository. It never consults the working tree or
// applies index lock authority to committed bytes.
func (p *Project) headTreeAndLock(ctx context.Context) (*snapshot.Tree, *manifest.Lock, error) {
	repo, err := p.gitRepo()
	if err != nil { // coverage-ignore: the staged read that precedes this already required the same handle
		return nil, nil, err
	}
	has, err := repo.HeadExists(ctx)
	if err != nil { // coverage-ignore: IndexTree already opened the same containing repository in CheckStaged; only a concurrent repository removal can fail here
		return nil, nil, err
	}
	if !has {
		tree, err := snapshot.NewTree(nil)
		return tree, nil, err
	}
	tree, err := snapshot.CommitTree(ctx, repo, "HEAD")
	if err != nil { // coverage-ignore: HEAD resolved by HeadExists just above; CommitTree fails only on a mid-read repository fault
		return nil, nil, err
	}
	lock, found, err := optionalLockFromTree(tree)
	if !found {
		return tree, nil, err
	}
	return tree, lock, err
}

func optionalLockFromTree(tree *snapshot.Tree) (*manifest.Lock, bool, error) {
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, false, nil
	}
	if !file.Scannable() {
		return nil, true, fmt.Errorf("snapshot %s/awf.lock is not a scannable file", config.DirName)
	}
	lock, err := manifest.Parse(file.Bytes)
	if err != nil {
		return nil, true, fmt.Errorf("parse snapshot lock: %w", err)
	}
	return lock, true, nil
}

// validateLockTransition preserves the remaining first-adoption identity. A
// schema-31 migration is the only accepted lock-shape change: compatibility
// routing input is parsed from the old side and discarded in the new lock.
func validateLockTransition(beforeTree *snapshot.Tree, before, after *manifest.Lock) error {
	if before == nil {
		if _, hasConfig := beforeTree.Lookup(config.DirName + "/config.yaml"); !hasConfig {
			return nil
		}
		return errors.New("pre-tracking authority: staged lock requires an empty pre-adoption HEAD without .awf/config.yaml")
	}
	if before.InitializedWithVersion != after.InitializedWithVersion {
		return errors.New("staged .awf/awf.lock changes immutable initializedWithVersion authority")
	}
	return nil
}

// loadTreeCurrentState loads the current-state view from tree, parsing config
// from that same tree so the load is single-universe (ADR-0135). The returned
// config is nil, with no error, when the tree carries no .awf/config.yaml: a
// pre-adoption or empty universe a caller may treat as an empty side.
func loadTreeCurrentState(root string, tree *snapshot.Tree, lock *manifest.Lock) (currentstate.Loaded, *config.Config, error) {
	cfgFile, ok := tree.Lookup(config.DirName + "/config.yaml")
	if !ok {
		return currentstate.Loaded{}, nil, nil
	}
	if !cfgFile.Scannable() {
		return currentstate.Loaded{}, nil, fmt.Errorf("snapshot %s/config.yaml is not a scannable file", config.DirName)
	}
	schema := migrate.Current()
	if lock != nil {
		schema = lock.SchemaVersion
	}
	configBytes, err := migrate.ConfigForCurrentSchema(cfgFile.Bytes, schema)
	if err != nil {
		return currentstate.Loaded{}, nil, err
	}
	cfg, err := config.ParseTree(config.RootDir(root), configBytes, configSnapshotReader{tree: tree})
	if err != nil {
		return currentstate.Loaded{}, nil, err
	}
	if err := cfg.Validate(); err != nil {
		return currentstate.Loaded{}, nil, err
	}
	loaded, err := currentstate.LoadFromTree(tree, cfg)
	if err != nil {
		return currentstate.Loaded{}, nil, err
	}
	return loaded, cfg, nil
}

type configSnapshotReader struct{ tree *snapshot.Tree }

func (r configSnapshotReader) ReadFile(path string) ([]byte, bool) {
	f, ok := r.tree.Lookup(config.DirName + "/" + filepath.ToSlash(path))
	if !ok || !f.Scannable() {
		return nil, false
	}
	return slices.Clone(f.Bytes), true
}
func (r configSnapshotReader) Paths(prefix string) []string {
	full := config.DirName + "/" + filepath.ToSlash(prefix)
	out := []string{}
	for _, f := range r.tree.List() {
		if f.Scannable() && strings.HasPrefix(f.Path, full) {
			out = append(out, strings.TrimPrefix(f.Path, config.DirName+"/"))
		}
	}
	return out
}

// auditTransitions runs the snapshot-diff transition check over every commit in
// the range (ADR-0135), pairing each commit's tree with its first-parent tree so
// a root commit uses the empty before universe and a merge follows its first
// parent, integrating a branch's net change at the merge. It is advisory like
// the rest of the audit: a pair whose universes cannot load is a warning rather
// than a hard stop, and a genuine transition violation is an error. Each side
// derives format boundaries from its own committed lock.
func (p *Project) auditTransitions(ctx context.Context, base, head string) ([]audit.Finding, error) {
	repo, err := p.gitRepo()
	if err != nil {
		return nil, err
	}
	commits, err := repo.RangeCommits(ctx, base, head)
	if err != nil {
		return nil, err
	}
	var out []audit.Finding
	for _, c := range commits {
		before, after, err := p.rangePairUniverses(ctx, c.Hash)
		if err != nil {
			out = append(out, audit.Finding{Severity: severity.Warn, Rule: currentStateTransitionRule, Commit: c.Hash, Subject: c.Subject,
				Detail: "could not load the current-state universes for this commit: " + err.Error()})
			continue
		}
		mode := currentstate.AuthoredCommit
		if c.IsMerge {
			mode = currentstate.MergeAggregate
		}
		for _, f := range currentstate.CheckPair(before, after, mode) {
			out = append(out, audit.Finding{Severity: severity.Error, Rule: currentStateTransitionRule, Commit: c.Hash, Subject: c.Subject, Detail: f.Message})
		}
	}
	return out, nil
}

// rangePairUniverses loads the before (first-parent) and after (commit)
// current-state universes for the transition into rev. A tree carrying no awf
// config yields the empty universe, so a pre-adoption or root pair produces no
// findings rather than an error.
func (p *Project) rangePairUniverses(ctx context.Context, rev string) (before, after currentstate.Universe, err error) {
	repo, err := p.gitRepo()
	if err != nil { // coverage-ignore: the audit range walk that reached here already required the same handle
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	beforeTree, afterTree, err := snapshot.RangePair(ctx, repo, rev)
	if err != nil {
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	beforeLock, _, err := optionalLockFromTree(beforeTree)
	if err != nil {
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	beforeLoaded, _, err := loadTreeCurrentState(p.Root, beforeTree, beforeLock)
	if err != nil {
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	afterLock, _, err := optionalLockFromTree(afterTree)
	if err != nil {
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	afterLoaded, _, err := loadTreeCurrentState(p.Root, afterTree, afterLock)
	if err != nil {
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	return beforeLoaded.Universe(), afterLoaded.Universe(), nil
}

// coveragePolicy reads only the fan-out budget from a currentState config block.
// Which checks run and the rank each reports at are fixed in code (ADR-0183). A
// nil block is a real input at both call sites and needs no special case: since
// ADR-0192 both checks evaluate regardless of block presence, and
// EffectiveMaxTopicsPerPath returns the default of 8 on a nil receiver, so a tree
// declaring no block evaluates exactly as one declaring an empty one.
func coveragePolicy(cs *config.CurrentStateConfig) topic.CoveragePolicy {
	return topic.CoveragePolicy{
		Coverage:         true,
		Fanout:           true,
		MaxTopicsPerPath: cs.EffectiveMaxTopicsPerPath(),
	}
}

// InvariantReport is one invariant claim in the working-tree topic corpus for the
// standalone `awf check invariants` report (ADR-0134): its full claim ID, backing mode
// (test or unbacked), an unbacked claim's Verify guidance, and the sorted
// proof-marker sites of a test-backed claim. Rule claims never appear. A
// backing-contract violation is a corpus load error surfaced by
// CurrentStateInvariants, never a reported entry.
type InvariantReport struct {
	ID      string   `json:"id"`
	Backing string   `json:"backing"`
	Verify  string   `json:"verify,omitempty"`
	Proofs  []string `json:"proofs,omitempty"`
}

// CurrentStateInvariants reports the invariant claims in the working-tree topic
// corpus (ADR-0134). Authority is the topic claim set: test-backed proof and
// unbacked Verify contracts are already enforced when the corpus loads, so this
// reads only typed claims and their qualified proof markers - no ADR is consulted.
func (p *Project) CurrentStateInvariants(ctx context.Context) ([]InvariantReport, error) {
	ws, err := p.workingCurrentState(ctx)
	if err != nil {
		return nil, err
	}
	var out []InvariantReport
	for _, t := range ws.Loaded.Topics.All() {
		for _, claim := range t.Claims {
			if claim.Type != topic.Invariant {
				continue
			}
			r := InvariantReport{ID: claim.ID, Backing: string(claim.Backing), Verify: claim.Verify}
			for _, s := range ws.Loaded.Topics.Markers.ForClaim(claim.ID) {
				if s.Kind == topic.ProofMarker {
					r.Proofs = append(r.Proofs, fmt.Sprintf("%s:%d", s.Path, s.Line))
				}
			}
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// eligiblePaths returns the snapshot files that are neither a generated output (a
// lock entry) nor matched by one of the contextIgnore globs. Symlinks,
// deletions, ignored, and nested-adopter paths are already excluded by the
// snapshot Tree. It takes the contextIgnore list explicitly so each caller
// filters its own universe by that universe's own config rather than the
// working config.
func eligiblePaths(tree *snapshot.Tree, lock *manifest.Lock, ignores []string) []string {
	generated := map[string]bool{}
	if lock != nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	files := tree.List()
	var nested []string
	for _, f := range files {
		if !f.Scannable() || resident.IsResidentPath(f.Path) {
			continue
		}
		const suffix = "/" + config.DirName + "/config.yaml"
		if strings.HasSuffix(f.Path, suffix) {
			nested = append(nested, strings.TrimSuffix(f.Path, suffix))
		}
	}
	var out []string
	for _, f := range files {
		if !f.Scannable() || resident.IsResidentPath(f.Path) {
			continue
		}
		insideNested := false
		for _, root := range nested {
			if f.Path == root || strings.HasPrefix(f.Path, root+"/") {
				insideNested = true
				break
			}
		}
		if insideNested || generated[f.Path] || pathglob.MatchAny(ignores, f.Path) {
			continue
		}
		out = append(out, f.Path)
	}
	return out
}
