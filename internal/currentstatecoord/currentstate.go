// Package currentstatecoord coordinates current-state transition operations over explicitly selected immutable repository universes.
package currentstatecoord

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/plancheck"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// CurrentStateReport is the retained current-state outcome. Coverage and fan-out
// are the only live current-state findings; plan fields remain while staged plan
// artifact checks still use this coordinator.
type CurrentStateReport struct {
	Coverage           []topic.CoverageFinding
	PlanDrift          []manifest.Drift
	PlanNotes          []string
	PlanResult         checkresult.Result
	CurrentResult      checkresult.Result
	PlanArtifactResult checkresult.Result
	OwnerResult        checkresult.Result
}

// Result returns the coverage/fan-out and residual plan-artifact result.
func (r CurrentStateReport) Result() checkresult.Result { return r.OwnerResult }

// classifyCurrentState preserves the report partitions required by the check
// presenter. The current-state partition derives only from coverage and fan-out.
func classifyCurrentState(report CurrentStateReport) (CurrentStateReport, error) {
	findings := make([]checkresult.Finding, 0, len(report.Coverage))
	for _, coverage := range report.Coverage {
		findings = append(findings, checkresult.Finding{Rank: coverage.Severity, Property: propertyCurrentCoverage, Evidence: checkresult.Evidence{Kind: "current-state", Detail: coverage.Message()}})
	}
	current, err := checkresult.New(findings, nil)
	if err != nil {
		return CurrentStateReport{}, err
	}
	planFindings := make([]checkresult.Finding, 0, len(report.PlanDrift)+len(report.PlanResult.Findings()))
	for _, drift := range report.PlanDrift {
		planFindings = append(planFindings, checkresult.Finding{Rank: severity.Error, Property: propertyPlanArtifact, Evidence: checkresult.Evidence{Kind: drift.Kind, Path: drift.Path, Detail: fmt.Sprintf("%s %s: %s", drift.Kind, drift.Path, drift.Detail)}})
	}
	planFindings = append(planFindings, report.PlanResult.Findings()...)
	planArtifact, err := checkresult.New(planFindings, report.PlanResult.Information())
	if err != nil {
		return CurrentStateReport{}, err
	}
	all := append(current.Findings(), planArtifact.Findings()...)
	info := append(current.Information(), planArtifact.Information()...)
	owner, err := checkresult.New(all, info)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report.CurrentResult, report.PlanArtifactResult, report.OwnerResult = current, planArtifact, owner
	return report, nil
}

const (
	propertyCurrentCoverage checkresult.Property = "current-state-coverage"
	propertyPlanArtifact    checkresult.Property = "plan-artifact-validity"
)

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
	report := CurrentStateReport{}
	report.Coverage = topic.EvaluateCoverage(ws.Loaded.Topics, eligiblePaths(ws.Tree, ws.Lock), coveragePolicy(ws.Cfg.CurrentState))
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
	// The passive ADR corpus is loaded only because residual plan checks use it.
	after, afterCfg, err := loadTreeCurrentState(root, afterTree, afterLock)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report := CurrentStateReport{}
	report.Coverage = topic.EvaluateCoverage(after.Topics, eligiblePaths(afterTree, afterLock), coveragePolicy(afterCfg.CurrentState))
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
	return classified, nil
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
	configBytes, err := migrate.ConfigBytesForGeneration(lock.SchemaVersion, cfgFile.Bytes)
	if err != nil {
		return nil, false, err
	}
	cfg, err := config.ParseTree(config.RootDir(root), configBytes, configSnapshotReader{tree: tree})
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

// eligiblePaths returns ordinary snapshot files that are not generated outputs.
// Symlinks, deletions, ignored files, and nested adopters retain their independent
// exclusions.
func eligiblePaths(tree *snapshot.Tree, lock *manifest.Lock) []string {
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
		if insideNested || generated[f.Path] {
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
