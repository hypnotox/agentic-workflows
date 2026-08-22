package currentstatecoord

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

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

func authorizationRepo(root string, repo *awfgit.Repo) (*awfgit.Repo, error) {
	if repo == nil {
		return nil, fmt.Errorf("%s: %w", root, awfgit.ErrNotARepository)
	}
	return repo, nil
}

// CheckCommitAuthorization validates the index, first parent, every incoming
// MERGE_HEAD parent, and the cleaned final message without mutating any axis.
func CheckCommitAuthorization(root string, repo *awfgit.Repo, ctx context.Context, msg commitmsg.Message) (CommitAuthorizationResult, error) {
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
	repository, err := authorizationRepo(root, repo)
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
