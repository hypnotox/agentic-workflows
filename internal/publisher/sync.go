// Package publisher owns application-level output publication.
package publisher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/configcheck"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

func validatePublicationArtifact(content []byte) error {
	return generatedcheck.ValidateFrontmatter(content)
}

func invalidSkillArtifact(path string, err error) error {
	return fmt.Errorf("invalid skill artifact in %s: %w", path, err)
}

// Change records a sync-written file whose rendered bytes differ from the
// prior lock's or whose required mode was corrected, with the cause the lock's
// hashes able to attribute: "template", "config", "template+config",
// "internal", "regenerated", or "added".
type Change struct {
	Path  string
	Cause string
}

// Initialize derives and publishes a first adoption through one ordered publication.
func (p *Publisher) Initialize(seed InitAuthority) (Result, error) {
	return p.run(context.Background(), nil, &seed, migrate.Current(), true)
}

// SyncLeased derives and publishes under a lease acquired before mutable
// authority loading by the operation owner.
func (p *Publisher) SyncLeased(ctx context.Context, lease *filesystem.Lease) (Result, error) {
	return p.run(ctx, lease, nil, migrate.Current(), true)
}

// PreflightSyncLeased completely validates ordinary publication against the
// current filesystem without applying any planned mutation.
func (p *Publisher) PreflightSyncLeased(ctx context.Context, lease *filesystem.Lease) error {
	_, err := p.run(ctx, lease, nil, migrate.Current(), false)
	return err
}

// SyncUpgradeLeased publishes a supported live-schema migration without an
// intermediate lock write. Publisher consumes the old lock's ownership
// inventory and writes the complete current lock last.
func (p *Publisher) SyncUpgradeLeased(ctx context.Context, lease *filesystem.Lease, floor int) (Result, error) {
	return p.run(ctx, lease, nil, floor, true)
}

// InitializeLeased derives and publishes first-adoption output under the
// operation's pre-authority lease.
func (p *Publisher) InitializeLeased(ctx context.Context, lease *filesystem.Lease, seed InitAuthority) (Result, error) {
	if lease == nil {
		return p.Initialize(seed)
	}
	return p.run(ctx, lease, &seed, migrate.Current(), true)
}

// run owns publication. Lease acquisition precedes planning, lock observation,
// resident inspection, preflight, and every mutation.
func (p *Publisher) run(ctx context.Context, supplied *filesystem.Lease, seed *InitAuthority, lockFloor int, apply bool) (result Result, returnErr error) {
	if err := p.beginMutation(); err != nil {
		return Result{}, err
	}
	roots := p.inputs.session.Roots()
	lease := supplied
	owned := false
	if lease == nil {
		var err error
		lease, err = filesystem.AcquireProjectLease(ctx, roots.Tracked, roots.Resident)
		if err != nil {
			return Result{}, err
		}
		owned = true
	} else if !lease.CoversProject(roots.Tracked, roots.Resident) {
		return Result{}, errors.New("publisher: supplied lease does not cover project roots")
	}
	if owned {
		defer func() {
			if err := lease.Release(); err != nil {
				returnErr = errors.Join(returnErr, err)
			}
		}()
	}
	if seed == nil {
		if err := configcheck.ValidateCommandWiring(p.inputs.cfg); err != nil {
			return Result{}, err
		}
	}
	p.allowPublicationPlanning()
	plan, err := p.Plan()
	if err != nil {
		return Result{}, err
	}
	return p.sync(seed, &plan, lockFloor, apply)
}

// InitAuthority is the explicit provenance supplied only by first adoption.
type InitAuthority struct{ InitializedWithVersion string }

// Result records successful output changes and removals, including those made
// before a later mutation failed.
type Result struct {
	changes []Change
	pruned  []string
	touched []string
}

func newResult(changes []Change, pruned, touched []string) Result {
	return Result{changes: slices.Clone(changes), pruned: slices.Clone(pruned), touched: slices.Clone(touched)}
}
func (r Result) Changes() []Change { return slices.Clone(r.changes) }
func (r Result) Pruned() []string  { return slices.Clone(r.pruned) }
func (r Result) Touched() []string { return slices.Clone(r.touched) }

