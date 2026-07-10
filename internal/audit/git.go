package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

// openRepo opens the repo at repoRoot like git.PlainOpen, but hides its
// [extensions] config section from go-git's own extension-support check
// (repository_extensions.go verifyExtensions). That check has an upstream bug:
// it lowercases the incoming extension name ("worktreeconfig") before comparing
// it against its allow-list, whose key is mixed-case ("worktreeConfig") — the
// lookup never matches, so PlainOpen rejects any repo with
// `extensions.worktreeConfig` set (a flag `git worktree add` can leave behind
// even after the worktree is removed) regardless of repositoryformatversion.
// Neither Collect nor ruleUncommittedChanges reads repo extensions, so hiding
// the section is safe.
//
// Unlike git.PlainOpen with default options, this also resolves a `.git`
// *file* — the `gitdir:` pointer `git worktree add` leaves at a linked
// worktree's root (and the submodule layout) — mirroring what
// PlainOpenWithOptions' EnableDotGitCommonDir does, so awf's git-reading
// commands work from a linked worktree. The manual storage construction (over
// PlainOpenWithOptions) exists solely so the storer wrapper above can be
// injected.
func openRepo(repoRoot string) (*git.Repository, error) {
	dotFs, err := dotGitFs(repoRoot)
	if err != nil {
		return nil, err
	}
	st := filesystem.NewStorage(dotFs, cache.NewObjectLRUDefault())
	return git.Open(noExtensionsStorer{st}, osfs.New(repoRoot))
}

// dotGitFs resolves repoRoot's .git — a directory in a primary checkout, a
// `gitdir:` pointer file in a linked worktree or submodule — to the filesystem
// go-git should treat as the repository's dotgit. For a gitdir carrying a
// `commondir` file (the linked-worktree layout), the returned filesystem
// routes worktree-private files (HEAD, index) to the worktree's own gitdir and
// everything shared (objects, refs, config) to the common dir. A missing or
// unreadable .git falls through to the plain path so go-git reports its
// canonical not-a-repository error.
func dotGitFs(repoRoot string) (billy.Filesystem, error) {
	dotPath := filepath.Join(repoRoot, ".git")
	if fi, err := os.Stat(dotPath); err == nil && !fi.IsDir() {
		return gitfileFs(repoRoot, dotPath)
	}
	// A directory (primary checkout) or missing entirely — go-git reports its
	// canonical not-a-repository error downstream.
	return osfs.New(dotPath), nil
}

// gitfileFs resolves a `.git` pointer file to its gitdir's filesystem.
func gitfileFs(repoRoot, dotPath string) (billy.Filesystem, error) {
	raw, err := os.ReadFile(dotPath)
	if err != nil { // coverage-ignore: .git stat'd as a regular file just above; only a delete race loses it
		return nil, err
	}
	gitdirPath, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir: ")
	if !ok {
		return nil, fmt.Errorf("parse %s: expected a `gitdir:` pointer (the linked-worktree/submodule layout)", dotPath)
	}
	if !filepath.IsAbs(gitdirPath) {
		gitdirPath = filepath.Join(repoRoot, gitdirPath)
	}
	dot := osfs.New(gitdirPath)
	if commonRaw, cerr := os.ReadFile(filepath.Join(gitdirPath, "commondir")); cerr == nil {
		common := strings.TrimSpace(string(commonRaw))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdirPath, common)
		}
		return dotgit.NewRepositoryFilesystem(dot, osfs.New(common)), nil
	}
	// No commondir: the gitdir is self-contained (submodule layout). An
	// unreadable one degrades the same way — go-git then errors canonically on
	// the refs it cannot find.
	return dot, nil
}

type noExtensionsStorer struct {
	storage.Storer
}

func (s noExtensionsStorer) Config() (*gitconfig.Config, error) {
	cfg, err := s.Storer.Config()
	if err != nil {
		return nil, err
	}
	cfg.Raw.RemoveSection("extensions")
	return cfg, nil
}

// ruleUncommittedChanges flags a non-clean working tree as a branch-level Error
// (ADR-0025). It reads live working-tree state via go-git's Worktree().Status(),
// injecting the user's global and system gitignore patterns — which Status() does
// not consult on its own (it honours only the repo's .gitignore and
// .git/info/exclude) — so the rule mirrors `git status` and does not false-positive
// on globally-ignored files. Run evaluates it (it holds the repo root); it is
// range-independent, unlike the commit-history rules in evaluate.
// invariant: audit-uncommitted-changes
func ruleUncommittedChanges(repoRoot string, in Inputs) []Finding {
	if !in.UncommittedChanges {
		return nil
	}
	repo, err := openRepo(repoRoot)
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

// Collect returns the commits reachable from HEAD but not from baseBranch,
// as neutral Commit values. Empty range -> nil. Not-a-repo, an unresolvable
// base, and unrelated histories are errors.
func Collect(repoRoot, baseBranch string) ([]Commit, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}
	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil { // coverage-ignore: HEAD's hash was just resolved; errors only on a corrupt object store
		return nil, err
	}
	baseHash, err := repo.ResolveRevision(plumbing.Revision(baseBranch))
	if err != nil {
		return nil, fmt.Errorf("resolve base %q: %w", baseBranch, err)
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
		return nil, fmt.Errorf("HEAD and base %q have unrelated histories", baseBranch)
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
		nc, cerr := toCommit(c)
		if cerr != nil { // coverage-ignore: toCommit fails only on a corrupt object (see its own ignored branches)
			return cerr
		}
		commits = append(commits, nc)
		return nil
	})
	if err != nil { // coverage-ignore: mirrors the toCommit failure above; unreachable for valid commits
		return nil, err
	}
	return commits, nil
}

func toCommit(c *object.Commit) (Commit, error) {
	subject, body := splitMessage(c.Message)
	nc := Commit{
		Hash:    c.Hash.String()[:8],
		Subject: subject,
		Body:    body,
		IsMerge: c.NumParents() > 1,
	}
	if nc.IsMerge {
		// A merge's first-parent diff is the other side's work (typically main
		// merged into the branch), not this branch's — attributing it makes every
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
		fc, ferr := toFileChange(ch, parentTree, curTree, stats)
		if ferr != nil { // coverage-ignore: toFileChange fails only on a malformed change (see its own ignored branch)
			return Commit{}, ferr
		}
		nc.Changes = append(nc.Changes, fc)
	}
	return nc, nil
}

func toFileChange(ch *object.Change, parentTree, curTree *object.Tree, stats map[string]object.FileStat) (FileChange, error) {
	action, err := ch.Action()
	if err != nil { // coverage-ignore: Action() fails only on a malformed change entry
		return FileChange{}, err
	}
	fc := FileChange{OldPath: ch.From.Name, Path: ch.To.Name}
	switch action.String() {
	case "Insert":
		fc.Action = Added
		fc.Path = ch.To.Name
	case "Delete":
		fc.Action = Deleted
		fc.Path = ch.From.Name
	default:
		fc.Action = Modified
	}
	if s, ok := stats[fc.Path]; ok {
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
	return fc, nil
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
