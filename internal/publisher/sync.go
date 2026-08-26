// Package publisher owns application-level output publication.
package publisher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
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

// validatePublicationArtifact preserves Publisher's dialect-compatible callback.
func validatePublicationArtifact(content []byte, _ AgentDialect) error {
	return generatedcheck.ValidateFrontmatter(content)
}

// Backup records a foreign file preserved before sync overwrote its path.
type Backup struct {
	Path  string // project-relative file that was overwritten
	Bak   string // project-relative backup copy (.awf-bak[.N])
	Index bool   // the file is the generated ADR/domain index (ownership-takeover note)
}

// Change records a sync-written file whose rendered bytes differ from the
// prior lock's or whose required mode was corrected, with the cause the lock's
// hashes able to attribute: "template"
// (the upstream template source moved), "config" (the project's effective
// inputs - vars, sidecar, parts - moved), "template+config" (both),
// "internal" (hashes unmoved: a non-hashed input such as the binary's version
// stamp), "regenerated" (a generated index, which carries no hashes to
// attribute), or "added" (no prior entry). The provenance triage signal for
// reviewing a large sync diff - upstream churn vs the project's own inputs.
type Change struct {
	Path  string
	Cause string
}

// Sync renders and writes the project, additionally backing up any
// foreign file (on disk but absent from the start-of-sync lock) before overwriting
// it and returning those backups (ADR-0035) plus the per-file provenance of
// output that changed against the prior lock and the lock-relative paths of the
// files its prune actually removed (both path-sorted; a file whose output is
// byte-identical, and first-adoption initialization with no prior lock reports
// no change - a routine re-sync stays silent).
func (p *Publisher) Sync() (Result, error) { return p.run(context.Background(), nil, nil) }

// Initialize derives and publishes a first adoption in one transaction.
func (p *Publisher) Initialize(seed InitAuthority) (Result, error) {
	return p.run(context.Background(), nil, &seed)
}

// SyncLeased derives and publishes under a lease acquired before mutable
// authority loading by the operation owner.
func (p *Publisher) SyncLeased(ctx context.Context, lease *filesystem.Lease) (Result, error) {
	return p.run(ctx, lease, nil)
}

// InitializeLeased derives and publishes first-adoption output under the
// operation's pre-authority lease. Preparation intentionally has no mutator.
func (p *Publisher) InitializeLeased(ctx context.Context, lease *filesystem.Lease, seed InitAuthority) (Result, error) {
	if lease == nil {
		return p.Initialize(seed)
	}
	return p.run(ctx, lease, &seed)
}

// run owns the complete publication transaction. Lease acquisition precedes
// planning, mutable lock observation, resident inspection, and every effect.
func (p *Publisher) run(ctx context.Context, supplied *filesystem.Lease, seed *InitAuthority) (result Result, returnErr error) {
	roots := p.inputs.state.Roots()
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
				returnErr = partialOrError(result, errors.Join(returnErr, err))
			}
		}()
	}
	if seed == nil {
		if err := configcheck.ValidateCommandWiring(p.inputs.cfg); err != nil {
			return Result{}, err
		}
	}
	prepared, err := p.Prepare()
	if err != nil {
		return Result{}, err
	}
	return p.sync(seed, &prepared.plan)
}

// InitAuthority is the explicit provenance supplied only by first adoption.
type InitAuthority struct{ InitializedWithVersion string }

// Result records only mutations committed before the terminal outcome.
type Result struct {
	backups []Backup
	changes []Change
	pruned  []string
	effects []Effect
}

// Effect is one committed filesystem fact and its stable retry or recovery action.
type Effect struct {
	Kind     string
	Path     string
	Recovery string
}

// PartialError reports a failed publication after one or more committed effects.
type PartialError struct {
	Result Result
	Cause  error
}

func (e *PartialError) Error() string { return "publication partially committed: " + e.Cause.Error() }
func (e *PartialError) Unwrap() error { return e.Cause }

func newResult(backups []Backup, changes []Change, pruned []string, effects []Effect) Result {
	return Result{backups: slices.Clone(backups), changes: slices.Clone(changes), pruned: slices.Clone(pruned), effects: slices.Clone(effects)}
}
func (r Result) Backups() []Backup { return slices.Clone(r.backups) }
func (r Result) Changes() []Change { return slices.Clone(r.changes) }
func (r Result) Pruned() []string  { return slices.Clone(r.pruned) }
func (r Result) Effects() []Effect { return slices.Clone(r.effects) }