// Mutation maps this semantic result to the central presentation grammar.
func (r Result) Mutation() (presentation.Mutation, error) {
	return syncMutation(r.Changes(), r.Pruned())
}

// FailureMutation projects every successfully touched path for a caller that
// must report an incomplete publication without implying rollback state.
func (r Result) FailureMutation() (presentation.Mutation, error) {
	values := make([]presentation.Value, 0, len(r.touched))
	for _, path := range r.touched {
		value, err := presentation.Literal(path)
		if err != nil {
			return presentation.Mutation{}, err
		}
		values = append(values, value)
	}
	mutation := presentation.Mutation{Status: "incomplete"}
	if len(values) != 0 {
		mutation.Changes = []presentation.MutationChange{{Label: "touched path", Values: values}}
	}
	return mutation, nil
}

// syncFilesystem is sync's cohesive, root-confined filesystem dependency.
type syncFilesystem interface {
	MkdirAll(string, fs.FileMode) error
	Chmod(string, fs.FileMode) error
	ReplaceExpected(string, *filesystem.ExpectedIdentity, []byte, fs.FileMode) error
	ReplaceExpectedRegularFile(string, *filesystem.ExpectedIdentity, []byte, fs.FileMode, []byte, fs.FileMode) error
	RemoveExpected(string, *filesystem.ExpectedIdentity) error
	RemoveExpectedRegularFile(string, *filesystem.ExpectedIdentity, []byte, fs.FileMode) error
	ReadWithMode(string) ([]byte, fs.FileMode, error)
	LinkInfo(string) (fs.FileInfo, error)
	ExpectedIdentity(string) (*filesystem.ExpectedIdentity, error)
}

type syncFilesystems struct {
	tracked  syncFilesystem
	resident syncFilesystem
}

func (s syncFilesystems) output(rel string) (syncFilesystem, string) {
	if resident.IsResidentPath(rel) {
		return s.resident, rel
	}
	return s.tracked, rel
}

func openSyncFilesystems(p renderInputs) (syncFilesystems, func(), error) {
	tracked, err := filesystem.Open(p.session.Roots().Tracked)
	if err != nil {
		return syncFilesystems{}, nil, err
	}
	closeAll := func() { _ = tracked.Close() }
	if p.session.Roots().Resident == p.session.Roots().Tracked {
		return syncFilesystems{tracked: tracked, resident: tracked}, closeAll, nil
	}
	residentHandle, err := filesystem.Open(p.session.Roots().Resident)
	if err != nil {
		closeAll()
		return syncFilesystems{}, nil, err
	}
	return syncFilesystems{tracked: tracked, resident: residentHandle}, func() {
		_ = residentHandle.Close()
		_ = tracked.Close()
	}, nil
}

func (p *Publisher) sync(seed *InitAuthority, op *outputplan.Plan, lockFloor int, apply bool) (Result, error) {
	filesystems, closeAll, err := openSyncFilesystems(p.inputs)
	if err != nil {
		return Result{}, err
	}
	defer closeAll()
	var touched []string
	changes, pruned, err := syncReportWithPlan(p.inputs, seed, filesystems, op, lockFloor, apply, &touched)
	return newResult(changes, pruned, touched), err
}

type desiredPublication struct {
	path, outputPath string
	filesystem       syncFilesystem
	contents         []byte
	mode             fs.FileMode
	expected         *filesystem.ExpectedIdentity
	observed         []byte
	observedMode     fs.FileMode
	mutate           bool
	change           *Change
}

type retiredPublication struct {
	path, outputPath string
	filesystem       syncFilesystem
	expected         *filesystem.ExpectedIdentity
	observed         []byte
	mode             fs.FileMode
}

type directoryCorrection struct {
	path       string
	filesystem syncFilesystem
	mode       fs.FileMode
}

