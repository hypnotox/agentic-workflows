package migrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	gogit "github.com/go-git/go-git/v5"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// applyRemoveWorkflowResidents removes obsolete disposable resident roots from
// the primary checkout. It deliberately uses Lstat so a hostile resident link is
// never followed.
func applyRemoveWorkflowResidents(root string, out io.Writer) error {
	// Only go-git's canonical not-a-repository error permits the fixture-tree
	// fallback. A malformed .git, unsafe topology, or identity failure is a
	// present checkout and must stop migration rather than redirect deletion.
	if _, err := awfgit.OpenRepo(root); err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return removeWorkflowResidents(root, out, os.Lstat, os.RemoveAll)
		}
		return fmt.Errorf("inspect Git checkout at %s: %w", root, err)
	}
	roots, err := awfgit.ResolveControlRoots(context.Background(), root)
	if err != nil {
		return fmt.Errorf("resolve Git control roots at %s: %w", root, err)
	}
	return removeWorkflowResidents(roots.PrimaryRoot, out, os.Lstat, os.RemoveAll)
}

func removeWorkflowResidents(primary string, out io.Writer, lstat func(string) (fs.FileInfo, error), removeAll func(string) error) error {
	for _, name := range []string{"metrics", "assignments"} {
		path := filepath.Join(primary, ".awf", name)
		info, err := lstat(path)
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "remove-workflow-residents: %s already absent\n", name)
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("remove-workflow-residents: unsafe %s root %s", name, path)
		}
		if err := removeAll(path); err != nil {
			return fmt.Errorf("remove-workflow-residents: remove %s: %w", name, err)
		}
		fmt.Fprintf(out, "remove-workflow-residents: %s removed\n", name)
	}
	return nil
}
