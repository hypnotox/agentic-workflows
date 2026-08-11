package project

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// InitCollisions returns planned output paths that already exist on disk and are
// not recorded in the prior lock (i.e. not awf-managed). An awf-managed path that
// already exists is not a collision - re-init is idempotent.
func (p *Project) InitCollisions(ctx context.Context) ([]string, error) {
	planned, err := p.PlannedOutputs(ctx)
	if err != nil {
		return nil, err
	}
	return resident.CollisionsAt(p.Root, planned)
}

// backupFileConfined preserves backup naming and collision retry through the
// selected sync root. Source bytes and mode come from one confined open.
func (p *Project) backupFileConfined(rel string, filesystem syncFilesystem) (string, error) {
	data, mode, err := filesystem.ReadWithMode(rel)
	if err != nil {
		return "", fmt.Errorf("read backup source %s: %w", rel, err)
	}
	for suffix := 0; ; suffix++ {
		bak := backupPath(rel, suffix)
		err := filesystem.Publish(bak, data, mode)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("publish backup %s from %s: %w", bak, rel, err)
		}
		return bak, nil
	}
}

func backupPath(base string, suffix int) string {
	if suffix == 0 {
		return base + ".awf-bak"
	}
	return fmt.Sprintf("%s.awf-bak.%d", base, suffix)
}
