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
func applyAwfRelocation(root string) error {
	oldDir := filepath.Join(root, ".claude", "awf")
	newDir := config.RootDir(root)
	if _, err := os.Stat(oldDir); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("cannot relocate: %s already exists", newDir)
	}
	return os.Rename(oldDir, newDir)
}
