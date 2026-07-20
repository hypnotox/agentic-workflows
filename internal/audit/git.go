package audit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/object"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// ruleUncommittedChanges flags a non-clean working tree as a branch-level Error
// (ADR-0025). It reads live working-tree state via go-git's Worktree().Status(),
// injecting the user's global and system gitignore patterns - which Status() does
// not consult on its own (it honours only the repo's .gitignore and
// .git/info/exclude) - so the rule mirrors `git status` and does not false-positive
// on globally-ignored files. Run evaluates it (it holds the repo root); it is
// range-independent, unlike the commit-history rules in evaluate.
// touches-state: tooling/audit-and-snapshots:audit-uncommitted-changes - uncommitted-changes live-state rule; proof in git_test.go
func ruleUncommittedChanges(repoRoot string, in Inputs) []Finding {
	if !in.UncommittedChanges {
		return nil
	}
	repo, err := awfgit.OpenRepo(repoRoot)
	if err != nil { // coverage-ignore: Run calls Collect first, which opens the same repo and errors earlier on a non-repo
		return nil
	}
	wt, err := repo.Worktree()
	if err != nil { // coverage-ignore: a bare / worktree-less repo is outside awf audit's intended use
		return nil
	}
	root := osfs.New("/")
	global, _ := gitignore.LoadGlobalPatterns(root)
	system, _ := gitignore.LoadSystemPatterns(root)
	wt.Excludes = slices.Concat(global, system)
	status, err := wt.Status()
	if err != nil { // coverage-ignore: Status on the healthy worktree we just opened does not fail
		return nil
	}
	if status.IsClean() {
		return nil
	}
	tracked, untracked := 0, 0
	for _, st := range status {
		if st.Staging == git.Untracked && st.Worktree == git.Untracked {
			untracked++
		} else {
			tracked++
		}
	}
	return []Finding{{
		Severity: Error,
		Rule:     "uncommitted-changes",
		Detail: fmt.Sprintf("working tree not clean: %d tracked change(s), %d untracked file(s); commit or discard before concluding the implementation",
			tracked, untracked),
	}}
}

// Collect returns the commits reachable from head but not from base, as neutral
// Commit values. The range is always caller-supplied (ADR-0127): there is no
// default and no configured base. Empty range -> nil. Not-a-repo, an
// unresolvable base or head, and unrelated histories are errors.
func Collect(repoRoot, base, head string) ([]Commit, error) {
	repo, prefix, err := awfgit.OpenContainingRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	headHash, err := repo.ResolveRevision(plumbing.Revision(head))
	if err != nil {
		return nil, fmt.Errorf("resolve head %q: %w", head, err)
	}
	headCommit, err := repo.CommitObject(*headHash)
	if err != nil { // coverage-ignore: headHash was just resolved; errors only on a corrupt object store
		return nil, err
	}
	baseHash, err := repo.ResolveRevision(plumbing.Revision(base))
	if err != nil {
		return nil, fmt.Errorf("resolve base %q: %w", base, err)
	}
	baseCommit, err := repo.CommitObject(*baseHash)
	if err != nil { // coverage-ignore: baseHash was just resolved; errors only on a corrupt object store
		return nil, err
	}
	bases, err := headCommit.MergeBase(baseCommit)
	if err != nil { // coverage-ignore: MergeBase errors only on a corrupt object graph; unrelated roots return an empty slice
		return nil, err
	}
	if len(bases) == 0 {
		return nil, fmt.Errorf("head %q and base %q have unrelated histories", head, base)
	}
	// Prune the HEAD walk by everything reachable from base.
	seen := map[plumbing.Hash]bool{}
	if ferr := object.NewCommitPreorderIter(baseCommit, nil, nil).
		ForEach(func(c *object.Commit) error { seen[c.Hash] = true; return nil }); ferr != nil { // coverage-ignore: the callback never errors and walking a valid graph does not fail
		return nil, ferr
	}
	if seen[headCommit.Hash] {
		return nil, nil // HEAD already in base: empty range
	}
	var commits []Commit
	err = object.NewCommitPreorderIter(headCommit, seen, nil).ForEach(func(c *object.Commit) error {
		nc, cerr := toCommit(c, prefix)
		if cerr != nil { // coverage-ignore: toCommit fails only on a corrupt object (see its own ignored branches)
			return cerr
		}
		// A nested adopter audits only commits that changed its own subtree. A
		// containing-repository commit with no in-scope change is deliberately
		// absent rather than contributing foreign commit metadata to the rules.
		if prefix == "" || len(nc.Changes) != 0 {
			commits = append(commits, nc)
		}
		return nil
	})
	if err != nil { // coverage-ignore: mirrors the toCommit failure above; unreachable for valid commits
		return nil, err
	}
	return commits, nil
}

