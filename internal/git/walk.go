package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
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

// WalkRangeCommits visits commits reachable from head but not from base. The
// range is always caller-supplied. It returns the number successfully visited;
// unrelated histories are errors.
func (r *Repo) WalkRangeCommits(ctx context.Context, base, head string, visit func(Commit) error) (int, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	headHash, err := r.repo.ResolveRevision(plumbing.Revision(head))
	if err != nil {
		return 0, opaqueWrap(fmt.Sprintf("resolve head %q", head), err)
	}
	headCommit, err := r.repo.CommitObject(*headHash)
	if err != nil { // coverage-ignore: headHash was just resolved; errors only on a corrupt object store
		return 0, opaqueError(err)
	}
	baseHash, err := r.repo.ResolveRevision(plumbing.Revision(base))
	if err != nil {
		return 0, opaqueWrap(fmt.Sprintf("resolve base %q", base), err)
	}
	baseCommit, err := r.repo.CommitObject(*baseHash)
	if err != nil { // coverage-ignore: baseHash was just resolved; errors only on a corrupt object store
		return 0, opaqueError(err)
	}
	bases, err := headCommit.MergeBase(baseCommit)
	if err != nil {
		// Reachable without corruption: finding a merge base walks the graph, so
		// an ordinary range inside a shallow clone's fetched window still runs
		// off its boundary. Unrelated roots are the other case and are NOT an
		// error; they return an empty slice, handled below.
		return 0, opaqueError(err)
	}
	if len(bases) == 0 {
		return 0, fmt.Errorf("head %q and base %q have unrelated histories", head, base)
	}
	seen := map[plumbing.Hash]bool{}
	if err := object.NewCommitPreorderIter(baseCommit, nil, nil).ForEach(func(c *object.Commit) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		seen[c.Hash] = true
		return nil
	}); err != nil {
		return 0, opaqueError(err)
	}
	if seen[headCommit.Hash] {
		return 0, nil
	}
	visited := 0
	var visitorErr error
	err = object.NewCommitPreorderIter(headCommit, seen, nil).ForEach(func(c *object.Commit) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		nc, err := toCommit(ctx, c, r.prefix)
		if err != nil {
			return err
		}
		include := r.prefix == "" || len(nc.Changes) != 0
		if r.prefix != "" && nc.IsMerge {
			include, err = mergeTouchesPrefix(ctx, c, r.prefix)
			if err != nil {
				return err
			}
		}
		if include {
			if err := visit(nc); err != nil {
				visitorErr = err
				return err
			}
			visited++
		}
		return nil
	})
	if visitorErr != nil {
		return visited, visitorErr
	}
	if err != nil {
		return visited, opaqueError(err)
	}
	if err := checkContext(ctx); err != nil {
		return visited, err
	}
	return visited, nil
}

// FirstParentChangedPaths returns sorted, unique paths changed by rev relative
// to its first parent. Roots compare against the empty tree and merges compare
// only against their first parent. Paths are rerooted to the handle root.
func (r *Repo) FirstParentChangedPaths(ctx context.Context, rev string) ([]string, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	commit, err := r.resolveCommit(rev)
	if err != nil {
		return nil, err
	}
	current, err := commit.Tree()
	if err != nil {
		return nil, opaqueError(err)
	}
	if err := validateChangedPathTree(ctx, current); err != nil {
		return nil, opaqueError(err)
	}
	var parent *object.Tree
	if commit.NumParents() > 0 {
		first, err := commit.Parent(0)
		if err != nil {
			return nil, opaqueError(err)
		}
		parent, err = first.Tree()
		if err != nil {
			return nil, opaqueError(err)
		}
		if err := validateChangedPathTree(ctx, parent); err != nil {
			return nil, opaqueError(err)
		}
	}
	changes, err := object.DiffTreeContext(ctx, parent, current)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, opaqueError(err)
	}
	paths := map[string]bool{}
	for _, change := range changes {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		if path, ok := rerootPath(change.From.Name, r.prefix); ok && path != "" {
			paths[path] = true
		}
		if path, ok := rerootPath(change.To.Name, r.prefix); ok && path != "" {
			paths[path] = true
		}
	}
	return sortedPaths(paths), nil
}

