package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/git"
)

// WorkingTree captures the repository's working universe as an immutable Tree:
// every tracked-and-present or nonignored-untracked file, with executable and
// symlink modes preserved. Symlinks are not followed; their target is retained
// as inert bytes. Deleted, ignored, and nested-repository paths are excluded
// by the handle's WorkingPaths. It is the complete selected filesystem
// universe; generated-output and other consumer-specific eligibility filters
// are applied by downstream consumers, not here.
func WorkingTree(ctx context.Context, repo *git.Repo) (*Tree, error) {
	paths, err := repo.WorkingPaths(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot working: %w", err)
	}
	root := repo.Root()
	var files []File
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		info, statErr := os.Lstat(full)
		if statErr != nil { // coverage-ignore: git just enumerated this path; only a concurrent filesystem mutation can make Lstat fail
			return nil, fmt.Errorf("snapshot working stat %s: %w", p, statErr)
		}
		mode := Regular
		var data []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(full)
			if readErr != nil { // coverage-ignore: Lstat just identified this link; only a concurrent mutation can fail Readlink
				return nil, fmt.Errorf("snapshot working readlink %s: %w", p, readErr)
			}
			mode, data = Symlink, []byte(target)
		} else {
			if !info.Mode().IsRegular() {
				continue
			}
			var readErr error
			data, readErr = os.ReadFile(full)
			if readErr != nil {
				return nil, fmt.Errorf("snapshot working read %s: %w", p, readErr)
			}
			if info.Mode().Perm()&0o111 != 0 {
				mode = Executable
			}
		}
		files = append(files, File{Path: p, Mode: mode, Bytes: data})
	}
	return NewTree(files)
}

// WorkingContextFromEntries captures selected bytes against one already-read
// Git-owned inventory. It keeps enumeration and reading in the same operation
// without silently taking a second live view.
func WorkingContextFromEntries(ctx context.Context, repo *git.Repo, entries []git.TreeEntry, selected []string) (*LiveContext, error) {
	if repo == nil {
		return nil, git.ErrNotARepository
	}
	inventoryEntries := make([]Entry, len(entries))
	for n, entry := range entries {
		inventoryEntries[n] = Entry{Path: entry.Path, Mode: Mode(entry.Mode)}
	}
	inventory, err := NewInventory(inventoryEntries)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(selected))
	seen := map[string]bool{}
	for _, p := range selected {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		entry, ok := inventory.Lookup(p)
		if !ok {
			continue
		}
		full := filepath.Join(repo.Root(), filepath.FromSlash(p))
		var bytes []byte
		if entry.Mode == Symlink {
			target, err := os.Readlink(full)
			if err != nil {
				return nil, fmt.Errorf("snapshot working readlink %s: %w", p, err)
			}
			bytes = []byte(target)
		} else {
			data, err := os.ReadFile(full)
			if err != nil {
				return nil, fmt.Errorf("snapshot working read %s: %w", p, err)
			}
			bytes = data
		}
		files = append(files, File{Path: p, Mode: entry.Mode, Bytes: bytes})
	}
	// Every selected file came from the validated inventory and was de-duplicated.
	selection, _ := NewSelection(files)
	return NewLiveContext(inventory, selection)
}