// HasCommittedEffects reports whether publication crossed a mutation boundary.
// It lets a composing operation retain Publisher's owner-rendered partial
// outcome without depending on Publisher's concrete effect representation.
func (r Result) HasCommittedEffects() bool { return len(r.effects) != 0 }
func (r Result) committed() bool           { return r.HasCommittedEffects() }

func partialOrError(result Result, err error) error {
	if err == nil || !result.committed() {
		return err
	}
	var partial *PartialError
	if errors.As(err, &partial) {
		return err
	}
	return &PartialError{Result: result, Cause: err}
}

// Mutation maps this semantic result to the central presentation grammar.
func (r Result) Mutation() (presentation.Mutation, error) {
	return syncMutation(r.Backups(), r.Changes(), r.Pruned())
}

// PartialMutation presents every committed effect and its recovery action.
func (r Result) PartialMutation() (presentation.Mutation, error) {
	values := make([]presentation.Value, 0, len(r.effects))
	for _, effect := range r.effects {
		value, err := presentation.Literal(fmt.Sprintf("%s %s; recovery: %s", effect.Kind, effect.Path, effect.Recovery))
		if err != nil {
			return presentation.Mutation{}, err
		}
		values = append(values, value)
	}
	next, err := presentation.Prose("apply the listed recovery actions, then rerun awf render")
	if err != nil { // coverage-ignore: fixed presentation text
		return presentation.Mutation{}, err
	}
	changes := []presentation.MutationChange(nil)
	if len(values) > 0 {
		changes = append(changes, presentation.MutationChange{Label: "committed effects", Values: values})
	}
	return presentation.Mutation{
		Status:      "partially committed",
		Changes:     changes,
		NextActions: []presentation.Value{next},
	}, nil
}

func committedPublication(err error) (string, string, bool) {
	return filesystem.CommittedPublication(err)
}

func appendCommittedOperationEffects(effects []Effect, err error, effect Effect) ([]Effect, bool) {
	committedPath, residuePath, committed := committedPublication(err)
	if !committed {
		return effects, false
	}
	if effect.Path == "" {
		effect.Path = committedPath
	}
	effects = append(effects, effect)
	if residuePath != "" {
		effects = append(effects, Effect{Kind: "publication-residue", Path: residuePath, Recovery: "remove this temporary residue, then rerun awf render"})
	}
	return effects, true
}

func backupFileConfined(rel string, handle syncFilesystem) (string, error) {
	return filesystem.Backup(rel, func(source string) ([]byte, fs.FileMode, error) {
		data, mode, err := handle.ReadWithMode(source)
		if err != nil {
			return nil, 0, fmt.Errorf("read backup source %s: %w", source, err)
		}
		return data, mode, nil
	}, func(destination string, data []byte, mode fs.FileMode) error {
		if err := handle.Publish(destination, data, mode); err != nil {
			return fmt.Errorf("publish backup %s from %s: %w", destination, rel, err)
		}
		return nil
	})
}

// syncFilesystem is sync's cohesive, root-confined filesystem dependency.
type syncFilesystem interface {
	MkdirAll(string, fs.FileMode) error
	Chmod(string, fs.FileMode) error
	Publish(string, []byte, fs.FileMode) error
	Replace(string, []byte, fs.FileMode) error
	ReplaceExpected(string, fs.FileInfo, []byte, fs.FileMode) error
	Remove(string) error
	RemoveExpected(string, fs.FileInfo) error
	Read(string) ([]byte, error)
	ReadWithMode(string) ([]byte, fs.FileMode, error)
	LinkInfo(string) (fs.FileInfo, error)
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
	tracked, err := filesystem.Open(p.state.Roots().Tracked)
	if err != nil {
		return syncFilesystems{}, nil, err
	}
	closeAll := func() { _ = tracked.Close() }
	if p.state.Roots().Resident == p.state.Roots().Tracked {
		return syncFilesystems{tracked: tracked, resident: tracked}, closeAll, nil
	}
	residentHandle, err := filesystem.Open(p.state.Roots().Resident)
	if err != nil {
		closeAll()
		return syncFilesystems{}, nil, err
	}
	return syncFilesystems{tracked: tracked, resident: residentHandle}, func() {
		_ = residentHandle.Close()
		_ = tracked.Close()
	}, nil
}

