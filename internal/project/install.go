package project

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
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

// BackupFile copies a colliding project-relative file to a free <path>.awf-bak[.N]
// sibling (never clobbering a prior backup) and returns the backup's
// project-relative path.
func (p *Project) BackupFile(rel string) (string, error) {
	return p.backupFile(rel, filepublication.Publish)
}

// backupFile retains project-owned backup naming and collision retry policy;
// its publication parameter lets tests force the consumer's namespace boundary.
func (p *Project) backupFile(rel string, publish func(string, []byte, fs.FileMode) error) (string, error) {
	src := filepath.Join(p.Root, rel)
	source, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open backup source %s: %w", src, err)
	}
	info, err := source.Stat()
	if err != nil { // coverage-ignore: stat on a successfully opened local source handle requires a storage fault
		return "", fmt.Errorf("inspect backup source %s: %w", src, errors.Join(err, source.Close())) // coverage-ignore: stat on a successfully opened local source handle requires a storage fault
	}
	data, err := io.ReadAll(source)
	if err != nil {
		return "", fmt.Errorf("read backup source %s: %w", src, errors.Join(err, source.Close()))
	}
	if err := source.Close(); err != nil { // coverage-ignore: closing a successfully read local source requires a storage fault
		return "", fmt.Errorf("close backup source %s: %w", src, err) // coverage-ignore: closing a successfully read local source requires a storage fault
	}
	for suffix := 0; ; suffix++ {
		bak := backupPath(src, suffix)
		err := publish(bak, data, info.Mode().Perm())
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("publish backup %s from %s: %w", bak, src, err)
		}
		bakRel, _ := filepath.Rel(p.Root, bak)
		return bakRel, nil
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func backupPath(base string, suffix int) string {
	if suffix == 0 {
		return base + ".awf-bak"
	}
	return fmt.Sprintf("%s.awf-bak.%d", base, suffix)
}
