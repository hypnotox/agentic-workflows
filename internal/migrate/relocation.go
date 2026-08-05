package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyAwfRelocation moves a finished .claude/awf/ config tree (and its lock) to
// .awf/ (ADR-0016). Idempotent: a no-op when .claude/awf/ is absent. Fails rather
// than overwrite if .awf/ already exists.
func applyAwfRelocation(root string, out *Changes) error {
	return applyAwfRelocationWithRename(root, out, os.Rename)
}

func applyAwfRelocationWithRename(root string, out *Changes, rename func(string, string) error) error {
	oldDir := filepath.Join(root, ".claude", "awf")
	newDir := config.RootDir(root)
	if _, err := os.Stat(oldDir); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("cannot relocate: %s already exists", newDir)
	}
	_, lockErr := os.Stat(filepath.Join(oldDir, "awf.lock"))
	hasAuthorityLock := lockErr == nil
	if err := rename(oldDir, newDir); err != nil {
		return err
	}
	out.Add("awf-dir-relocation: moved .claude/awf to .awf")
	if hasAuthorityLock {
		out.Add("awf-dir-relocation: moved authority lock .claude/awf/awf.lock to .awf/awf.lock")
	}
	return nil
}