func (p *Publisher) sync(seed *InitAuthority, op *outputplan.Plan) (Result, error) {
	filesystems, closeAll, err := openSyncFilesystems(p.inputs)
	if err != nil {
		return Result{}, err
	}
	defer closeAll()
	backups, changes, pruned, effects, err := syncReportWithPlan(p.inputs, seed, filesystems, op)
	result := newResult(backups, changes, pruned, effects)
	return result, partialOrError(result, err)
}

func syncReportWithPlan(p renderInputs, seed *InitAuthority, filesystems syncFilesystems, op *outputplan.Plan) (backups []Backup, changes []Change, pruned []string, effects []Effect, err error) {
	defer func() {
		slices.Sort(pruned)
		slices.SortFunc(changes, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	}()
	// Refuse before rendering or writing anything: a corrupt lock must never
	// produce a backup, skip a prune, or be overwritten (ADR-0076 Decision 2).
	lockPath := path.Join(config.DirName, "awf.lock")
	lockIdentity, identityErr := filesystems.tracked.LinkInfo(lockPath)
	if identityErr != nil && !errors.Is(identityErr, fs.ErrNotExist) {
		return nil, nil, nil, nil, fmt.Errorf("inspect .awf/awf.lock identity: %w", identityErr)
	}
	lockBytes, lockErr := filesystems.tracked.Read(lockPath)
	var old *manifest.Lock
	found := lockErr == nil
	if found {
		old, err = manifest.ParseLive(lockBytes, migrate.Current(), migrate.Current())
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
		}
	} else if !errors.Is(lockErr, fs.ErrNotExist) {
		return nil, nil, nil, nil, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", lockErr)
	}
	if seed != nil {
		if found {
			return nil, nil, nil, nil, errors.New("first-adoption initialization requires an absent lock")
		}
	} else {
		if !found {
			return nil, nil, nil, nil, errors.New("pre-tracking authority: ordinary sync requires a supported permanent lock; restore .awf/awf.lock from version control")
		}
		state, stateErr := old.AuthorityState()
		if stateErr != nil || state != manifest.AuthorityPermanent {
			return nil, nil, nil, nil, errors.New("pre-tracking authority: ordinary sync requires a supported permanent lock; restore .awf/awf.lock from version control")
		}
	}
	preservedResidents, err := resident.InspectRoots(p.state.Roots().Resident)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	files := op.Outputs()
	for _, f := range files {
		if f.Policy().ValidateFrontmatter {
			if err := validatePublicationArtifact([]byte(f.Content()), AgentDialect(f.Encoder())); err != nil { // coverage-ignore: rendered catalog skill/agent syntax is template-fixed and cannot be invalid at sync time
				return nil, nil, nil, nil, fmt.Errorf("invalid agent artifact in %s: %w", f.Path(), err)
			}
		}
	}
	// Prior lock, read before any write (top of this func): membership decides
	// foreign (back up) vs awf-managed (overwrite silently), and drives pruning.
	prior := map[string]bool{}
	if old != nil {
		for path := range old.Files {
			prior[path] = true
		}
	}

	lock := &manifest.Lock{AWFVersion: p.version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if old != nil {
		lock.InitializedWithVersion = old.InitializedWithVersion
	} else {
		lock.InitializedWithVersion = seed.InitializedWithVersion
	}
	want := map[string]bool{}
	for _, f := range files {
		filesystem, outputPath := filesystems.output(f.Path())
		dir := path.Dir(outputPath)
		if dir != "." {
			missing := missingAncestors(filesystem, dir)
			if err := filesystem.MkdirAll(dir, 0o755); err != nil {
				effects = appendCreatedDirectories(effects, filesystem, missing)
				return backups, changes, pruned, effects, err
			}
			effects = appendCreatedDirectories(effects, filesystem, missing)
		}
		if strings.HasPrefix(f.Path(), config.DirName+"/") && strings.HasSuffix(f.Path(), "/.gitignore") && resident.IsResidentPath(strings.TrimSuffix(f.Path(), "/.gitignore")) {
			beforeMode, observeErr := filesystem.LinkInfo(dir)
			if observeErr != nil {
				return backups, changes, pruned, effects, fmt.Errorf("inspect resident directory mode %s: %w", dir, observeErr)
			}
			if err := filesystem.Chmod(dir, 0o700); err != nil {
				return backups, changes, pruned, effects, err
			}
			if beforeMode != nil && beforeMode.Mode().Perm() != 0o700 {
				effects = append(effects, Effect{Kind: "mode-corrected", Path: dir, Recovery: "rerun awf render"})
			}
		}
		info, infoErr := filesystem.LinkInfo(outputPath)
		if infoErr != nil && !errors.Is(infoErr, fs.ErrNotExist) {
			return backups, changes, pruned, effects, infoErr
		}
		if !prior[f.Path()] && infoErr == nil {
			// touches-state: rendering/sync-and-drift:sync-backs-up-foreign - foreign-file backup on sync; proof in publication_sync_test.go
			bak, err := backupFileConfined(outputPath, filesystem)
			if err != nil {
				if committedPath, residuePath, committed := committedPublication(err); committed {
					bak = committedPath
					backups = append(backups, Backup{Path: f.Path(), Bak: bak, Index: f.RegenChecked()})
					effects = append(effects, Effect{Kind: "backup-created", Path: bak, Recovery: "retain while recovering the render"})
					if residuePath != "" {
						effects = append(effects, Effect{Kind: "publication-residue", Path: residuePath, Recovery: "remove this temporary residue, then rerun awf render"})
					}
				}
				return backups, changes, pruned, effects, fmt.Errorf("back up %s: %w", f.Path(), err)
			}
			backups = append(backups, Backup{Path: f.Path(), Bak: bak, Index: f.RegenChecked()})
			effects = append(effects, Effect{Kind: "backup-created", Path: bak, Recovery: "retain until the render completes, then review or remove"})
		}
		perm := fs.FileMode(0o644)
		if strings.HasPrefix(f.Content(), "#!") {
			perm = 0o755
		}
		modeChanged := false
		changedOutput := infoErr != nil
		if infoErr == nil && info.Mode()&fs.ModeSymlink == 0 {
			before, mode, readErr := filesystem.ReadWithMode(outputPath)
			if readErr != nil {
				return backups, changes, pruned, effects, readErr
			}
			modeChanged = mode != perm
			changedOutput = string(before) != f.Content()
		} else if infoErr == nil {
			// A managed final symlink is replaced as its entry without target access.
			changedOutput = true
		}
		recordChange := func() {
			if old == nil {
				return
			}
			oldE, ok := old.Files[f.Path()]
			if !ok {
				changes = append(changes, Change{Path: f.Path(), Cause: "added"})
				return
			}
			tMoved, cMoved := f.TemplateHash() != oldE.TemplateHash, f.ConfigHash() != oldE.ConfigHash
			cause := "internal"
			switch {
			case tMoved && cMoved:
				cause = "template+config"
			case tMoved:
				cause = "template"
			case cMoved:
				cause = "config"
			case f.RegenChecked():
				cause = "regenerated"
			}
			changes = append(changes, Change{Path: f.Path(), Cause: cause})
		}
		effectKind := "output-replaced"
		if info == nil {
			effectKind = "output-created"
		}
		if err := filesystem.ReplaceExpected(outputPath, info, []byte(f.Content()), perm); err != nil {
			effects, _ = appendCommittedOperationEffects(effects, err, Effect{Kind: effectKind, Path: f.Path(), Recovery: "rerun awf render to complete authority publication"})
			return backups, changes, pruned, effects, err
		}
		effects = append(effects, Effect{Kind: effectKind, Path: f.Path(), Recovery: "rerun awf render to complete authority publication"})
		// Replacement commits bytes and final mode together, so a change becomes
		// reportable only after the namespace commit succeeds.
		if changedOutput || modeChanged {
			recordChange()
		}
		lock.Files[f.Path()] = manifest.Entry{
			TemplateID: f.TemplateID(), TemplateHash: f.TemplateHash(),
			ConfigHash: f.ConfigHash(), OutputHash: manifest.Hash([]byte(f.Content())),
			RegenChecked: f.RegenChecked(),
		}
		want[f.Path()] = true
	}
	// Prune files from the previous lock that are no longer produced, then remove
	// every directory left empty - walking all ancestors deepest-first, not just the
	// immediate parent, so dropping a target clears its whole tree (inv:
	// target-prune-ancestors; reuses Uninstall's idiom).
	if old != nil {
		type cleanupDir struct {
			filesystem syncFilesystem
			path       string
			resident   bool
		}
		dirs := map[string]cleanupDir{}
		retiredPaths := slices.Sorted(maps.Keys(old.Files))
		for _, path := range retiredPaths {
			entry := old.Files[path]
			if want[path] || resident.PreserveRemoval(path, preservedResidents) {
				continue
			}
			filesystem, outputPath := filesystems.output(path)
			if entry.TemplateID == localDocTID {
				if info, existsErr := filesystem.LinkInfo(outputPath); existsErr != nil && !errors.Is(existsErr, fs.ErrNotExist) {
					return backups, changes, pruned, effects, fmt.Errorf("inspect pruned local document %s: %w", path, existsErr)
				} else if existsErr == nil {
					if info.Mode()&fs.ModeSymlink != 0 {
						return backups, changes, pruned, effects, fmt.Errorf("unsafe pruned local document %s", path)
					}
					bak, bakErr := backupFileConfined(outputPath, filesystem)
					if bakErr != nil {
						if committedPath, residuePath, committed := committedPublication(bakErr); committed {
							bak = committedPath
							backups = append(backups, Backup{Path: path, Bak: bak})
							effects = append(effects, Effect{Kind: "backup-created", Path: bak, Recovery: "retain as recovery for the local document"})
							if residuePath != "" {
								effects = append(effects, Effect{Kind: "publication-residue", Path: residuePath, Recovery: "remove this temporary residue, then rerun awf render"})
							}
						}
						return backups, changes, pruned, effects, fmt.Errorf("back up pruned local document %s: %w", path, bakErr)
					}
					backups = append(backups, Backup{Path: path, Bak: bak})
					effects = append(effects, Effect{Kind: "backup-created", Path: bak, Recovery: "retain as recovery for the removed local document"})
				}
			}
			// Report only an actual removal - a path whose file is already gone
			// must not be claimed pruned. Any other failure preserves the old lock
			// so the managed path remains visible and the operation can be retried.
			removeIdentity, observeErr := filesystem.LinkInfo(outputPath)
			if observeErr != nil && !errors.Is(observeErr, fs.ErrNotExist) {
				return backups, changes, pruned, effects, fmt.Errorf("inspect retired output %s: %w", path, observeErr)
			}
			var err error
			if removeIdentity == nil {
				err = fs.ErrNotExist
			} else {
				err = filesystem.RemoveExpected(outputPath, removeIdentity)
			}
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				var committed bool
				effects, committed = appendCommittedOperationEffects(effects, err, Effect{Kind: "output-removed", Path: path, Recovery: "rerun awf render to complete pruning and lock publication"})
				if committed {
					pruned = append(pruned, path)
				}
				return backups, changes, pruned, effects, fmt.Errorf("remove retired output %s: %w", path, err)
			}
			if err == nil {
				pruned = append(pruned, path)
				effects = append(effects, Effect{Kind: "output-removed", Path: path, Recovery: "rerun awf render to complete pruning and lock publication"})
			}
			for d := filepath.ToSlash(filepath.Dir(filepath.FromSlash(outputPath))); d != "."; d = filepath.ToSlash(filepath.Dir(filepath.FromSlash(d))) {
				key := fmt.Sprintf("%t:%s", resident.IsResidentPath(path), d)
				dirs[key] = cleanupDir{filesystem: filesystem, path: d, resident: resident.IsResidentPath(path)}
			}
		}
		dirList := slices.Collect(maps.Values(dirs))
		slices.SortFunc(dirList, func(a, b cleanupDir) int {
			if aDepth, bDepth := strings.Count(a.path, "/"), strings.Count(b.path, "/"); aDepth != bDepth {
				return bDepth - aDepth
			}
			if a.resident != b.resident {
				if a.resident {
					return 1
				}
				return -1
			}
			return strings.Compare(a.path, b.path)
		})
		for _, d := range dirList {
			info, infoErr := d.filesystem.LinkInfo(d.path)
			if infoErr != nil {
				if errors.Is(infoErr, fs.ErrNotExist) {
					continue
				}
				return backups, changes, pruned, effects, fmt.Errorf("inspect empty directory %s: %w", d.path, infoErr)
			}
			if !info.IsDir() {
				continue
			}
			if err := d.filesystem.RemoveExpected(d.path, info); err != nil {
				if errors.Is(err, filesystem.ErrDirectoryNotEmpty) {
					continue
				}
				effects, _ = appendCommittedOperationEffects(effects, err, Effect{Kind: "empty-directory-removed", Path: d.path, Recovery: "rerun awf render"})
				return backups, changes, pruned, effects, fmt.Errorf("remove empty directory %s: %w", d.path, err)
			}
			effects = append(effects, Effect{Kind: "empty-directory-removed", Path: d.path, Recovery: "rerun awf render"})
		}
	}
	lockBytes, err = lock.Marshal()
	if err != nil { // coverage-ignore: sync constructs only authority-valid lock fields, so marshal failure requires a future representation change
		return backups, changes, pruned, effects, err // coverage-ignore: sync constructs only authority-valid lock fields, so marshal failure requires a future representation change
	}
	if err := filesystems.tracked.ReplaceExpected(lockPath, lockIdentity, lockBytes, 0o644); err != nil {
		effects, _ = appendCommittedOperationEffects(effects, err, Effect{Kind: "lock-replaced", Path: lockPath, Recovery: "rerun awf render to verify and complete publication"})
		return backups, changes, pruned, effects, err
	}
	effects = append(effects, Effect{Kind: "lock-replaced", Path: lockPath, Recovery: "none; publication is complete"})
	return backups, changes, pruned, effects, nil
}

