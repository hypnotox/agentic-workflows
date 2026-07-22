package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// InitCollisions returns planned output paths that already exist on disk and are
// not recorded in the prior lock (i.e. not awf-managed). An awf-managed path that
// already exists is not a collision - re-init is idempotent.
func (p *Project) InitCollisions() ([]string, error) {
	planned, err := p.PlannedOutputs()
	if err != nil {
		return nil, err
	}
	return CollisionsAt(p.Root, planned)
}

// CollisionsAt filters planned project-relative paths to those that already
// exist under root and are not recorded in root's lock (not awf-managed).
// Split from InitCollisions so init's pre-prompt probe can plan outputs in a
// throwaway scaffold and test them against the real root; the ADR-0016
// collision semantics are unchanged.
func CollisionsAt(root string, planned []string) ([]string, error) {
	managed := map[string]bool{}
	lock, _, err := manifest.LoadOptional(config.LockPath(root))
	if err != nil {
		return nil, err
	}
	if lock != nil {
		for path := range lock.Files {
			managed[path] = true
		}
	}
	var collisions []string
	for _, rel := range planned {
		if managed[rel] {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			collisions = append(collisions, rel)
		}
	}
	sort.Strings(collisions)
	return collisions, nil
}

// BackupFile copies a colliding project-relative file to a free <path>.awf-bak[.N]
// sibling (never clobbering a prior backup) and returns the backup's
// project-relative path.
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

type UninstallReport struct {
	Removed          int
	MetricsPreserved bool
}

// inspectResidentMetrics inspects only the direct children of the dynamic
// telemetry root. It never follows a metrics-root symlink, and any child other
// than the governed ignore file counts as resident data regardless of its type.
var lstatResidentMetrics = os.Lstat

func inspectResidentMetrics(root string) (bool, error) {
	metricsRoot := filepath.Join(root, config.DirName, "metrics")
	info, err := lstatResidentMetrics(metricsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect resident workflow metrics: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("unsafe resident workflow metrics root %s", filepath.Join(config.DirName, "metrics"))
	}
	entries, err := os.ReadDir(metricsRoot)
	if err != nil {
		return false, fmt.Errorf("inspect resident workflow metrics: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() != ".gitignore" {
			return true, nil
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false, errors.New("unsafe resident workflow metrics ignore file")
		}
	}
	return false, nil
}

func preserveMetricsRemoval(path string, resident bool) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	if !isMetricsResidentPath(path) {
		return false
	}
	return path != config.DirName+"/metrics/.gitignore" || resident
}

// Uninstall removes awf's generated footprint while preserving dynamic resident
// workflow metrics. It is a free function so a broken config does not block it.
// touches-state: rendering/sync-and-drift:uninstall-removes-lock-entries - lock-tracked file removal; proof in install_test.go
func Uninstall(root string) (UninstallReport, error) {
	lockPath := config.LockPath(root)
	lock, found, err := manifest.LoadOptional(lockPath)
	if err != nil {
		return UninstallReport{}, err
	}
	if !found {
		return UninstallReport{}, fmt.Errorf("no %s: nothing to uninstall", filepath.Join(config.DirName, "awf.lock"))
	}
	resident, err := inspectResidentMetrics(root)
	if err != nil {
		return UninstallReport{}, err
	}
	report := UninstallReport{MetricsPreserved: resident}
	dirs := map[string]bool{}
	for path := range lock.Files {
		// A non-local entry (corrupted or malicious lock) would delete outside
		// the root. Runtime-shaped metrics entries are corrupt and never removed.
		if !filepath.IsLocal(filepath.FromSlash(path)) || preserveMetricsRemoval(path, resident) {
			continue
		}
		abs := filepath.Join(root, path)
		if err := os.Remove(abs); err == nil {
			report.Removed++
		}
		for d := filepath.Dir(abs); d != root; d = filepath.Dir(d) {
			if resident && d == filepath.Join(root, config.DirName, "metrics") { // coverage-ignore: resident metrics paths are rejected by preserveMetricsRemoval before this ancestor walk
				break
			}
			dirs[d] = true
		}
	}
	// Remove now-empty directories deepest-first.
	dirList := slices.Collect(maps.Keys(dirs))
	slices.SortFunc(dirList, func(a, b string) int { return len(b) - len(a) })
	for _, d := range dirList {
		_ = os.Remove(d)
	}
	if err := os.Remove(lockPath); err != nil { // coverage-ignore: lock was just loaded, so removal fails only on a permission fault root bypasses
		return report, fmt.Errorf("remove lock: %w", err)
	}
	return report, nil
}
