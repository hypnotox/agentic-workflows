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
// under the config dir. Output planning, render, drift, ownership detection,
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

// UninstallReport records the simple path facts committed by an uninstall.
// Removed remains the compatibility summary.
type UninstallReport struct {
	Removed          int
	RemovedGenerated []string
	PreservedRoots   []string
	RemovedEmptyDirs []string
	LockRemoved      bool
}

// PartialUninstallError retains removed path facts when a later mutation fails.
type PartialUninstallError struct {
	Report UninstallReport
	Cause  error
}

func (e *PartialUninstallError) Error() string { return e.Cause.Error() }
func (e *PartialUninstallError) Unwrap() error { return e.Cause }
func (e *PartialUninstallError) Document() (presentation.Document, error) {
	return e.Report.PartialDocument()
}

func partialUninstall(report UninstallReport, cause error) error {
	return &PartialUninstallError{Report: report, Cause: cause}
}

// Document maps an uninstall result into its complete ordinary presentation.
func (r UninstallReport) Document() (presentation.Document, error) {
	note, err := presentation.Prose("the authored .awf config remains in place; delete it to fully remove")
	if err != nil {
		return presentation.Document{}, err
	}
	return r.document("uninstall completed", []presentation.Value{note}, nil)
}

// PartialDocument retains completed path facts and tells the operator how to
// converge after inspecting the failure.
func (r UninstallReport) PartialDocument() (presentation.Document, error) {
	next, err := presentation.Prose("inspect the reported cause, then rerun awf uninstall")
	if err != nil {
		return presentation.Document{}, err
	}
	return r.document("uninstall stopped", nil, []presentation.Value{next})
}

func (r UninstallReport) document(status string, prefix, next []presentation.Value) (presentation.Document, error) {
	removed, err := presentation.Literal(strconv.Itoa(r.Removed))
	if err != nil {
		return presentation.Document{}, err
	}
	removedField, err := presentation.NewField("generated files removed", removed)
	if err != nil {
		return presentation.Document{}, err
	}
	notes := append([]presentation.Value{}, prefix...)
	appendNote := func(text string) error {
		value, err := presentation.Prose(text)
		if err != nil {
			return err
		}
		notes = append(notes, value)
		return nil
	}
	for _, root := range r.PreservedRoots {
		if err := appendNote("preserved resident data under .awf/" + root); err != nil {
			return presentation.Document{}, err
		}
	}
	for _, path := range r.RemovedGenerated {
		if err := appendNote("removed generated " + path); err != nil {
			return presentation.Document{}, err
		}
	}
	for _, path := range r.RemovedEmptyDirs {
		if err := appendNote("removed empty directory " + path); err != nil {
			return presentation.Document{}, err
		}
	}
	if r.LockRemoved {
		if err := appendNote("removed lock .awf/awf.lock"); err != nil {
			return presentation.Document{}, err
		}
	}
	return (presentation.Mutation{Status: status, Identity: []presentation.Field{removedField}, Notes: notes, NextActions: next}).Document()
}

// InspectRoots examines direct children only. It never traverses a dynamic
// resident tree; a descendant other than the managed .gitignore keeps its root
// intact. Publisher's read-only inspection owns its existing host-path seam.
// InspectRoots is the read-only inspection entry point used by Publisher.
func InspectRoots(root string) ([]string, error) {
	files, err := filesystem.Open(root)
	if err != nil {
		return nil, err
	}
	defer files.Close()
	return inspectRootsConfined(files)
}

