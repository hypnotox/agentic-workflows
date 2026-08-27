// Package currentstatecoord coordinates current-state transition operations over explicitly selected immutable repository universes.
package currentstatecoord

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/plancheck"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

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
	PlanDrift   []manifest.Drift
	PlanNotes   []string
	// PlanResult retains PlanChecker classification while PlanDrift and
	// PlanNotes remain compatibility projections.
	PlanResult checkresult.Result
	// CurrentResult and PlanArtifactResult retain disjoint typed partitions for
	// command presentation. OwnerResult is their immutable aggregate. Legacy
	// slices remain compatibility projections.
	CurrentResult      checkresult.Result
	PlanArtifactResult checkresult.Result
	OwnerResult        checkresult.Result
}

// Information returns unranked provisional introductions. They are not
// findings because the staged boundary lacks definitive merge-parent and
// message evidence; every independently derivable finding remains blocking.
func (r CurrentStateReport) Information() []string {
	out := make([]string, 0, len(r.Provisional))
	for _, introduction := range r.Provisional {
		marker := adr.FormatMarker(introduction.Format)
		if marker == "" {
			marker = "legacy"
		}
		out = append(out, fmt.Sprintf("provisional older-format ADR-%s (%s) requires commit-msg qualification", introduction.Identity, marker))
	}
	return out
}

// workingState is one loaded working-tree current-state universe: the parsed
// ADR/topic view, the Tree it came from, and the lock.
// It is the shared substrate for CheckCurrentState, keeping the loaded corpus,
// tree, lock, and config in one working-tree universe.
type workingState struct {
	Loaded currentstate.Loaded
	Tree   *snapshot.Tree
	Lock   *manifest.Lock
	Cfg    *config.Config
}

// workingCurrentState loads the working-tree ADR/topic view and recorded gaps.
func workingCurrentState(root string, repo *awfgit.Repo, ctx context.Context) (workingState, error) {
	tree, err := workingTree(root, repo, ctx)
	if errors.Is(err, awfgit.ErrNotARepository) {
		tree, err = snapshot.FilesystemTree(ctx, root)
	}
	if err != nil {
		return workingState{}, err
	}
	lock, _, err := optionalLockFromTree(tree)
	if err != nil {
		return workingState{}, err
	}
	loaded, cfg, err := loadTreeCurrentState(root, tree, lock)
	if err != nil {
		return workingState{}, err
	}
	if cfg == nil {
		return workingState{}, fmt.Errorf("working snapshot has no %s/config.yaml", config.DirName)
	}
	return workingState{Loaded: loaded, Tree: tree, Lock: lock, Cfg: cfg}, nil
}

// CheckWorking loads the working-tree current-state view and runs the
// static ADR-to-claim handshake and the coverage/fan-out evaluator over it
// (ADR-0135, ADR-0134). It reads exactly one working Tree, so the two checks
// never mix a working and an index universe. Coverage and fan-out always
// evaluate, whether or not the project configures a currentState policy
// (ADR-0192).
func CheckWorking(root string, repo *awfgit.Repo, ctx context.Context) (CurrentStateReport, error) {
	ws, err := workingCurrentState(root, repo, ctx)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report := CurrentStateReport{
		Static: currentstate.Check(ws.Loaded.ADRs, ws.Loaded.Topics.All()),
	}
	report.Coverage = topic.EvaluateCoverage(ws.Loaded.Topics, eligiblePaths(ws.Tree, ws.Lock, ws.Cfg.ContextIgnore), coveragePolicy(ws.Cfg.CurrentState))
	return classifyCurrentState(report)
}

// CheckStagedRoot validates the staged current-state transition without opening
// working-tree project configuration. The staged command must remain operable
// when a valid adopted index deliberately deletes or lacks the working config.
func CheckStagedRoot(ctx context.Context, root string) (CurrentStateReport, error) {
	repo, prefix, err := awfgit.OpenContaining(root)
	if err != nil {
		return CurrentStateReport{}, err
	}
	_ = prefix // nestedness does not alter staged current-state semantics.
	return CheckStaged(root, repo, ctx)
}

