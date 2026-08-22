package project

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
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/plancheck"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
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
	// OwnerResult is the immutable owner-classified projection consumed by
	// repository aggregation. Legacy slices remain compatibility projections.
	OwnerResult checkresult.Result
}

// Warnings returns ranked non-failing findings: coverage fan-out and Proposed
// plan assignment or detail advisories.
func (r CurrentStateReport) Warnings() []string {
	var out []string
	for _, c := range r.Coverage {
		if c.Severity == severity.Warn {
			out = append(out, c.Message())
		}
	}
	return append(out, r.PlanNotes...)
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
func checkCurrentState(root string, repo *awfgit.Repo, ctx context.Context) (CurrentStateReport, error) {
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
	return checkStaged(root, repo, ctx)
}

// CheckStaged loads the HEAD (before) and staged index (after) current-state
// universes and runs the snapshot-diff transition check between them plus the
// coverage/fan-out evaluator over the index (ADR-0135, ADR-0134). Both sides are
// committed or index universes, so a dirty working tree never affects the result.
// The before side is the empty universe on a repository with no commit yet, and
// the after config, policy, and eligible paths all come from the index tree so
// the staged check reads one universe. Coverage and fan-out always evaluate,
// whether or not the staged config declares a currentState policy (ADR-0192).
func checkStaged(root string, repo *awfgit.Repo, ctx context.Context) (CurrentStateReport, error) {
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
	if err := validateLockTransition(beforeTree, beforeLock, afterLock); err != nil {
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
	plans, planDrift, err := plansFromTree(afterTree, config.DocsDir)
	if err != nil { // coverage-ignore: plansFromTree converts every validated plan parse failure into plan drift
		return CurrentStateReport{}, err
	}
	report.PlanDrift = planDrift
	planResult, err := plancheck.Artifact(plans, after.Corpus)
	if err != nil { // coverage-ignore: staged plans and corpora are already validated semantic values
		return CurrentStateReport{}, err
	}
	appendStagedPlanResult(&report, planResult)
	return classifyCurrentState(report)
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
	result, err := currentStateResult(report)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report.OwnerResult = result
	return report, nil
}

func currentStateResult(report CurrentStateReport) (checkresult.Result, error) {
	var findings []checkresult.Finding
	for _, finding := range report.Static {
		findings = append(findings, checkresult.Finding{Rank: severity.Error, Property: propertyCurrentState, Evidence: checkresult.Evidence{Kind: "current-state", Detail: finding.Message}})
	}
	for _, coverage := range report.Coverage {
		findings = append(findings, checkresult.Finding{Rank: coverage.Severity, Property: propertyCurrentCoverage, Evidence: checkresult.Evidence{Kind: "current-state", Detail: coverage.Message()}})
	}
	for _, drift := range report.PlanDrift {
		findings = append(findings, checkresult.Finding{Rank: severity.Error, Property: propertyPlanArtifact, Evidence: checkresult.Evidence{Kind: drift.Kind, Path: drift.Path, Detail: fmt.Sprintf("%s %s: %s", drift.Kind, drift.Path, drift.Detail)}})
	}
	var information []checkresult.Information
	for _, message := range report.Information() {
		information = append(information, checkresult.Information{Evidence: checkresult.Evidence{Kind: "current-state", Detail: message}})
	}
	return checkresult.New(findings, information)
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

// CommitAuthorizationResult is the non-mutating outcome of definitive
// commit-message stale-merge authorization.
type CommitAuthorizationResult struct {
	Category          string
	Condition         string
	ChangedIndex      bool
	ChangedMessage    bool
	ChangedMergeState bool
	NextActions       []string
}

// Diagnostic maps this non-mutating authorization outcome to the shared
// actionable presentation shape. All safety axes remain explicit even when
// none moved, so a hook user can safely distinguish correction from retry.
func (r CommitAuthorizationResult) Diagnostic() (presentation.Diagnostic, error) {
	yesNo := func(changed bool) string {
		if changed {
			return "yes"
		}
		return "no"
	}
	changed := make([]presentation.Field, 0, 3)
	for _, axis := range []struct{ label, value string }{
		{"index", yesNo(r.ChangedIndex)},
		{"message", yesNo(r.ChangedMessage)},
		{"merge state", yesNo(r.ChangedMergeState)},
	} {
		value, err := presentation.Literal(axis.value)
		if err != nil { // coverage-ignore: yes/no literal is fixed valid text
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(axis.label, value)
		if err != nil { // coverage-ignore: fixed axis labels are presentation-valid
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	steps := make([]presentation.Value, len(r.NextActions))
	for i, action := range r.NextActions {
		value, err := presentation.Prose(action)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps[i] = value
	}
	return presentation.Diagnostic{Condition: r.Condition, State: r.Category, Changed: changed, Steps: steps}, nil
}

// CheckCommitAuthorization validates the index, first parent, every incoming
// MERGE_HEAD parent, and the cleaned final message without mutating any axis.
func checkCommitAuthorization(root string, repo *awfgit.Repo, ctx context.Context, msg commitmsg.Message) (CommitAuthorizationResult, error) {
	success := CommitAuthorizationResult{Category: "operation", Condition: "stale merge authorization satisfied"}
	refusal := func(observed, deficiency string) CommitAuthorizationResult {
		return CommitAuthorizationResult{
			Category:    "operation",
			Condition:   observed + ": " + deficiency,
			NextActions: []string{"correct the message trailers", "run git commit to finish the existing merge"},
		}
	}
	heads, err := awfgit.MergeHeads(root)
	if err != nil {
		return CommitAuthorizationResult{}, fmt.Errorf("read merge heads: %w", err)
	}
	observed := "non-merge"
	if len(heads) > 0 {
		observed = "merge with MERGE_HEAD " + strings.Join(heads, ",")
	}
	authorizations, parseErr := commitmsg.ParseAuthorizations(msg, func(value string) bool {
		return value == "legacy" || adr.KnownFormatMarker(value)
	})
	if parseErr != nil {
		var syntax *commitmsg.SyntaxError
		if errors.As(parseErr, &syntax) {
			return refusal(observed, fmt.Sprintf("malformed reserved trailer at cleaned line %d: %s", syntax.Line, syntax.Reason)), parseErr
		}
		return CommitAuthorizationResult{}, parseErr // coverage-ignore: commitmsg exposes only SyntaxError refusals
	}
	repository, err := gitRepo(root, repo)
	if err != nil {
		return CommitAuthorizationResult{}, fmt.Errorf("open authorization repository: %w", err)
	}
	resultTree, err := snapshot.IndexTree(ctx, repository)
	if err != nil {
		return CommitAuthorizationResult{}, fmt.Errorf("load result index tree: %w", err)
	}
	hasHead, err := repository.HeadExists(ctx)
	if err != nil {
		return CommitAuthorizationResult{}, fmt.Errorf("resolve first-parent HEAD: %w", err)
	}
	var firstTree *snapshot.Tree
	if hasHead {
		firstTree, err = snapshot.CommitTree(ctx, repository, "HEAD")
	} else {
		firstTree, err = snapshot.NewTree(nil)
	}
	if err != nil { // coverage-ignore: NewTree(nil) cannot fail, and HeadExists resolved the same HEAD immediately before CommitTree; only a concurrent repository fault reaches this
		return CommitAuthorizationResult{}, fmt.Errorf("load first-parent HEAD tree: %w", err)
	}
	incomingTrees, err := snapshot.CommitTrees(ctx, repository, heads)
	if err != nil {
		return CommitAuthorizationResult{}, fmt.Errorf("load incoming parent trees %s: %w", strings.Join(heads, ","), err)
	}
	load := func(label string, tree *snapshot.Tree) (currentstate.Universe, error) {
		lock, _, err := optionalLockFromTree(tree)
		if err != nil {
			return currentstate.Universe{}, fmt.Errorf("load %s lock: %w", label, err)
		}
		loaded, _, err := loadTreeCurrentState(root, tree, lock)
		if err != nil {
			return currentstate.Universe{}, fmt.Errorf("load %s current state: %w", label, err)
		}
		return loaded.Universe(), nil
	}
	first, err := load("first-parent HEAD", firstTree)
	if err != nil {
		return CommitAuthorizationResult{}, err
	}
	result, err := load("result index", resultTree)
	if err != nil {
		return CommitAuthorizationResult{}, err
	}
	incoming := make([]currentstate.Universe, len(incomingTrees))
	for i, tree := range incomingTrees {
		incoming[i], err = load("incoming parent "+heads[i], tree)
		if err != nil {
			return CommitAuthorizationResult{}, err
		}
	}
	qualifications := currentstate.QualifyIncoming(first, result, incoming, adr.CurrentFormat())
	if len(qualifications) == 0 {
		return success, nil
	}
	if len(heads) == 0 {
		return refusal(observed, "provisional older-format introduction without merge parents"), nil
	}
	allowed := map[string]bool{}
	for _, authorization := range authorizations {
		allowed[authorization.Version] = true
	}
	for _, qualification := range qualifications {
		if !qualification.Qualified {
			return refusal(observed, "unqualified incoming-parent record ADR-"+qualification.Introduction.Identity), nil
		}
		version := adr.FormatMarker(qualification.Introduction.Format)
		if version == "" {
			version = "legacy"
		}
		if !allowed[version] {
			return refusal(observed, "missing authorization version "+version+" for ADR-"+qualification.Introduction.Identity), nil
		}
	}
	return success, nil
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
	if cfg.Profile == catalog.ProfileCore {
		return currentstate.Loaded{}, cfg, nil
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