func toCommit(c *object.Commit, prefix string) (Commit, error) {
	subject, body := splitMessage(c.Message)
	nc := Commit{
		Hash:    c.Hash.String()[:8],
		Subject: subject,
		Body:    body,
		IsMerge: c.NumParents() > 1,
	}
	if nc.IsMerge {
		// A merge's first-parent diff is the other side's work (typically main
		// merged into the branch), not this branch's - attributing it makes every
		// file-based rule fire on foreign changes. The branch's own work is
		// carried by its non-merge commits, so a merge contributes no Changes.
		return nc, nil
	}
	curTree, err := c.Tree()
	if err != nil { // coverage-ignore: a commit's own tree resolves for any valid commit
		return Commit{}, err
	}
	var parentTree *object.Tree
	if c.NumParents() > 0 {
		parent, perr := c.Parent(0)
		if perr != nil { // coverage-ignore: parent count was just checked > 0; the parent object exists
			return Commit{}, perr
		}
		if parentTree, perr = parent.Tree(); perr != nil { // coverage-ignore: a valid parent commit's tree resolves
			return Commit{}, perr
		}
	}
	changes, err := object.DiffTree(parentTree, curTree)
	if err != nil { // coverage-ignore: diffing two resolved trees does not fail
		return Commit{}, err
	}
	patch, err := changes.Patch()
	if err != nil { // coverage-ignore: building a patch from a valid change set does not fail
		return Commit{}, err
	}
	stats := map[string]object.FileStat{}
	for _, s := range patch.Stats() {
		stats[s.Name] = s
	}
	for _, ch := range changes {
		fc, include, ferr := toFileChange(ch, parentTree, curTree, stats, prefix)
		if ferr != nil { // coverage-ignore: toFileChange fails only on a malformed change (see its own ignored branch)
			return Commit{}, ferr
		}
		if include {
			nc.Changes = append(nc.Changes, fc)
		}
	}
	return nc, nil
}

func toFileChange(ch *object.Change, parentTree, curTree *object.Tree, stats map[string]object.FileStat, prefix string) (FileChange, bool, error) {
	action, err := ch.Action()
	if err != nil { // coverage-ignore: Action() fails only on a malformed change entry
		return FileChange{}, false, err
	}
	oldPath, oldInside := scopedPath(ch.From.Name, prefix)
	newPath, newInside := scopedPath(ch.To.Name, prefix)
	if !oldInside && !newInside {
		return FileChange{}, false, nil
	}
	fc := FileChange{OldPath: oldPath, Path: newPath}
	switch {
	case action.String() == "Insert" || !oldInside:
		fc.Action = Added
		fc.OldPath = ""
	case action.String() == "Delete" || !newInside:
		fc.Action = Deleted
		fc.Path = oldPath
	default:
		fc.Action = Modified
	}
	statPath := ch.To.Name
	if fc.Action == Deleted {
		statPath = ch.From.Name
	}
	if s, ok := stats[statPath]; ok {
		fc.Added, fc.Deleted = s.Addition, s.Deletion
	}
	if strings.HasSuffix(fc.Path, ".md") {
		if fc.Action != Added && parentTree != nil {
			fc.OldText = fileText(parentTree, ch.From.Name)
		}
		if fc.Action != Deleted {
			fc.NewText = fileText(curTree, ch.To.Name)
		}
	}
	return fc, true, nil
}

func scopedPath(path, prefix string) (string, bool) {
	if path == "" {
		return "", false
	}
	if prefix == "" {
		return path, true
	}
	return strings.CutPrefix(path, prefix+"/")
}

func fileText(tree *object.Tree, name string) string {
	f, err := tree.File(name)
	if err != nil {
		return ""
	}
	s, err := f.Contents()
	if err != nil { // coverage-ignore: a valid tree entry's blob contents read back without error
		return ""
	}
	return s
}

func splitMessage(msg string) (subject, body string) {
	msg = strings.ReplaceAll(msg, "\r\n", "\n")
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return strings.TrimRight(msg[:i], " "), strings.TrimSpace(msg[i+1:])
	}
	return strings.TrimRight(msg, " "), ""
}
