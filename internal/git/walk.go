package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Action is how a file changed in a commit.
type Action int

const (
	Added Action = iota
	Modified
	Deleted
)

// FileChange is one file touched by a commit. OldText/NewText are populated
// only for markdown files, empty otherwise.
type FileChange struct {
	Path             string
	OldPath          string
	Action           Action
	Added, Deleted   int
	OldText, NewText string
}

// Commit is the semantic view of one range commit.
type Commit struct {
	Hash     string
	Revision string
	Subject  string
	Body     string
	Message  string
	Parents  []string
	IsMerge  bool
	Changes  []FileChange
}

// RangeCommits returns the commits reachable from head but not from base. The
// range is always caller-supplied. Empty range returns nil, and unrelated
// histories are errors.
func (r *Repo) RangeCommits(ctx context.Context, base, head string) ([]Commit, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	headHash, err := r.repo.ResolveRevision(plumbing.Revision(head))
	if err != nil {
		return nil, opaqueWrap(fmt.Sprintf("resolve head %q", head), err)
	}
	headCommit, err := r.repo.CommitObject(*headHash)
	if err != nil { // coverage-ignore: headHash was just resolved; errors only on a corrupt object store
		return nil, opaqueError(err)
	}
	baseHash, err := r.repo.ResolveRevision(plumbing.Revision(base))
	if err != nil {
		return nil, opaqueWrap(fmt.Sprintf("resolve base %q", base), err)
	}
	baseCommit, err := r.repo.CommitObject(*baseHash)
	if err != nil { // coverage-ignore: baseHash was just resolved; errors only on a corrupt object store
		return nil, opaqueError(err)
	}
	bases, err := headCommit.MergeBase(baseCommit)
	if err != nil {
		// Reachable without corruption: finding a merge base walks the graph, so
		// an ordinary range inside a shallow clone's fetched window still runs
		// off its boundary. Unrelated roots are the other case and are NOT an
		// error; they return an empty slice, handled below.
		return nil, opaqueError(err)
	}
	if len(bases) == 0 {
		return nil, fmt.Errorf("head %q and base %q have unrelated histories", head, base)
	}
	seen := map[plumbing.Hash]bool{}
	if err := object.NewCommitPreorderIter(baseCommit, nil, nil).ForEach(func(c *object.Commit) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		seen[c.Hash] = true
		return nil
	}); err != nil {
		return nil, opaqueError(err)
	}
	if seen[headCommit.Hash] {
		return nil, nil
	}
	var commits []Commit
	err = object.NewCommitPreorderIter(headCommit, seen, nil).ForEach(func(c *object.Commit) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		nc, err := toCommit(c, r.prefix)
		if err != nil { // coverage-ignore: every branch toCommit can fail on is itself unreachable from a walk that already enumerated this commit's ancestry (see its own ignored branches)
			return err
		}
		include := r.prefix == "" || len(nc.Changes) != 0
		if r.prefix != "" && nc.IsMerge {
			include, err = mergeTouchesPrefix(c, r.prefix)
			if err != nil { // coverage-ignore: range iteration already resolved this merge and its first parent trees
				return err
			}
		}
		if include {
			commits = append(commits, nc)
		}
		return nil
	})
	if err != nil {
		return nil, opaqueError(err)
	}
	return commits, nil
}

// FileText reads path from rev. path is relative to the handle root. A missing
// path returns found false; revision and object failures remain errors.
func (r *Repo) FileText(ctx context.Context, rev, path string) (text string, found bool, err error) {
	if err := checkContext(ctx); err != nil {
		return "", false, err
	}
	tree, err := treeAt(r.repo, rev)
	if err != nil {
		return "", false, opaqueError(err)
	}
	if r.prefix != "" {
		path = r.prefix + "/" + path
	}
	file, err := tree.File(path)
	if errors.Is(err, object.ErrFileNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, opaqueError(err)
	}
	text, err = file.Contents()
	if err != nil { // coverage-ignore: a valid tree entry's blob contents read back without error
		return "", false, opaqueError(err)
	}
	return text, true, nil
}