func validateChangedPathTree(ctx context.Context, tree *object.Tree) error {
	for _, entry := range tree.Entries {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if entry.Mode != filemode.Dir {
			continue
		}
		child, err := tree.Tree(entry.Name)
		if err != nil {
			return err
		}
		if err := validateChangedPathTree(ctx, child); err != nil {
			return err
		}
	}
	return nil
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

func toCommit(ctx context.Context, c *object.Commit, prefix string) (Commit, error) {
	subject, body := splitMessage(c.Message)
	parents := make([]string, len(c.ParentHashes))
	for i, parent := range c.ParentHashes {
		parents[i] = parent.String()
	}
	nc := Commit{Hash: c.Hash.String()[:8], Revision: c.Hash.String(), Subject: subject, Body: body, Message: c.Message, Parents: parents, IsMerge: c.NumParents() > 1}
	if nc.IsMerge {
		return nc, checkContext(ctx)
	}
	curTree, err := c.Tree()
	if err != nil {
		return Commit{}, err
	}
	var parentTree *object.Tree
	if c.NumParents() > 0 {
		parent, err := c.Parent(0)
		if err != nil { // coverage-ignore: unlike the same lookup in RangeBlobs this is unreachable through a range walk, because enumerating the ancestry already failed on the absent object before any commit reached toCommit; a shallow clone errors during iteration, not here
			return Commit{}, err
		}
		parentTree, err = parent.Tree()
		if err != nil {
			return Commit{}, err
		}
	}
	if err := validateChangedTreeFrontier(ctx, parentTree, curTree); err != nil {
		return Commit{}, err
	}
	changes, err := object.DiffTreeContext(ctx, parentTree, curTree)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return Commit{}, contextErr
		}
		return Commit{}, err // coverage-ignore: the validated change frontier leaves cancellation as the only reachable diff failure
	}
	patch, err := changes.PatchContext(ctx)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return Commit{}, contextErr
		}
		return Commit{}, err
	}
	stats := map[string]object.FileStat{}
	for _, s := range patch.Stats() {
		stats[s.Name] = s
	}
	for _, ch := range changes {
		fc, include, err := toFileChange(ch, parentTree, curTree, stats, prefix)
		if err != nil { // coverage-ignore: toFileChange fails only on a malformed change
			return Commit{}, err
		}
		if include {
			nc.Changes = append(nc.Changes, fc)
		}
	}
	return nc, checkContext(ctx)
}

func mergeTouchesPrefix(ctx context.Context, c *object.Commit, prefix string) (bool, error) {
	curTree, err := c.Tree()
	if err != nil {
		return false, err
	}
	parent, err := c.Parent(0)
	if err != nil { // coverage-ignore: range iteration already traversed the merge's first parent
		return false, err
	}
	parentTree, err := parent.Tree()
	if err != nil {
		return false, err
	}
	if err := validateChangedTreeFrontier(ctx, parentTree, curTree); err != nil {
		return false, err
	}
	changes, err := object.DiffTreeContext(ctx, parentTree, curTree)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return false, contextErr
		}
		return false, err // coverage-ignore: the validated change frontier leaves cancellation as the only reachable diff failure
	}
	for _, change := range changes {
		_, oldInside := scopedPath(change.From.Name, prefix)
		_, newInside := scopedPath(change.To.Name, prefix)
		if oldInside || newInside {
			return true, nil
		}
	}
	return false, checkContext(ctx)
}

func validateChangedTreeFrontier(ctx context.Context, before, after *object.Tree) error {
	beforeEntries, afterEntries := map[string]object.TreeEntry{}, map[string]object.TreeEntry{}
	names := map[string]bool{}
	if before != nil {
		for _, entry := range before.Entries {
			beforeEntries[entry.Name], names[entry.Name] = entry, true
		}
	}
	if after != nil {
		for _, entry := range after.Entries {
			afterEntries[entry.Name], names[entry.Name] = entry, true
		}
	}
	for _, name := range sortedPaths(names) {
		if err := checkContext(ctx); err != nil {
			return err
		}
		oldEntry, oldOK := beforeEntries[name]
		newEntry, newOK := afterEntries[name]
		if oldOK && newOK && oldEntry.Mode == newEntry.Mode && oldEntry.Hash == newEntry.Hash {
			continue
		}
		var oldTree, newTree *object.Tree
		var err error
		if oldOK && oldEntry.Mode == filemode.Dir {
			oldTree, err = before.Tree(name)
			if err != nil {
				return err
			}
		}
		if newOK && newEntry.Mode == filemode.Dir {
			newTree, err = after.Tree(name)
			if err != nil {
				return err
			}
		}
		switch {
		case oldTree != nil && newTree != nil:
			if err := validateChangedTreeFrontier(ctx, oldTree, newTree); err != nil {
				return err
			}
		case oldTree != nil:
			if err := validateChangedPathTree(ctx, oldTree); err != nil {
				return err
			}
		case newTree != nil:
			if err := validateChangedPathTree(ctx, newTree); err != nil {
				return err
			}
		}
	}
	return nil
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