func syncReportWithPlan(p renderInputs, seed *InitAuthority, filesystems syncFilesystems, op *outputplan.Plan, lockFloor int, apply bool, touchedOut *[]string) (changes []Change, pruned []string, err error) {
	touched := []string(nil)
	defer func() {
		slices.Sort(pruned)
		slices.Sort(touched)
		touched = slices.Compact(touched)
		if touchedOut != nil {
			*touchedOut = slices.Clone(touched)
		}
		slices.SortFunc(changes, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	}()

	lockPath := path.Join(config.DirName, "awf.lock")
	lockIdentity, identityErr := filesystems.tracked.ExpectedIdentity(lockPath)
	if identityErr != nil && !errors.Is(identityErr, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("inspect %s: %w", lockPath, identityErr)
	}
	if lockIdentity != nil {
		defer func(identity *filesystem.ExpectedIdentity) { _ = identity.Release() }(lockIdentity)
		if !lockIdentity.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("inspect %s: destination is not a regular file", lockPath)
		}
	}
	var lockBytes []byte
	var lockMode fs.FileMode
	var old *manifest.Lock
	found := lockIdentity != nil
	if found {
		lockBytes, lockMode, err = filesystems.tracked.ReadWithMode(lockPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", lockPath, err)
		}
		old, err = manifest.ParseLive(lockBytes, lockFloor, migrate.Current())
		if err != nil {
			return nil, nil, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
		}
	}
	if seed != nil {
		if found {
			return nil, nil, errors.New("first-adoption initialization requires an absent lock")
		}
	} else {
		if !found {
			return nil, nil, errors.New("pre-tracking authority: ordinary sync requires a supported permanent lock; restore .awf/awf.lock from version control")
		}
	}

	preservedResidents, err := resident.InspectRoots(p.session.Roots().Resident)
	if err != nil {
		return nil, nil, err
	}
	files := op.Outputs()
	for _, f := range files {
		if f.Policy().ValidateFrontmatter {
			if err := validatePublicationArtifact([]byte(f.Content())); err != nil {
				return nil, nil, invalidSkillArtifact(f.Path(), err)
			}
		}
	}

	lock := &manifest.Lock{AWFVersion: p.version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if old != nil {
		lock.InitializedWithVersion = old.InitializedWithVersion
	} else {
		lock.InitializedWithVersion = seed.InitializedWithVersion
	}
	prior := map[string]manifest.Entry{}
	if old != nil {
		for name, entry := range old.Files {
			prior[name] = entry
		}
	}

	var desired []desiredPublication
	var retired []retiredPublication
	defer func() {
		for i := range desired {
			if desired[i].expected != nil {
				_ = desired[i].expected.Release()
			}
		}
		for i := range retired {
			if retired[i].expected != nil {
				_ = retired[i].expected.Release()
			}
		}
	}()
	missingParents := map[string]directoryCorrection{}
	corrections := map[string]directoryCorrection{}
	want := map[string]bool{}
	for _, f := range files {
		contents := []byte(f.Content())
		mode := fs.FileMode(0o644)
		if bytes.HasPrefix(contents, []byte("#!")) {
			mode = 0o755
		}
		entry := manifest.Entry{
			TemplateID: f.TemplateID(), TemplateHash: f.TemplateHash(),
			ConfigHash: f.ConfigHash(), OutputHash: manifest.Hash(contents),
			Mode: uint32(mode.Perm()), RegenChecked: f.RegenChecked(),
		}
		lock.Files[f.Path()] = entry
		want[f.Path()] = true

		filesystem, outputPath := filesystems.output(f.Path())
		if err := preflightParents(filesystem, path.Dir(outputPath), missingParents); err != nil {
			return nil, nil, fmt.Errorf("preflight desired output %s: %w", f.Path(), err)
		}
		expected, observeErr := filesystem.ExpectedIdentity(outputPath)
		if observeErr != nil && !errors.Is(observeErr, fs.ErrNotExist) {
			return nil, nil, fmt.Errorf("inspect desired output %s: %w", f.Path(), observeErr)
		}
		d := desiredPublication{path: f.Path(), outputPath: outputPath, filesystem: filesystem, contents: contents, mode: mode, expected: expected}
		if expected == nil {
			d.mutate = true
		} else {
			if !expected.Mode().IsRegular() {
				_ = expected.Release()
				return nil, nil, fmt.Errorf("inspect desired output %s: destination is not a regular file", f.Path())
			}
			d.observed, d.observedMode, err = filesystem.ReadWithMode(outputPath)
			if err != nil {
				_ = expected.Release()
				return nil, nil, fmt.Errorf("read desired output %s: %w", f.Path(), err)
			}
			exact := bytes.Equal(d.observed, contents) && d.observedMode.Perm() == mode.Perm()
			if _, managed := prior[f.Path()]; !managed && !exact {
				_ = expected.Release()
				return nil, nil, fmt.Errorf("refuse unmanaged output %s: existing regular file differs from desired bytes or mode", f.Path())
			}
			d.mutate = !exact
		}
		if d.mutate {
			change := classifyChange(old, f.Path(), f.TemplateHash(), f.ConfigHash(), f.RegenChecked())
			d.change = &change
		}
		desired = append(desired, d)

		if strings.HasPrefix(f.Path(), config.DirName+"/") && strings.HasSuffix(f.Path(), "/.gitignore") && resident.IsResidentPath(strings.TrimSuffix(f.Path(), "/.gitignore")) {
			dir := path.Dir(outputPath)
			info, infoErr := filesystem.LinkInfo(dir)
			if infoErr != nil && !errors.Is(infoErr, fs.ErrNotExist) {
				return nil, nil, fmt.Errorf("inspect resident directory mode %s: %w", dir, infoErr)
			}
			if infoErr == nil && !info.IsDir() {
				return nil, nil, fmt.Errorf("inspect resident directory mode %s: destination is not a directory", dir)
			}
			if errors.Is(infoErr, fs.ErrNotExist) || info.Mode().Perm() != 0o700 {
				key := fmt.Sprintf("%p:%s", filesystem, dir)
				corrections[key] = directoryCorrection{path: dir, filesystem: filesystem, mode: 0o700}
			}
		}
	}

	if old != nil {
		for _, retiredPath := range sortedStringKeys(old.Files) {
			entry := old.Files[retiredPath]
			if want[retiredPath] || resident.PreserveRemoval(retiredPath, preservedResidents) {
				continue
			}
			filesystem, outputPath := filesystems.output(retiredPath)
			expected, observeErr := filesystem.ExpectedIdentity(outputPath)
			if errors.Is(observeErr, fs.ErrNotExist) {
				continue
			}
			if observeErr != nil {
				return nil, nil, fmt.Errorf("inspect retired output %s: %w", retiredPath, observeErr)
			}
			if !expected.Mode().IsRegular() {
				_ = expected.Release()
				return nil, nil, fmt.Errorf("inspect retired output %s: destination is not a regular file", retiredPath)
			}
			observed, mode, readErr := filesystem.ReadWithMode(outputPath)
			if readErr != nil {
				_ = expected.Release()
				return nil, nil, fmt.Errorf("read retired output %s: %w", retiredPath, readErr)
			}
			expectedMode := fs.FileMode(entry.Mode)
			if expectedMode == 0 {
				expectedMode = 0o644
			}
			if manifest.Hash(observed) != entry.OutputHash || mode.Perm() != expectedMode.Perm() {
				_ = expected.Release()
				return nil, nil, fmt.Errorf("refuse retired output %s: existing regular file differs from locked bytes or mode", retiredPath)
			}
			retired = append(retired, retiredPublication{path: retiredPath, outputPath: outputPath, filesystem: filesystem, expected: expected, observed: observed, mode: mode})
		}
	}

	desiredLockBytes, err := lock.Marshal()
	if err != nil {
		return nil, nil, fmt.Errorf("marshal %s: %w", lockPath, err)
	}
	if !apply {
		return nil, nil, nil
	}

	parents := mapValues(missingParents)
	slices.SortFunc(parents, func(a, b directoryCorrection) int {
		if da, db := strings.Count(a.path, "/"), strings.Count(b.path, "/"); da != db {
			return da - db
		}
		return strings.Compare(a.path, b.path)
	})
	for _, dir := range parents {
		if err := dir.filesystem.MkdirAll(dir.path, 0o755); err != nil {
			return changes, pruned, fmt.Errorf("create parent directory %s: %w", dir.path, err)
		}
		touched = append(touched, dir.path)
	}
	modeDirs := mapValues(corrections)
	slices.SortFunc(modeDirs, func(a, b directoryCorrection) int { return strings.Compare(a.path, b.path) })
	for _, dir := range modeDirs {
		if err := dir.filesystem.Chmod(dir.path, dir.mode); err != nil {
			return changes, pruned, fmt.Errorf("set resident directory mode %s: %w", dir.path, err)
		}
		touched = append(touched, dir.path)
	}
	for i := range desired {
		d := &desired[i]
		if !d.mutate {
			continue
		}
		expected := d.expected
		d.expected = nil
		if expected == nil {
			err = d.filesystem.ReplaceExpected(d.outputPath, nil, d.contents, d.mode)
		} else {
			err = d.filesystem.ReplaceExpectedRegularFile(d.outputPath, expected, d.observed, d.observedMode, d.contents, d.mode)
		}
		if err != nil {
			return changes, pruned, fmt.Errorf("publish desired output %s: %w", d.path, err)
		}
		touched = append(touched, d.path)
		changes = append(changes, *d.change)
	}

	type cleanupDir struct {
		filesystem syncFilesystem
		path       string
		resident   bool
	}
	cleanup := map[string]cleanupDir{}
	for i := range retired {
		r := &retired[i]
		expected := r.expected
		r.expected = nil
		if err := r.filesystem.RemoveExpectedRegularFile(r.outputPath, expected, r.observed, r.mode); err != nil {
			return changes, pruned, fmt.Errorf("remove retired output %s: %w", r.path, err)
		}
		touched = append(touched, r.path)
		pruned = append(pruned, r.path)
		for dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(r.outputPath))); dir != "."; dir = filepath.ToSlash(filepath.Dir(filepath.FromSlash(dir))) {
			key := fmt.Sprintf("%p:%s", r.filesystem, dir)
			cleanup[key] = cleanupDir{filesystem: r.filesystem, path: dir, resident: resident.IsResidentPath(r.path)}
		}
	}
	cleanupDirs := mapValues(cleanup)
	slices.SortFunc(cleanupDirs, func(a, b cleanupDir) int {
		if da, db := strings.Count(a.path, "/"), strings.Count(b.path, "/"); da != db {
			return db - da
		}
		if a.resident != b.resident {
			if a.resident {
				return 1
			}
			return -1
		}
		return strings.Compare(a.path, b.path)
	})
	for _, dir := range cleanupDirs {
		identity, inspectErr := dir.filesystem.ExpectedIdentity(dir.path)
		if errors.Is(inspectErr, fs.ErrNotExist) {
			continue
		}
		if inspectErr != nil {
			return changes, pruned, fmt.Errorf("inspect empty ancestor %s: %w", dir.path, inspectErr)
		}
		if !identity.IsDir() {
			_ = identity.Release()
			continue
		}
		if err := dir.filesystem.RemoveExpected(dir.path, identity); err != nil {
			if errors.Is(err, filesystem.ErrDirectoryNotEmpty) {
				continue
			}
			return changes, pruned, fmt.Errorf("remove empty ancestor %s: %w", dir.path, err)
		}
		touched = append(touched, dir.path)
	}

	if found && bytes.Equal(lockBytes, desiredLockBytes) && lockMode.Perm() == 0o644 {
		return changes, pruned, nil
	}
	expectedLock := lockIdentity
	if expectedLock == nil {
		err = filesystems.tracked.ReplaceExpected(lockPath, nil, desiredLockBytes, 0o644)
	} else {
		err = filesystems.tracked.ReplaceExpectedRegularFile(lockPath, expectedLock, lockBytes, lockMode, desiredLockBytes, 0o644)
	}
	if err != nil {
		return changes, pruned, fmt.Errorf("publish %s: %w", lockPath, err)
	}
	touched = append(touched, lockPath)
	return changes, pruned, nil
}

