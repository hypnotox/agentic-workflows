package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// InitCollisions returns planned output paths that already exist on disk and are
// not recorded in the prior lock (i.e. not awf-managed). An awf-managed path that
// already exists is not a collision — re-init is idempotent.
func (p *Project) InitCollisions() ([]string, error) {
	planned, err := p.PlannedOutputs()
	if err != nil {
		return nil, err
	}
	managed := map[string]bool{}
	if lock, err := manifest.Load(p.lockPath()); err == nil {
		for path := range lock.Files {
			managed[path] = true
		}
	}
	var collisions []string
	for _, rel := range planned {
		if managed[rel] {
			continue
		}
		if _, err := os.Stat(filepath.Join(p.Root, rel)); err == nil {
			collisions = append(collisions, rel)
		}
	}
	sort.Strings(collisions)
	return collisions, nil
}

// BackupFile copies a colliding project-relative file to a free <path>.awf-bak[.N]
// sibling (never clobbering a prior backup) and returns the backup's
// project-relative path.
// invariant: init-force-backs-up
func (p *Project) BackupFile(rel string) (string, error) {
	src := filepath.Join(p.Root, rel)
	bak := freeBackupPath(src)
	if err := copyFile(src, bak); err != nil { // coverage-ignore: rel is a known-existing collision and bak is a free sibling path; copyFile fails only on a permission fault root bypasses
		return "", err
	}
	bakRel, _ := filepath.Rel(p.Root, bak)
	return bakRel, nil
}

// freeBackupPath returns base+".awf-bak", or "...awf-bak.N" with the lowest N
// that does not yet exist, so a forced backup never overwrites a prior one.
func freeBackupPath(base string) string {
	p := base + ".awf-bak"
	for i := 1; fileExists(p); i++ {
		p = fmt.Sprintf("%s.awf-bak.%d", base, i)
	}
	return p
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// copyFile copies src to dst, preserving the source file's permission bits.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil { // coverage-ignore: src is a known-existing collision path
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil { // coverage-ignore: src was just stat'd and is readable
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}