// inspectRootsConfined preserves resident policy while consuming the caller's
// already-open confined root capability.
func inspectRootsConfined(files interface {
	LinkInfo(string) (fs.FileInfo, error)
	ReadDir(string) ([]fs.DirEntry, error)
}) ([]string, error) {
	preserved := []string{}
	for _, name := range RootNames() {
		path := filepath.ToSlash(filepath.Join(config.DirName, name))
		info, err := files.LinkInfo(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect resident root %s: %w", name, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("unsafe resident root %s", name)
		}
		entries, err := files.ReadDir(path)
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

type uninstallHandle interface {
	ReadDir(string) ([]fs.DirEntry, error)
	LinkInfo(string) (fs.FileInfo, error)
	ExpectedIdentity(string) (*filesystem.ExpectedIdentity, error)
	ReadExpected(string, *filesystem.ExpectedIdentity) ([]byte, fs.FileMode, error)
	RemoveExpected(string, *filesystem.ExpectedIdentity) error
	RemoveExpectedRegularFile(string, *filesystem.ExpectedIdentity, []byte, fs.FileMode) error
	Close() error
}

type uninstallCandidate struct {
	path     string
	handle   uninstallHandle
	identity *filesystem.ExpectedIdentity
	contents []byte
	mode     fs.FileMode
}

func emptyLocalDocumentShell(contents []byte) bool {
	const boundary = "<!-- awf:edit-in-place body -->"
	lines := strings.Split(string(contents), "\n")
	for index, line := range lines {
		if strings.TrimSpace(line) == boundary {
			return strings.TrimSpace(strings.Join(lines[index+1:], "\n")) == ""
		}
	}
	return false
}

type uninstallOps struct {
	open         func(string) (uninstallHandle, error)
	residentRoot func(context.Context, string) string
	inspectRoots func(uninstallHandle) ([]string, error)
}

func productionUninstallOpen(root string) (uninstallHandle, error) {
	handle, err := filesystem.Open(root)
	if err != nil {
		return nil, err
	}
	return handle, nil
}

const uninstallLockPath = ".awf/awf.lock"

// UninstallLeased removes awf's generated footprint while preserving dynamic
// resident state. The caller supplies the live dual-root lease acquired before
// authority loading; this operation verifies it before reading the lock.
func UninstallLeased(ctx context.Context, root string, preserveTemplate func(string) bool, lease *filesystem.Lease) (UninstallReport, error) {
	residentRoot := awfgit.ProjectResidentRoot(ctx, root)
	if !lease.CoversProject(root, residentRoot) {
		return UninstallReport{}, fmt.Errorf("uninstall requires a live project lease")
	}
	return uninstallWith(ctx, root, preserveTemplate, uninstallOps{open: productionUninstallOpen, residentRoot: func(context.Context, string) string { return residentRoot }, inspectRoots: func(handle uninstallHandle) ([]string, error) { return inspectRootsConfined(handle) }})
}

func uninstallWith(ctx context.Context, root string, preserveTemplate func(string) bool, ops uninstallOps) (report UninstallReport, returnErr error) {
	residentRoot := root
	if ops.residentRoot != nil {
		residentRoot = ops.residentRoot(ctx, root)
	}
	tracked, err := ops.open(root)
	if err != nil {
		return report, fmt.Errorf("open tracked uninstall root: %w", err)
	}
	trackedClosed := false
	defer func() {
		if !trackedClosed {
			returnErr = errors.Join(returnErr, tracked.Close())
		}
	}()
	resident := tracked
	residentClosed := true
	if residentRoot != root {
		resident, err = ops.open(residentRoot)
		if err != nil {
			return report, fmt.Errorf("open resident uninstall root: %w", err)
		}
		residentClosed = false
		defer func() {
			if !residentClosed {
				returnErr = errors.Join(returnErr, resident.Close())
			}
		}()
	}
	lockInfo, err := tracked.ExpectedIdentity(uninstallLockPath)
	if errors.Is(err, fs.ErrNotExist) {
		return report, fmt.Errorf("no %s: nothing to uninstall", filepath.Join(config.DirName, "awf.lock"))
	}
	if err != nil {
		return report, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
	}
	defer func() {
		if lockInfo != nil {
			_ = lockInfo.Release()
		}
	}()
	if lockInfo.Mode()&fs.ModeSymlink != 0 || !lockInfo.Mode().IsRegular() {
		return report, fmt.Errorf("unreadable .awf/awf.lock (unsafe lock): restore it from version control, or delete it deliberately to re-adopt")
	}
	lockBytes, lockMode, err := tracked.ReadExpected(uninstallLockPath, lockInfo)
	if err != nil {
		return report, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
	}
	lock, err := manifest.Parse(lockBytes)
	if err != nil {
		return report, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
	}
	preserved, err := ops.inspectRoots(resident)
	if err != nil {
		return report, err
	}
	report.PreservedRoots = preserved
	paths := slices.Collect(maps.Keys(lock.Files))
	slices.Sort(paths)
	dirs := map[string]uninstallHandle{}
	candidates := make([]uninstallCandidate, 0, len(paths))
	defer func() {
		for _, candidate := range candidates {
			if candidate.identity != nil {
				_ = candidate.identity.Release()
			}
		}
	}()

	// Preflight every locked output before removing any of them. The retained
	// identities and exact images make the later removal conditional on the same
	// regular files still being present with the lock-owned bytes and mode.
	for _, path := range paths {
		if PreserveRemoval(path, preserved) {
			continue
		}
		handle := tracked
		if IsResidentPath(path) {
			handle = resident
		}
		identity, identityErr := handle.ExpectedIdentity(path)
		if errors.Is(identityErr, fs.ErrNotExist) {
			continue
		}
		if identityErr != nil {
			return report, fmt.Errorf("inspect generated file %s: %w", path, identityErr)
		}
		if identity.Mode()&fs.ModeSymlink != 0 || !identity.Mode().IsRegular() {
			_ = identity.Release()
			return report, fmt.Errorf("refuse unsafe locked output %s: expected an ordinary file", path)
		}
		contents, mode, readErr := handle.ReadExpected(path, identity)
		if readErr != nil {
			_ = identity.Release()
			return report, fmt.Errorf("inspect generated file %s: %w", path, readErr)
		}
		entry := lock.Files[path]
		expectedMode := fs.FileMode(entry.Mode)
		if expectedMode == 0 {
			expectedMode = 0o644
		}
		if manifest.Hash(contents) != entry.OutputHash || mode.Perm() != expectedMode.Perm() {
			_ = identity.Release()
			return report, fmt.Errorf("refuse diverged locked output %s: current bytes and mode must match the lock-owned image", path)
		}
		if preserveTemplate != nil && preserveTemplate(entry.TemplateID) && !emptyLocalDocumentShell(contents) {
			_ = identity.Release()
			return report, fmt.Errorf("refuse local document %s: protected authored body is present", path)
		}
		candidates = append(candidates, uninstallCandidate{path: path, handle: handle, identity: identity, contents: contents, mode: mode})
		for dir := filepath.ToSlash(filepath.Dir(path)); dir != "." && dir != "/"; dir = filepath.ToSlash(filepath.Dir(dir)) {
			dirs[dir] = handle
		}
	}
	for index := range candidates {
		candidate := &candidates[index]
		removeErr := candidate.handle.RemoveExpectedRegularFile(candidate.path, candidate.identity, candidate.contents, candidate.mode)
		candidate.identity = nil // RemoveExpectedRegularFile consumes the identity even on failure.
		if removeErr != nil {
			return report, partialUninstall(report, fmt.Errorf("remove generated file %s: %w", candidate.path, removeErr))
		}
		report.Removed++
		report.RemovedGenerated = append(report.RemovedGenerated, candidate.path)
	}
	dirList := slices.Collect(maps.Keys(dirs))
	slices.SortFunc(dirList, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return strings.Compare(a, b)
	})
	for _, dir := range dirList {
		info, err := dirs[dir].ExpectedIdentity(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return report, partialUninstall(report, fmt.Errorf("inspect empty directory %s: %w", dir, err))
		}
		if !info.IsDir() {
			_ = info.Release()
			continue
		}
		if err := dirs[dir].RemoveExpected(dir, info); err != nil {
			if errors.Is(err, filesystem.ErrDirectoryNotEmpty) {
				continue
			}
			return report, partialUninstall(report, fmt.Errorf("remove empty directory %s: %w", dir, err))
		}
		report.RemovedEmptyDirs = append(report.RemovedEmptyDirs, filepath.ToSlash(dir))
	}
	removeLockErr := tracked.RemoveExpectedRegularFile(uninstallLockPath, lockInfo, lockBytes, lockMode)
	lockInfo = nil // RemoveExpectedRegularFile consumes the identity even on failure.
	if removeLockErr != nil {
		return report, partialUninstall(report, fmt.Errorf("remove lock: %w", removeLockErr))
	}
	report.LockRemoved = true
	if resident != tracked {
		if err := resident.Close(); err != nil {
			return report, partialUninstall(report, fmt.Errorf("close resident uninstall root: %w", err))
		}
		residentClosed = true
	}
	if err := tracked.Close(); err != nil {
		return report, partialUninstall(report, fmt.Errorf("close tracked uninstall root: %w", err))
	}
	trackedClosed = true
	return report, nil
}