func preflightParents(filesystem syncFilesystem, dir string, missing map[string]directoryCorrection) error {
	if dir == "." {
		return nil
	}
	var ancestors []string
	for current := dir; current != "."; current = path.Dir(current) {
		ancestors = append(ancestors, current)
	}
	slices.Reverse(ancestors)
	for _, current := range ancestors {
		info, err := filesystem.LinkInfo(current)
		if errors.Is(err, fs.ErrNotExist) {
			key := fmt.Sprintf("%p:%s", filesystem, current)
			missing[key] = directoryCorrection{path: current, filesystem: filesystem}
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect parent %s: %w", current, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("parent %s is not a directory", current)
		}
	}
	return nil
}

func classifyChange(old *manifest.Lock, outputPath, templateHash, configHash string, regenerated bool) Change {
	if old == nil {
		return Change{Path: outputPath, Cause: "added"}
	}
	oldEntry, ok := old.Files[outputPath]
	if !ok {
		return Change{Path: outputPath, Cause: "added"}
	}
	templateMoved, configMoved := templateHash != oldEntry.TemplateHash, configHash != oldEntry.ConfigHash
	cause := "internal"
	switch {
	case templateMoved && configMoved:
		cause = "template+config"
	case templateMoved:
		cause = "template"
	case configMoved:
		cause = "config"
	case regenerated:
		cause = "regenerated"
	}
	return Change{Path: outputPath, Cause: cause}
}

func sortedStringKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func mapValues[V any](values map[string]V) []V {
	out := make([]V, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

// SyncMutation maps the completed sync outcome into presentation-owned syntax.
func syncMutation(changes []Change, pruned []string) (presentation.Mutation, error) {
	groups := make([]presentation.MutationChange, 0, 2)
	if len(changes) > 0 {
		values := make([]presentation.Value, 0, len(changes))
		for _, change := range changes {
			text := change.Path
			if change.Cause == "added" {
				text = "added " + text
			} else {
				text = fmt.Sprintf("changed %s (%s)", text, change.Cause)
			}
			value, err := presentation.Literal(text)
			if err != nil {
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		groups = append(groups, presentation.MutationChange{Label: "outputs", Values: values})
	}
	if len(pruned) > 0 {
		values := make([]presentation.Value, 0, len(pruned))
		for _, path := range pruned {
			value, err := presentation.Literal(path)
			if err != nil {
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		groups = append(groups, presentation.MutationChange{Label: "pruned", Values: values})
	}
	next, err := presentation.Prose("continue with the rendered project state")
	if err != nil {
		return presentation.Mutation{}, err
	}
	return presentation.Mutation{Status: "completed", Changes: groups, NextActions: []presentation.Value{next}}, nil
}

// InitCollisions uses the path-only definition projection.  In particular it
// must remain safe before init prompts: probing a foreign path is not an
// authoritative render pass and never executes a template.
func (p *Publisher) InitCollisions() ([]string, error) {
	return p.InitCollisionsAt(p.inputs.root())
}

// InitCollisionsAt probes only the supplied filesystem paths.  It intentionally
// builds definitions, not an operation plan, so initialization can refuse a
// foreign file before prompting or executing a render closure.
func (p *Publisher) InitCollisionsAt(root string) ([]string, error) {
	definitions, err := buildOutputDefinitions(p.inputs.cfg, projectCatalog(p.inputs), p.inputs.targets(), projectTreeReader(p.inputs))
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		paths = append(paths, definition.Path)
	}
	return resident.CollisionsAt(root, paths)
}

// IsLocalDocTemplate is the bounded recognition policy outer composition passes to uninstall.
func IsLocalDocTemplate(templateID string) bool { return templateID == localDocTID }
