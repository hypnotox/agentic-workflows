package project

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// currentStateTransitionRule names the range transition check in audit output.
const currentStateTransitionRule = "current-state-transition"

// CurrentStateReport is the routed outcome of a current-state check over one
// snapshot: the static ADR-to-claim handshake findings (all blocking) and the
// coverage/fan-out findings (each carrying its configured severity, ADR-0134
// item 11). Findings and Notes split the report into blocking lines and
// non-failing note lines so the command layer never re-derives the routing.
type CurrentStateReport struct {
	Static   []currentstate.Finding
	Coverage []topic.CoverageFinding
}

// Findings returns the blocking lines: every static handshake finding and every
// coverage/fan-out finding at error severity.
func (r CurrentStateReport) Findings() []string {
	var out []string
	for _, f := range r.Static {
		out = append(out, f.Message)
	}
	for _, c := range r.Coverage {
		if c.Severity == topic.CoverageError {
			out = append(out, coverageLine(c))
		}
	}
	return out
}

// Notes returns the non-failing lines: coverage/fan-out findings at warn
// severity. Off findings are never emitted by the evaluator, so they never
// appear here.
func (r CurrentStateReport) Notes() []string {
	var out []string
	for _, c := range r.Coverage {
		if c.Severity == topic.CoverageWarn {
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
// ADR/topic view, the Tree it came from, the lock, and the sealed cutoff/gaps.
// It is the shared substrate for CheckCurrentState and CurrentStateInvariants,
// which each read exactly one working Tree so a check and a report never mix a
// working and an index universe.
type workingState struct {
	Loaded currentstate.Loaded
	Tree   *snapshot.Tree
	Lock   *manifest.Lock
	Cutoff int
}

// workingCurrentState loads the working-tree ADR/topic view plus the sealed
// cutoff/gaps. Parse has already classified the lock: permanent authority owns
// the fields directly, while a bridge attestation owns them until cutover.
func (p *Project) workingCurrentState() (workingState, error) {
	tree, err := snapshot.WorkingTree(p.Root)
	if err != nil {
		return workingState{}, err
	}
	lock, _, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return workingState{}, err
	}
	cutoff, gaps := attestationCutoff(lock)
	loaded, err := currentstate.LoadFromTree(tree, p.Cfg, cutoff, gaps)
	if err != nil {
		return workingState{}, err
	}
	return workingState{Loaded: loaded, Tree: tree, Lock: lock, Cutoff: cutoff}, nil
}

// attestationCutoff returns the format cutoff and recorded legacy gaps that
// govern ADR parsing. After cutover the permanent lock fields hold them; during
// the migration window the still-present bridge attestation does. Before either
// exists no ADR is current-state-v1 and the cutoff is zero, so every ADR parses
// as legacy.
func attestationCutoff(lock *manifest.Lock) (int, []int) {
	if lock == nil {
		return 0, nil
	}
	if lock.ADRFormatV1From != 0 {
		return lock.ADRFormatV1From, lock.LegacyADRGaps
	}
	if lock.BridgeAttestation != nil {
		return lock.BridgeAttestation.ADRFormatV1From, lock.BridgeAttestation.LegacyADRGaps
	}
	return 0, nil
}

// CheckCurrentState loads the working-tree current-state view and runs the
// static ADR-to-claim handshake and the coverage/fan-out evaluator over it
// (ADR-0135, ADR-0134). It reads exactly one working Tree, so the two checks
// never mix a working and an index universe. Coverage runs only when the project
// configures a currentState policy.
func (p *Project) CheckCurrentState() (CurrentStateReport, error) {
	ws, err := p.workingCurrentState()
	if err != nil {
		return CurrentStateReport{}, err
	}
	report := CurrentStateReport{Static: currentstate.Check(ws.Loaded.ADRs, ws.Loaded.Topics.All(), ws.Cutoff)}
	if p.Cfg.CurrentState != nil {
		report.Coverage = topic.EvaluateCoverage(ws.Loaded.Topics, p.eligibleCoveragePaths(ws.Tree, ws.Lock), coveragePolicy(p.Cfg.CurrentState))
	}
	return report, nil
}

// CheckStagedRoot validates the staged current-state transition without opening
// working-tree project configuration. The staged command must remain operable
// when a valid adopted index deliberately deletes or lacks the working config.
func CheckStagedRoot(root string) (CurrentStateReport, error) {
	return (&Project{Root: root}).CheckStaged()
}

// CheckStaged loads the HEAD (before) and staged index (after) current-state
// universes and runs the snapshot-diff transition check between them plus the
// coverage/fan-out evaluator over the index (ADR-0135, ADR-0134). Both sides are
// committed or index universes, so a dirty working tree never affects the result.
// The before side is the empty universe on a repository with no commit yet, and
// the after config, policy, and eligible paths all come from the index tree so
// the staged check reads one universe. Coverage runs only when the staged config
// declares a currentState policy.
func (p *Project) CheckStaged() (CurrentStateReport, error) {
	afterTree, err := snapshot.IndexTree(p.Root)
	if err != nil {
		return CurrentStateReport{}, err
	}
	afterLock, err := lockFromTree(afterTree)
	if err != nil {
		return CurrentStateReport{}, err
	}
	before, beforeLock, err := p.headCurrentState()
	if err != nil {
		return CurrentStateReport{}, err
	}
	if err := validatePermanentLockTransition(beforeLock, afterLock); err != nil {
		return CurrentStateReport{}, err
	}
	afterCutoff, afterGaps := attestationCutoff(afterLock)
	after, afterCfg, err := loadTreeCurrentState(p.Root, afterTree, afterCutoff, afterGaps)
	if err != nil {
		return CurrentStateReport{}, err
	}
	if afterCfg == nil {
		return CurrentStateReport{}, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	report := CurrentStateReport{Static: currentstate.CheckPair(before.Universe(), after.Universe(), afterCutoff)}
	if afterCfg.CurrentState != nil {
		report.Coverage = topic.EvaluateCoverage(after.Topics, eligiblePaths(afterTree, afterLock, afterCfg.ContextIgnore), coveragePolicy(afterCfg.CurrentState))
	}
	return report, nil
}

func lockFromTree(tree *snapshot.Tree) (*manifest.Lock, error) {
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, fmt.Errorf("no staged %s/awf.lock", config.DirName)
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
func (p *Project) headTreeAndLock() (*snapshot.Tree, *manifest.Lock, error) {
	has, err := git.HeadExists(p.Root)
	if err != nil { // coverage-ignore: IndexTree already opened the same containing repository in CheckStaged; only a concurrent repository removal can fail here
		return nil, nil, err
	}
	if !has {
		tree, err := snapshot.NewTree(nil)
		return tree, nil, err
	}
	tree, err := snapshot.CommitTree(p.Root, "HEAD")
	if err != nil { // coverage-ignore: HEAD resolved by HeadExists just above; CommitTree fails only on a mid-read repository fault
		return nil, nil, err
	}
	lock, _, err := optionalLockFromTree(tree)
	return tree, lock, err
}

func (p *Project) headCurrentState() (currentstate.Loaded, *manifest.Lock, error) {
	tree, lock, err := p.headTreeAndLock()
	if err != nil {
		return currentstate.Loaded{}, nil, err
	}
	cutoff, gaps := attestationCutoff(lock)
	loaded, _, err := loadTreeCurrentState(p.Root, tree, cutoff, gaps)
	return loaded, lock, err
}

func optionalLockFromTree(tree *snapshot.Tree) (*manifest.Lock, bool, error) {
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, false, nil
	}
	lock, err := manifest.Parse(file.Bytes)
	if err != nil {
		return nil, true, fmt.Errorf("parse snapshot lock: %w", err)
	}
	return lock, true, nil
}

// validatePermanentLockTransition makes the promoted format identity immutable.
// The sole non-identical edge consumes a HEAD bridge attestation into exactly
// those same permanent values. Initial adoption has no before lock and remains
// valid.
func validatePermanentLockTransition(before, after *manifest.Lock) error {
	if before == nil {
		return nil
	}
	if before.InitializedWithVersion == after.InitializedWithVersion &&
		before.ADRFormatV1From == after.ADRFormatV1From &&
		slices.Equal(before.LegacyADRGaps, after.LegacyADRGaps) {
		return nil
	}
	if before.InitializedWithVersion == "" && after.InitializedWithVersion == "" &&
		before.ADRFormatV1From == 0 && before.BridgeAttestation != nil &&
		after.BridgeAttestation == nil &&
		after.ADRFormatV1From == before.BridgeAttestation.ADRFormatV1From &&
		slices.Equal(after.LegacyADRGaps, before.BridgeAttestation.LegacyADRGaps) {
		return nil
	}
	return errors.New("staged .awf/awf.lock changes immutable initializedWithVersion/adrFormatV1From/legacyAdrGaps authority")
}

// loadTreeCurrentState loads the current-state view from tree, parsing config
// from that same tree so the load is single-universe (ADR-0135). The returned
// config is nil, with no error, when the tree carries no .awf/config.yaml: a
// pre-adoption or empty universe a caller may treat as an empty side.
func loadTreeCurrentState(root string, tree *snapshot.Tree, cutoff int, gaps []int) (currentstate.Loaded, *config.Config, error) {
	cfgFile, ok := tree.Lookup(config.DirName + "/config.yaml")
	if !ok {
		return currentstate.Loaded{}, nil, nil
	}
	cfg, err := config.Parse(config.RootDir(root), cfgFile.Bytes)
	if err != nil {
		return currentstate.Loaded{}, nil, err
	}
	if err := cfg.Validate(); err != nil {
		return currentstate.Loaded{}, nil, err
	}
	loaded, err := currentstate.LoadFromTree(tree, cfg, cutoff, gaps)
	if err != nil {
		return currentstate.Loaded{}, nil, err
	}
	return loaded, cfg, nil
}

// auditTransitions runs the snapshot-diff transition check over every commit in
// the range (ADR-0135), pairing each commit's tree with its first-parent tree so
// a root commit uses the empty before universe and a merge follows its first
// parent, integrating a branch's net change at the merge. It is advisory like
// the rest of the audit: a pair whose universes cannot load is a warning rather
// than a hard stop, and a genuine transition violation is an error. cutoff/gaps
// are the sealed values the caller already read from the lock. Both sides read
// only committed content.
func (p *Project) auditTransitions(base, head string, cutoff int, gaps []int) ([]audit.Finding, error) {
	commits, err := audit.Collect(p.Root, base, head)
	if err != nil {
		return nil, err
	}
	var out []audit.Finding
	for _, c := range commits {
		before, after, err := p.rangePairUniverses(c.Hash, cutoff, gaps)
		if err != nil {
			out = append(out, audit.Finding{Severity: audit.Warning, Rule: currentStateTransitionRule, Commit: c.Hash, Subject: c.Subject,
				Detail: "could not load the current-state universes for this commit: " + err.Error()})
			continue
		}
		for _, f := range currentstate.CheckPair(before, after, cutoff) {
			out = append(out, audit.Finding{Severity: audit.Error, Rule: currentStateTransitionRule, Commit: c.Hash, Subject: c.Subject, Detail: f.Message})
		}
	}
	return out, nil
}

// rangePairUniverses loads the before (first-parent) and after (commit)
// current-state universes for the transition into rev. A tree carrying no awf
// config yields the empty universe, so a pre-adoption or root pair produces no
// findings rather than an error.
func (p *Project) rangePairUniverses(rev string, cutoff int, gaps []int) (before, after currentstate.Universe, err error) {
	beforeTree, afterTree, err := snapshot.RangePair(p.Root, rev)
	if err != nil {
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	beforeLoaded, _, err := loadTreeCurrentState(p.Root, beforeTree, cutoff, gaps)
	if err != nil {
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	afterLoaded, _, err := loadTreeCurrentState(p.Root, afterTree, cutoff, gaps)
	if err != nil {
		return currentstate.Universe{}, currentstate.Universe{}, err
	}
	return beforeLoaded.Universe(), afterLoaded.Universe(), nil
}

// coveragePolicy reads the coverage and fan-out severities and the fan-out
// budget from a currentState config block.
func coveragePolicy(cs *config.CurrentStateConfig) topic.CoveragePolicy {
	return topic.CoveragePolicy{
		Coverage:         topic.CoverageSeverity(cs.TopicCoverage),
		Fanout:           topic.CoverageSeverity(cs.TopicFanout),
		MaxTopicsPerPath: cs.EffectiveMaxTopicsPerPath(),
	}
}

// InvariantReport is one invariant claim in the working-tree topic corpus for the
// standalone `awf invariants` report (ADR-0134): its full claim ID, backing mode
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
func (p *Project) CurrentStateInvariants() ([]InvariantReport, error) {
	ws, err := p.workingCurrentState()
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

// eligibleCoveragePaths returns the working paths coverage evaluates: every
// snapshot file that is neither a generated output (a lock entry) nor matched by
// a configured contextIgnore glob. Symlinks, deletions, ignored, and
// nested-adopter paths are already excluded by the working Tree.
func (p *Project) eligibleCoveragePaths(tree *snapshot.Tree, lock *manifest.Lock) []string {
	return eligiblePaths(tree, lock, p.Cfg.ContextIgnore)
}

// eligiblePaths returns the snapshot files that are neither a generated output (a
// lock entry) nor matched by one of the contextIgnore globs. It takes the
// contextIgnore list explicitly so the staged check can filter the index
// universe by the index config rather than the working config.
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
		const suffix = "/" + config.DirName + "/config.yaml"
		if strings.HasSuffix(f.Path, suffix) {
			nested = append(nested, strings.TrimSuffix(f.Path, suffix))
		}
	}
	var out []string
	for _, f := range files {
		insideNested := false
		for _, root := range nested {
			if f.Path == root || strings.HasPrefix(f.Path, root+"/") {
				insideNested = true
				break
			}
		}
		if insideNested || generated[f.Path] || pathMatchesAny(ignores, f.Path) {
			continue
		}
		out = append(out, f.Path)
	}
	return out
}