// CheckStaged loads the HEAD (before) and staged index (after) current-state
// universes and runs the snapshot-diff transition check between them plus the
// coverage/fan-out evaluator over the index (ADR-0135, ADR-0134). Both sides are
// committed or index universes, so a dirty working tree never affects the result.
// The before side is the empty universe on a repository with no commit yet, and
// the after config, policy, and eligible paths all come from the index tree so
// the staged check reads one universe. Coverage and fan-out always evaluate,
// whether or not the staged config declares a currentState policy (ADR-0192).
func CheckStaged(root string, repo *awfgit.Repo, ctx context.Context) (CurrentStateReport, error) {
	afterTree, err := indexTree(root, repo, ctx)
	if err != nil {
		return CurrentStateReport{}, err
	}
	afterLock, err := lockFromTree(afterTree)
	if err != nil {
		return CurrentStateReport{}, err
	}
	beforeTree, beforeLock, err := headTreeAndLock(repo, ctx)
	if err != nil {
		return CurrentStateReport{}, err
	}
	if err := validateLockTransition(beforeTree, afterTree, beforeLock, afterLock); err != nil {
		return CurrentStateReport{}, err
	}
	before, _, err := loadTreeCurrentState(root, beforeTree, beforeLock)
	if err != nil {
		return CurrentStateReport{}, err
	}
	after, afterCfg, err := loadTreeCurrentState(root, afterTree, afterLock)
	if err != nil {
		return CurrentStateReport{}, err
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
	merging, err := awfgit.MergeInProgress(root)
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
	beforePlans, beforePlanDrift, err := plansFromTree(beforeTree, config.DocsDir)
	if err != nil {
		return CurrentStateReport{}, fmt.Errorf("load selected before-plan comparison evidence: %w", err)
	}
	if len(beforePlanDrift) != 0 {
		return CurrentStateReport{}, fmt.Errorf("load selected before-plan comparison evidence: malformed plan")
	}
	plans, planDrift, err := plansFromTree(afterTree, config.DocsDir)
	if err != nil { // coverage-ignore: plansFromTree converts every validated plan parse failure into plan drift
		return CurrentStateReport{}, err
	}
	selected, err := selectedTerminalEvidence(beforePlans, plans, repo, ctx)
	if err != nil {
		return CurrentStateReport{}, fmt.Errorf("load selected implementation evidence: %w", err)
	}
	if err := plancheck.TerminalTransition(beforePlans, plans, selected); err != nil {
		planDrift = append(planDrift, manifest.Drift{Kind: "plan-terminal-transition", Path: "docs/plans", Detail: err.Error()})
	}
	report.PlanDrift = planDrift
	planResult, err := plancheck.Artifact(plans, after.Corpus)
	if err != nil { // coverage-ignore: staged plans and corpora are already validated semantic values
		return CurrentStateReport{}, err
	}
	report.PlanResult = planResult
	classified, err := classifyCurrentState(report)
	if err != nil { // coverage-ignore: staged semantic owners supplied validated nonempty evidence
		return CurrentStateReport{}, err
	}
	appendStagedPlanResult(&classified, planResult)
	return classified, nil
}

const (
	propertyCurrentState    checkresult.Property = "current-state-authority"
	propertyCurrentCoverage checkresult.Property = "current-state-coverage"
	propertyPlanArtifact    checkresult.Property = "plan-artifact-validity"
)

// Result returns the completed owner-classified result captured by the
// current-state coordinator. Compatibility slices cannot change it afterward.
func (r CurrentStateReport) Result() checkresult.Result {
	return r.OwnerResult
}

// classifyCurrentState stores the compatibility projection produced by the
// current-state coordinator's narrow result adapter.
func classifyCurrentState(report CurrentStateReport) (CurrentStateReport, error) {
	current, planArtifact, result, err := currentStateResult(report)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report.CurrentResult = current
	report.PlanArtifactResult = planArtifact
	report.OwnerResult = result
	return report, nil
}

func currentStateResult(report CurrentStateReport) (checkresult.Result, checkresult.Result, checkresult.Result, error) {
	var currentFindings []checkresult.Finding
	for _, finding := range report.Static {
		currentFindings = append(currentFindings, checkresult.Finding{Rank: severity.Error, Property: propertyCurrentState, Evidence: checkresult.Evidence{Kind: "current-state", Detail: finding.Message}})
	}
	for _, coverage := range report.Coverage {
		currentFindings = append(currentFindings, checkresult.Finding{Rank: coverage.Severity, Property: propertyCurrentCoverage, Evidence: checkresult.Evidence{Kind: "current-state", Detail: coverage.Message()}})
	}
	var currentInformation []checkresult.Information
	for _, message := range report.Information() {
		currentInformation = append(currentInformation, checkresult.Information{Evidence: checkresult.Evidence{Kind: "current-state", Detail: message}})
	}
	current, err := checkresult.New(currentFindings, currentInformation)
	if err != nil {
		return checkresult.Result{}, checkresult.Result{}, checkresult.Result{}, err
	}

	var planFindings []checkresult.Finding
	for _, drift := range report.PlanDrift {
		planFindings = append(planFindings, checkresult.Finding{Rank: severity.Error, Property: propertyPlanArtifact, Evidence: checkresult.Evidence{Kind: drift.Kind, Path: drift.Path, Detail: fmt.Sprintf("%s %s: %s", drift.Kind, drift.Path, drift.Detail)}})
	}
	for _, finding := range report.PlanResult.Findings() {
		if finding.Rank == severity.Error {
			finding.Evidence.Detail = fmt.Sprintf("%s %s: %s", finding.Evidence.Kind, finding.Evidence.Path, finding.Evidence.Detail)
		}
		planFindings = append(planFindings, finding)
	}
	planArtifact, err := checkresult.New(planFindings, report.PlanResult.Information())
	if err != nil { // coverage-ignore: partitions copy only validated owner results and fixed nonempty parser evidence
		return checkresult.Result{}, checkresult.Result{}, checkresult.Result{}, err
	}
	findings := append(current.Findings(), planArtifact.Findings()...)
	information := append(current.Information(), planArtifact.Information()...)
	result, err := checkresult.New(findings, information)
	if err != nil { // coverage-ignore: aggregation copies only validated immutable partitions
		return checkresult.Result{}, checkresult.Result{}, checkresult.Result{}, err
	}
	return current, planArtifact, result, nil
}

func appendStagedPlanResult(report *CurrentStateReport, result checkresult.Result) {
	for _, finding := range result.Findings() {
		if finding.Rank == severity.Error {
			report.PlanDrift = append(report.PlanDrift, manifest.Drift{Kind: finding.Evidence.Kind, Path: finding.Evidence.Path, Detail: finding.Evidence.Detail})
		} else {
			report.PlanNotes = append(report.PlanNotes, finding.Evidence.Detail)
		}
	}
}

// plansFromTree parses only regular plan files from one immutable universe.
// The caller supplies that universe's configured docs directory; it never falls
// back to working-tree paths or bytes.
// selectedTerminalEvidence resolves each terminal plan's own immutable
// implementation selector through the repository owner. It deliberately does
// not use the status-flip index diff, which normally omits prior implementation.
func selectedTerminalEvidence(before, after []plan.Plan, repo *awfgit.Repo, ctx context.Context) (map[string][]string, error) {
	prior := make(map[string]plan.Plan, len(before))
	for _, p := range before {
		prior[p.Filename] = p
	}
	selected := map[string][]string{}
	for _, next := range after {
		old, found := prior[next.Filename]
		if !found || !old.IsProposed() || !next.IsImplemented() {
			continue
		}
		if next.TerminalReconciliation == nil {
			continue // plancheck reports the missing parsed record as terminal drift.
		}
		base, head := next.TerminalReconciliation.ImplementationEndpoints()
		resolvedBase, err := repo.ResolveCommit(ctx, base)
		if err != nil {
			return nil, fmt.Errorf("%s: resolve selected implementation range base: %w", next.Path, err)
		}
		if resolvedBase != base { // coverage-ignore: the terminal parser admits only exact lowercase full object IDs, which ResolveCommit returns unchanged
			return nil, fmt.Errorf("%s: selected implementation range has an unavailable base", next.Path)
		}
		resolvedHead, err := repo.ResolveCommit(ctx, head)
		if err != nil {
			return nil, fmt.Errorf("%s: resolve selected implementation range head: %w", next.Path, err)
		}
		if resolvedHead != head { // coverage-ignore: the terminal parser admits only exact lowercase full object IDs, which ResolveCommit returns unchanged
			return nil, fmt.Errorf("%s: selected implementation range has an unavailable head", next.Path)
		}
		checkoutHead, err := repo.ResolveCommit(ctx, "HEAD")
		if err != nil { // coverage-ignore: both selected objects just resolved; only cancellation or a concurrent repository fault can now make HEAD resolution fail
			return nil, fmt.Errorf("%s: resolve current checkout HEAD: %w", next.Path, err)
		}
		if checkoutHead != head {
			return nil, fmt.Errorf("%s: selected implementation range head is not the current checkout HEAD", next.Path)
		}
		ancestor, err := repo.Ancestor(ctx, base, head)
		if err != nil { // coverage-ignore: both exact commits just resolved; only cancellation or a corrupt graph can make ancestry inspection fail
			return nil, fmt.Errorf("%s: verify selected implementation range ancestry: %w", next.Path, err)
		}
		if !ancestor {
			return nil, fmt.Errorf("%s: selected implementation range base is not an ancestor of head", next.Path)
		}
		paths, err := repo.RangeTouchedPaths(ctx, base, head)
		if err != nil { // coverage-ignore: exact reachable ancestry was just verified; only cancellation or a mid-walk object-store fault can fail accumulation
			return nil, fmt.Errorf("%s: %w", next.Path, err)
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("%s: selected implementation range has no touched paths", next.Path)
		}
		selected[next.Filename] = paths
	}
	return selected, nil
}

func plansFromTree(tree *snapshot.Tree, docsDir string) ([]plan.Plan, []manifest.Drift, error) {
	prefix := filepath.ToSlash(filepath.Join(docsDir, "plans")) + "/"
	var sources []plan.Source
	for _, file := range tree.List() {
		if !file.Scannable() {
			continue
		}
		name, ok := strings.CutPrefix(file.Path, prefix)
		if !ok || strings.Contains(name, "/") {
			continue
		}
		sources = append(sources, plan.Source{Filename: name, Path: file.Path, Bytes: file.Bytes})
	}
	plans, err := plan.ParseSources(sources)
	if err == nil {
		return plans, nil, nil
	}
	var out []manifest.Drift
	var diagnostics *plan.DiagnosticsError
	if errors.As(err, &diagnostics) {
		for _, diagnostic := range diagnostics.Diagnostics {
			out = append(out, manifest.Drift{Path: filepath.ToSlash(filepath.Join(docsDir, "plans", diagnostic.Path)), Kind: "plan-" + diagnostic.Category, Detail: diagnostic.Detail})
		}
		return plans, out, nil
	}
	return nil, nil, fmt.Errorf("parse staged plans: %w", err) // coverage-ignore: ParseSources converts every parser failure into DiagnosticsError
}

func lockFromTree(tree *snapshot.Tree) (*manifest.Lock, error) {
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, fmt.Errorf("no staged %s/awf.lock", config.DirName)
	}
	if !file.Scannable() {
		return nil, fmt.Errorf("staged %s/awf.lock is not a scannable file", config.DirName)
	}
	lock, err := manifest.ParseLive(file.Bytes, migrate.LiveSchemaFloor, migrate.Current())
	if err != nil {
		return nil, fmt.Errorf("parse staged lock: %w", err)
	}
	return lock, nil
}

