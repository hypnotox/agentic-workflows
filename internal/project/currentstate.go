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
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// CurrentStateReport is the direct compatibility projection of the focused coordinator result.
type CurrentStateReport = currentstatecoord.CurrentStateReport

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
