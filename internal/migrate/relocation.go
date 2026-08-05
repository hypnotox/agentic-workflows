package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyAwfRelocation moves a finished .claude/awf/ config tree (and its lock) to
// .awf/ (ADR-0016). Idempotent: a no-op when .claude/awf/ is absent. Fails rather
// than overwrite if .awf/ already exists.
type awfRelocationOperation struct {
	stat   func(string) (os.FileInfo, error)
	rename func(string, string) error
}

func productionAwfRelocationOperation() awfRelocationOperation {
	return awfRelocationOperation{stat: os.Stat, rename: os.Rename}
}

func applyAwfRelocation(root string, out *Changes) error {
	return applyAwfRelocationWith(root, out, productionAwfRelocationOperation())
}

func applyAwfRelocationWith(root string, out *Changes, operation awfRelocationOperation) error {
	oldDir := filepath.Join(root, ".claude", "awf")
	newDir := config.RootDir(root)
	if _, err := operation.stat(oldDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect legacy awf directory %s: %w", oldDir, err)
	}
	if _, err := operation.stat(newDir); err == nil {
		return fmt.Errorf("cannot relocate: %s already exists", newDir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect relocation destination %s: %w", newDir, err)
	}
	lockPath := filepath.Join(oldDir, "awf.lock")
	_, lockErr := operation.stat(lockPath)
	if lockErr != nil && !errors.Is(lockErr, fs.ErrNotExist) {
		return fmt.Errorf("inspect legacy authority lock %s: %w", lockPath, lockErr)
	}
	hasAuthorityLock := lockErr == nil
	if err := operation.rename(oldDir, newDir); err != nil {
		return err
	}
	out.Add("awf-dir-relocation: moved .claude/awf to .awf")
	if hasAuthorityLock {
		out.Add("awf-dir-relocation: moved authority lock .claude/awf/awf.lock to .awf/awf.lock")
	}
	return nil
}
