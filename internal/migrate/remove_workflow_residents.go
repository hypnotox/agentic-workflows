package migrate

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// applyRemoveWorkflowResidents removes obsolete disposable resident roots from
// the primary checkout. It deliberately uses Lstat so a hostile resident link is
// never followed.
func applyRemoveWorkflowResidents(root string, out io.Writer) error {
	roots, err := awfgit.ResolveControlRoots(context.Background(), root)
	if err != nil {
		// Historical and freshly scaffolded non-Git trees have no control-root
		// metadata; their resident roots are local to the supplied root.
		return removeWorkflowResidents(root, out, os.Lstat, os.RemoveAll)
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
