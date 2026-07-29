package project

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
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
	Removed           int
	PreservedRoots    []string
	ResidentPreserved bool // compatibility summary for callers; roots are authoritative.
}

// residentRoots is the closed set of repository-wide resident roots awf owns,
// each paired with the template that renders its one governed .gitignore.
// Output planning, render, drift, backup detection, current-state and context
// discovery, sweep, nested-adopter filtering, install, and uninstall all read
// this single table, so a root joins or leaves awf's ownership here and only
// here. Everything below a root is dynamic local authority: it is never
// rendered, manifested, recursed into, or deleted.
var residentRoots = []struct{ Name, TemplateID string }{
	{"efforts", "efforts/gitignore.tmpl"},
	{"worktrees", "worktrees/gitignore.tmpl"},
}

// residentRootNames returns just the owned root names, in table order.
func residentRootNames() []string {
	names := make([]string, len(residentRoots))
	for i, resident := range residentRoots {
		names[i] = resident.Name
	}
	return names
}

// inspectResidentRoots examines direct children only. It never traverses a
// dynamic resident tree; a descendant other than the managed .gitignore keeps
// its root intact.
func inspectResidentRoots(root string) ([]string, error) {
	preserved := []string{}
	for _, name := range residentRootNames() {
		path := filepath.Join(root, config.DirName, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil { // coverage-ignore: root discovery's lstat error needs an external filesystem fault; unsafe and non-empty roots are covered
			return nil, fmt.Errorf("inspect resident root %s: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() { // coverage-ignore: resident-root tests exercise unsafe filesystem entries through the public uninstall path
			return nil, fmt.Errorf("unsafe resident root %s", name)
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.Name() != ".gitignore" {
				preserved = append(preserved, name)
				break
			}
		}
	}
	slices.Sort(preserved)
	return preserved, nil
}
func preserveResidentRemoval(path string, preserved []string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, name := range preserved {
		root := config.DirName + "/" + name
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// Uninstall removes awf's generated footprint while preserving dynamic resident
// state. It is a free function so a broken config does not block it.
// touches-state: rendering/sync-and-drift:uninstall-removes-lock-entries - lock-tracked file removal; proof in install_test.go
func Uninstall(root string) (UninstallReport, error) {
	lockPath := config.LockPath(root)
	residentRoot := root
	if roots, resolveErr := awfgit.ResolveControlRoots(context.Background(), root); resolveErr == nil {
		if effortRoot, residentErr := roots.ResidentRoot(awfgit.ResidentEfforts); residentErr == nil {
			residentRoot = filepath.Dir(filepath.Dir(effortRoot))
		}
	}
	lock, found, err := manifest.LoadOptional(lockPath)
	if err != nil {
		return UninstallReport{}, err
	}
	if !found {
		return UninstallReport{}, fmt.Errorf("no %s: nothing to uninstall", filepath.Join(config.DirName, "awf.lock"))
	}
	preserved, err := inspectResidentRoots(residentRoot)
	if err != nil {
		return UninstallReport{}, err
	}
	report := UninstallReport{PreservedRoots: preserved}
	dirs := map[string]bool{}
	for path := range lock.Files {
		// A non-local entry (corrupted or malicious lock) would delete outside
		// the root. Runtime-shaped resident entries are corrupt and never removed.
		if !filepath.IsLocal(filepath.FromSlash(path)) || preserveResidentRemoval(path, preserved) {
			continue
		}
		abs := filepath.Join(root, path)
		if isResidentPath(path) {
			abs = filepath.Join(residentRoot, filepath.FromSlash(path))
		}
		if err := os.Remove(abs); err == nil {
			report.Removed++
		}
		base := root
		if isResidentPath(path) {
			base = residentRoot
		}
		for d := filepath.Dir(abs); d != base; d = filepath.Dir(d) {
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