// headTreeAndLock loads HEAD and its own lock, or an empty tree and nil lock for
// an unborn or pre-adoption repository. It never consults the working tree or
// applies index lock authority to committed bytes.
func headTreeAndLock(repo *awfgit.Repo, ctx context.Context) (*snapshot.Tree, *manifest.Lock, error) {
	if repo == nil {
		return nil, nil, awfgit.ErrNotARepository
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
	lock, err := manifest.ParseLive(file.Bytes, migrate.LiveSchemaFloor, migrate.Current())
	if err != nil {
		return nil, true, fmt.Errorf("parse snapshot lock: %w", err)
	}
	return lock, true, nil
}

// validateLockTransition preserves the remaining first-adoption identity. A
// schema-31 migration is the only accepted lock-shape change: compatibility
// routing input is parsed from the old side and discarded in the new lock.
func validateLockTransition(beforeTree, afterTree *snapshot.Tree, before, after *manifest.Lock) error {
	if _, hasConfig := afterTree.Lookup(config.DirName + "/config.yaml"); !hasConfig {
		return errors.New("partial staged .awf authority: awf.lock requires .awf/config.yaml; restore it or delete .awf deliberately to re-adopt")
	}
	if before == nil {
		for _, file := range beforeTree.List() {
			if file.Path == config.DirName || strings.HasPrefix(file.Path, config.DirName+"/") {
				return errors.New("pre-tracking authority: staged lock requires a complete pre-adoption HEAD without any .awf authority or residue")
			}
		}
		return nil
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
	_, hasConfig := tree.Lookup(config.DirName + "/config.yaml")
	if !hasConfig && lock != nil {
		return currentstate.Loaded{}, nil, fmt.Errorf("partial .awf authority: .awf/awf.lock requires .awf/config.yaml; restore both or delete .awf deliberately to re-adopt")
	}
	cfg, found, err := configFromTree(root, tree, lock)
	if err != nil || !found {
		return currentstate.Loaded{}, cfg, err
	}
	if cfg.Profile == catalog.ProfileCore {
		return currentstate.Loaded{}, cfg, nil
	}
	loaded, err := currentstate.LoadFromTree(tree, cfg)
	if err != nil {
		return currentstate.Loaded{}, nil, err
	}
	return loaded, cfg, nil
}

// configFromTree parses and validates configuration from exactly the selected
// immutable tree. Semantic consumers that hand the tree to Publisher use this
// selection without independently loading ADR, topic, or plan corpora.
func configFromTree(root string, tree *snapshot.Tree, lock *manifest.Lock) (*config.Config, bool, error) {
	cfgFile, ok := tree.Lookup(config.DirName + "/config.yaml")
	if !ok {
		return nil, false, nil
	}
	if !cfgFile.Scannable() {
		return nil, false, fmt.Errorf("snapshot %s/config.yaml is not a scannable file", config.DirName)
	}
	if lock == nil {
		return nil, false, fmt.Errorf("partial .awf authority: .awf/config.yaml requires .awf/awf.lock; restore it or delete .awf deliberately to re-adopt")
	}
	cfg, err := config.ParseTree(config.RootDir(root), cfgFile.Bytes, configSnapshotReader{tree: tree})
	if err != nil {
		return nil, false, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, false, err
	}
	return cfg, true, nil
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

// coveragePolicy requests both checks; internal/topic owns the fixed fan-out budget.
func coveragePolicy(_ *config.CurrentStateConfig) topic.CoveragePolicy {
	return topic.CoveragePolicy{Coverage: true, Fanout: true}
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

// workingTree snapshots the selected working repository universe.
func workingTree(root string, repo *awfgit.Repo, ctx context.Context) (*snapshot.Tree, error) {
	if repo == nil {
		return nil, fmt.Errorf("%s: %w", root, awfgit.ErrNotARepository)
	}
	return snapshot.WorkingTree(ctx, repo)
}

// indexTree snapshots the selected index repository universe.
func indexTree(root string, repo *awfgit.Repo, ctx context.Context) (*snapshot.Tree, error) {
	if repo == nil {
		return nil, fmt.Errorf("%s: %w", root, awfgit.ErrNotARepository)
	}
	return snapshot.IndexTree(ctx, repo)
}

// Findings preserves the legacy finding projection while consumers use Result.