// MergeBase returns the merge-base revision for a and b.
func (r *Repo) MergeBase(ctx context.Context, a, b string) (string, error) {
	out, err := r.runner.run(ctx, "merge-base", a, b)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RangeChangedPaths returns paths changed between base and head, rerooted to
// the handle root, using native Git's --name-only semantics.
func (r *Repo) RangeChangedPaths(ctx context.Context, base, head string) ([]string, error) {
	out, err := r.runner.run(ctx, "diff", "--name-only", base, head)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path, ok := rerootPath(path, r.prefix); ok && path != "" {
			set[path] = true
		}
	}
	return sortedPaths(set), nil
}

// RangeDiffText returns unified diff text for base through head. The options
// pin the b/ destination prefix consumed by repoaudit's parser.
func (r *Repo) RangeDiffText(ctx context.Context, base, head string) (string, error) {
	// The three -c pins defend against REPOSITORY-local diff configuration, which
	// the isolated environment does not strip: a repo setting diff.noprefix or
	// diff.dstPrefix would otherwise render a diff this function's consumer
	// cannot parse. Two of them are individually falsifiable and pinned by the
	// contract suite; diff.mnemonicprefix is subsumed by the other two under
	// every configuration reachable here and is kept as defence rather than
	// because a test distinguishes it.
	out, err := r.runner.run(ctx,
		"-c", "diff.noprefix=false", "-c", "diff.mnemonicprefix=false", "-c", "diff.dstPrefix=b/",
		"diff", "--no-ext-diff", "-U0", base, head, "--", "*.go")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func toCommit(c *object.Commit, prefix string) (Commit, error) {
	subject, body := splitMessage(c.Message)
	parents := make([]string, len(c.ParentHashes))
	for i, parent := range c.ParentHashes {
		parents[i] = parent.String()
	}
	nc := Commit{Hash: c.Hash.String()[:8], Revision: c.Hash.String(), Subject: subject, Body: body, Message: c.Message, Parents: parents, IsMerge: c.NumParents() > 1}
	if nc.IsMerge {
		return nc, nil
	}
	curTree, err := c.Tree()
	if err != nil { // coverage-ignore: a commit's own tree resolves for any valid commit
		return Commit{}, err
	}
	var parentTree *object.Tree
	if c.NumParents() > 0 {
		parent, err := c.Parent(0)
		if err != nil { // coverage-ignore: unlike the same lookup in RangeBlobs this is unreachable through a range walk, because enumerating the ancestry already failed on the absent object before any commit reached toCommit; a shallow clone errors during iteration, not here
			return Commit{}, err
		}
		parentTree, err = parent.Tree()
		if err != nil { // coverage-ignore: a valid parent commit's tree resolves
			return Commit{}, err
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
		fc, include, err := toFileChange(ch, parentTree, curTree, stats, prefix)
		if err != nil { // coverage-ignore: toFileChange fails only on a malformed change (see its own ignored branch)
			return Commit{}, err
		}
		if include {
			nc.Changes = append(nc.Changes, fc)
		}
	}
	return nc, nil
}

func mergeTouchesPrefix(c *object.Commit, prefix string) (bool, error) {
	curTree, err := c.Tree()
	if err != nil { // coverage-ignore: range iteration already resolved the merge commit tree
		return false, err
	}
	parent, err := c.Parent(0)
	if err != nil { // coverage-ignore: range iteration already traversed the merge's first parent
		return false, err
	}
	parentTree, err := parent.Tree()
	if err != nil { // coverage-ignore: range iteration already resolved the first-parent commit tree
		return false, err
	}
	changes, err := object.DiffTree(parentTree, curTree)
	if err != nil { // coverage-ignore: diffing two resolved trees does not fail
		return false, err
	}
	for _, change := range changes {
		_, oldInside := scopedPath(change.From.Name, prefix)
		_, newInside := scopedPath(change.To.Name, prefix)
		if oldInside || newInside {
			return true, nil
		}
	}
	return false, nil
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
		fc.Action, fc.OldPath = Added, ""
	case action.String() == "Delete" || !newInside:
		fc.Action, fc.Path = Deleted, oldPath
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
