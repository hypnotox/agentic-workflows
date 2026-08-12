// Package resident owns resident-root policy and path anchoring: the closed
// table of repository-wide roots awf owns at the primary control root, the
// predicate that recognises a path or a render kind as resident, the Roots
// value that resolves an output path against the right anchor, and the
// resident lifecycle operations. The sync core depends on this package; this
// package never depends on the core.
package resident

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// roots is the closed set of repository-wide resident-root names awf owns
// under the config dir. Output planning, render, drift, backup detection,
// current-state and context discovery, sweep, nested-adopter filtering,
// install, and uninstall all read this single table, so a root joins or leaves
// awf's ownership here and only here. Everything below a root is dynamic local
// authority: it is never rendered, manifested, recursed into, or deleted.
//
// The table carries names only. Template identity for each root's one governed
// .gitignore belongs to the render core's single derivation, so this package
// never spells a template id (ADR-0195 item 5).
var roots = []string{"efforts", "worktrees", "effort-archive"}

// RootNames returns the owned root names in table order. It hands back a copy
// so the single declaration above stays the only writable home of the set.
func RootNames() []string { return slices.Clone(roots) }

// IsResidentPath reports whether a config-relative slash path names a resident
// root or something below one.
func IsResidentPath(path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, name := range RootNames() {
		root := config.DirName + "/" + name
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// IsResidentKind reports whether a render kind is one of the resident roots'
// self-ignoring render units, so a caller never spells the names to decide it.
func IsResidentKind(kind string) bool {
	return slices.Contains(RootNames(), kind)
}

// Roots anchors output paths. Tracked is the invoking checkout that owns every
// tracked output; Resident is the primary control root that owns the dynamic
// resident trees. Both are construction inputs, fixed when a project opens.
type Roots struct {
	Tracked  string
	Resident string
}

// NewRoots pairs the tracked and resident anchors into the value that owns
// output-path resolution.
func NewRoots(tracked, resident string) Roots {
	return Roots{Tracked: tracked, Resident: resident}
}

// ResolveOutput resolves resident root artifacts at the primary control root
// while leaving every tracked output anchored at the invoking checkout.
func (r Roots) ResolveOutput(path string) string {
	if IsResidentPath(path) {
		return filepath.Join(r.Resident, filepath.FromSlash(path))
	}
	return filepath.Join(r.Tracked, filepath.FromSlash(path))
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

// Backup records one local document preserved during uninstall. Both paths are
// lock-relative and Bak is the exact first-free sibling chosen by Backup.
type Backup struct {
	Path string
	Bak  string
}

type UninstallReport struct {
	Removed        int
	PreservedRoots []string
	Backups        []Backup
}

// Document maps an uninstall result into its complete ordinary presentation.
func (r UninstallReport) Document() (presentation.Document, error) {
	removed, err := presentation.Literal(strconv.Itoa(r.Removed))
	if err != nil { // coverage-ignore: decimal formatting always produces a nonempty literal without line breaks
		return presentation.Document{}, err
	}
	removedField, err := presentation.NewField("generated files removed", removed)
	if err != nil { // coverage-ignore: Literal validated the value and the label is fixed and grammar-valid
		return presentation.Document{}, err
	}
	note, err := presentation.Prose("the authored .awf config remains in place; delete it to fully remove")
	if err != nil { // coverage-ignore: fixed nonempty prose contains no forbidden line break
		return presentation.Document{}, err
	}
	notes := []presentation.Value{note}
	for _, root := range r.PreservedRoots {
		value, err := presentation.Prose("preserved resident data under .awf/" + root)
		if err != nil { // coverage-ignore: the fixed prefix remains nonempty after normalization of a validated resident-root name
			return presentation.Document{}, err
		}
		notes = append(notes, value)
	}
	for _, backup := range r.Backups {
		value, err := presentation.Prose("backed up " + backup.Path + " to " + backup.Bak)
		if err != nil { // coverage-ignore: backup paths are confined lock-relative paths
			return presentation.Document{}, err
		}
		notes = append(notes, value)
	}
	return (presentation.Mutation{Status: "uninstall completed", Identity: []presentation.Field{removedField}, Notes: notes}).Document()
}

// InspectRoots examines direct children only. It never traverses a
// dynamic resident tree; a descendant other than the managed .gitignore keeps
// its root intact.
func InspectRoots(root string) ([]string, error) {
	preserved := []string{}
	for _, name := range RootNames() {
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

// PreserveRemoval reports whether a lock-relative path lies inside a resident
// root the inspection found to hold dynamic data, so removal must skip it.
func PreserveRemoval(path string, preserved []string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, name := range preserved {
		root := config.DirName + "/" + name
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// RemoveGeneratedFile removes one generated file. An already-absent path is a
// successful no-op; every other failure remains actionable so its lock entry
// can be preserved for a retry.
func RemoveGeneratedFile(path string) (bool, error) {
	err := os.Remove(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("remove generated file %s: %w", path, err)
	}
}

type uninstallHandle interface {
	LinkInfo(string) (fs.FileInfo, error)
	Backup(string) (string, error)
	Close() error
}

type uninstallOps struct {
	open         func(string) (uninstallHandle, error)
	inspectRoots func(string) ([]string, error)
	removeFile   func(string) (bool, error)
	remove       func(string) error
}

func productionUninstallOpen(root string) (uninstallHandle, error) {
	handle, err := filesystem.Open(root)
	if err != nil {
		return nil, err
	}
	return asUninstallHandle(handle), nil
}

func asUninstallHandle(handle uninstallHandle) uninstallHandle { return handle }

// Uninstall removes awf's generated footprint while preserving dynamic resident
// state. It is a free function so a broken config does not block it.
// preserveTemplate is a bounded policy supplied by outer composition.
// touches-state: rendering/sync-and-drift:uninstall-removes-lock-entries - lock-tracked file removal; proof in resident_test.go
func Uninstall(ctx context.Context, root string, preserveTemplate func(string) bool) (UninstallReport, error) {
	return uninstallWith(ctx, root, preserveTemplate, uninstallOps{
		open:         productionUninstallOpen,
		inspectRoots: InspectRoots,
		removeFile:   RemoveGeneratedFile,
		remove:       os.Remove,
	})
}

func uninstallWith(ctx context.Context, root string, preserveTemplate func(string) bool, ops uninstallOps) (UninstallReport, error) {
	lockPath := config.LockPath(root)
	residentRoot := awfgit.ProjectResidentRoot(ctx, root)
	lock, found, err := manifest.LoadOptional(lockPath)
	if err != nil {
		return UninstallReport{}, err
	}
	if !found {
		return UninstallReport{}, fmt.Errorf("no %s: nothing to uninstall", filepath.Join(config.DirName, "awf.lock"))
	}
	preserved, err := ops.inspectRoots(residentRoot)
	if err != nil {
		return UninstallReport{}, err
	}
	report := UninstallReport{PreservedRoots: preserved}
	dirs := map[string]bool{}
	for path := range lock.Files {
		// A non-local entry (corrupted or malicious lock) would delete outside
		// the root. Runtime-shaped resident entries are corrupt and never removed.
		if !filepath.IsLocal(filepath.FromSlash(path)) || PreserveRemoval(path, preserved) {
			continue
		}
		abs := filepath.Join(root, path)
		if IsResidentPath(path) {
			abs = filepath.Join(residentRoot, filepath.FromSlash(path))
		}
		if preserveTemplate != nil && preserveTemplate(lock.Files[path].TemplateID) {
			backupRoot, backupPath := root, path
			if IsResidentPath(path) { // coverage-ignore: project policy recognizes only docs/local.md.tmpl, whose output path is never resident
				backupRoot = residentRoot
				backupPath = strings.TrimPrefix(path, config.DirName+"/")
			}
			handle, openErr := ops.open(backupRoot)
			if openErr != nil {
				return report, fmt.Errorf("open local-document root: %w", openErr)
			}
			info, statErr := handle.LinkInfo(backupPath)
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				_ = handle.Close()
				return report, fmt.Errorf("inspect local document %s: %w", path, statErr)
			}
			if statErr == nil {
				if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
					_ = handle.Close()
					return report, fmt.Errorf("unsafe local document %s", path)
				}
				bak, backupErr := handle.Backup(backupPath)
				if backupErr != nil {
					_ = handle.Close()
					return report, fmt.Errorf("back up local document %s: %w", path, backupErr)
				}
				report.Backups = append(report.Backups, Backup{Path: path, Bak: bak})
			}
			if closeErr := handle.Close(); closeErr != nil {
				return report, fmt.Errorf("close local-document root: %w", closeErr)
			}
		}
		removed, err := ops.removeFile(abs)
		if err != nil {
			return report, err
		}
		if removed {
			report.Removed++
		}
		base := root
		if IsResidentPath(path) {
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
	if err := ops.remove(lockPath); err != nil {
		return report, fmt.Errorf("remove lock: %w", err)
	}
	return report, nil
}