func missingAncestors(filesystem syncFilesystem, dir string) []string {
	var missing []string
	for current := dir; current != "."; current = path.Dir(current) {
		if _, err := filesystem.LinkInfo(current); errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, current)
		}
	}
	slices.Reverse(missing)
	return missing
}

func appendCreatedDirectories(effects []Effect, filesystem syncFilesystem, paths []string) []Effect {
	for _, created := range paths {
		if info, err := filesystem.LinkInfo(created); err == nil && info.IsDir() {
			effects = append(effects, Effect{Kind: "directory-created", Path: created, Recovery: "rerun awf render; remove only if still empty after recovery"})
		}
	}
	return effects
}

// SyncMutation maps the completed sync outcome into presentation-owned syntax.
// Backup ownership and output provenance stay semantic facts of this package.
func syncMutation(backups []Backup, changes []Change, pruned []string) (presentation.Mutation, error) {
	groups := make([]presentation.MutationChange, 0, 3)
	notes := []presentation.Value{}
	for _, backup := range backups {
		if backup.Index {
			value, err := presentation.Literal("awf now generates " + backup.Path + "; retire any external generator for it")
			if err != nil {
				return presentation.Mutation{}, err
			}
			notes = append(notes, value)
		}
	}
	if len(backups) > 0 {
		values := make([]presentation.Value, 0, len(backups))
		for _, backup := range backups {
			value, err := presentation.Literal(fmt.Sprintf("%s to %s", backup.Path, backup.Bak))
			if err != nil {
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		groups = append(groups, presentation.MutationChange{Label: "backups", Values: values})
	}
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
	if err != nil { // coverage-ignore: fixed nonempty completion action always validates as prose
		return presentation.Mutation{}, err
	}
	return presentation.Mutation{Status: "completed", Changes: groups, Notes: notes, NextActions: []presentation.Value{next}}, nil
}

// InitCollisions reports unmanaged planned paths that already exist at the tracked root.
func (p *Publisher) InitCollisions() ([]string, error) {
	plan, err := p.Plan()
	if err != nil {
		return nil, err
	}
	return resident.CollisionsAt(p.inputs.root(), plan.Paths())
}

// InitCollisions reports unmanaged paths in this exact prepared universe that
// already exist at the tracked root.
func (p Preparation) InitCollisions() ([]string, error) {
	if p.publisher == nil {
		return nil, errors.New("publisher: unbound preparation")
	}
	return resident.CollisionsAt(p.publisher.inputs.root(), p.plan.Paths())
}

// IsLocalDocTemplate is the bounded recognition policy outer composition passes to uninstall.
func IsLocalDocTemplate(templateID string) bool { return templateID == localDocTID }
