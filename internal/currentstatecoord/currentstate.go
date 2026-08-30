// Package currentstatecoord coordinates current-state operations over explicitly selected immutable repository universes.
package currentstatecoord

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// CurrentStateReport is the current-state coverage and fan-out outcome.
type CurrentStateReport struct {
	Coverage      []topic.CoverageFinding
	CurrentResult checkresult.Result
	OwnerResult   checkresult.Result
}

// Result returns the coverage and fan-out result.
func (r CurrentStateReport) Result() checkresult.Result { return r.OwnerResult }

func classifyCurrentState(report CurrentStateReport) (CurrentStateReport, error) {
	findings := make([]checkresult.Finding, 0, len(report.Coverage))
	for _, coverage := range report.Coverage {
		findings = append(findings, checkresult.Finding{Rank: coverage.Severity, Property: propertyCurrentCoverage, Evidence: checkresult.Evidence{Kind: "current-state", Detail: coverage.Message()}})
	}
	current, err := checkresult.New(findings, nil)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report.CurrentResult, report.OwnerResult = current, current
	return report, nil
}

const propertyCurrentCoverage checkresult.Property = "current-state-coverage"

// workingState is one loaded working-tree current-state universe: the parsed
// topic view, the Tree it came from, and the lock.
// It is the shared substrate for CheckCurrentState, keeping the loaded corpus,
// tree, lock, and config in one working-tree universe.
type workingState struct {
	Loaded currentstate.Loaded
	Tree   *snapshot.Tree
	Lock   *manifest.Lock
	Cfg    *config.Config
}

// workingCurrentState loads the working-tree topic view and recorded lock authority.
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

// CheckWorking loads one working-tree universe and evaluates current-state
// coverage and fan-out independently of historical decisions.
func CheckWorking(root string, repo *awfgit.Repo, ctx context.Context) (CurrentStateReport, error) {
	ws, err := workingCurrentState(root, repo, ctx)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report := CurrentStateReport{}
	report.Coverage = topic.EvaluateCoverage(ws.Loaded.Topics, eligiblePaths(ws.Tree, ws.Lock), coveragePolicy(ws.Cfg.CurrentState))
	return classifyCurrentState(report)
}

// CheckStagedRoot validates staged current state without opening
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

// CheckStaged validates the lock boundary and evaluates coverage and fan-out
// over exactly the staged index universe. Dirty working bytes never participate.
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
	after, afterCfg, err := loadTreeCurrentState(root, afterTree, afterLock)
	if err != nil {
		return CurrentStateReport{}, err
	}
	report := CurrentStateReport{Coverage: topic.EvaluateCoverage(after.Topics, eligiblePaths(afterTree, afterLock), coveragePolicy(afterCfg.CurrentState))}
	return classifyCurrentState(report)
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
	if err != nil {
		return nil, nil, err
	}
	if !has {
		tree, err := snapshot.NewTree(nil)
		return tree, nil, err
	}
	tree, err := snapshot.CommitTree(ctx, repo, "HEAD")
	if err != nil {
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

// validateLockTransition preserves first-adoption and immutable lock identity.
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
// from that same tree so the load is single-universe. The returned
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
	loaded, err := currentstate.LoadFromTree(tree, cfg)
	if err != nil {
		return currentstate.Loaded{}, nil, err
	}
	return loaded, cfg, nil
}

// configFromTree parses and validates configuration from exactly the selected
// immutable tree. Semantic consumers that hand the tree to Publisher use this
// selection without independently loading topic authority.
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
