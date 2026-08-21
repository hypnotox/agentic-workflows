package project

import (
	"fmt"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// InitCollisions returns planned output paths that already exist on disk and are
// not recorded in the prior lock (i.e. not awf-managed). An awf-managed path that
// already exists is not a collision - re-init is idempotent.
func initCollisions(p renderInputs, plan *OutputPlan) ([]string, error) {
	return resident.CollisionsAt(p.root(), plan.Paths())
}

// backupFileConfined maps the shared confined backup mechanism's result and
// errors into sync's project-specific reporting contract.
func backupFileConfined(rel string, fs syncFilesystem) (string, error) {
	return filesystem.Backup(rel,
		func(source string) ([]byte, os.FileMode, error) {
			data, mode, err := fs.ReadWithMode(source)
			if err != nil {
				return nil, 0, fmt.Errorf("read backup source %s: %w", source, err)
			}
			return data, mode, nil
		},
		func(destination string, data []byte, mode os.FileMode) error {
			if err := fs.Publish(destination, data, mode); err != nil {
				return fmt.Errorf("publish backup %s from %s: %w", destination, rel, err)
			}
			return nil
		},
	)
}
