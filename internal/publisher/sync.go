// Package publisher owns application-level output publication.
package publisher

import (
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
// it - plus a pruned co-owned runner before removing it (ADR-0156 item 9) -
// and returning those backups (ADR-0035) plus the per-file provenance of
// output that changed against the prior lock and the lock-relative paths of the
// files its prune actually removed (both path-sorted; a file whose output is
// byte-identical, and first-adoption initialization with no prior lock reports
// no change - a routine re-sync stays silent).
func (p *Publisher) Sync() (Result, error) {
	prepared, err := p.Prepare()
	if err != nil {
		return Result{}, err
	}
	return prepared.Sync()
}

// Sync publishes the exact operation universe captured by this preparation.
func (p Preparation) Sync() (Result, error) {
	if p.publisher == nil {
		return Result{}, errors.New("publisher: unbound preparation")
	}
	if err := configcheck.ValidateCommandWiring(p.publisher.inputs.cfg); err != nil {
		return Result{}, err
	}
	return p.publisher.sync(nil, &p.plan)
}

// Initialize publishes a first adoption from the exact operation universe
// captured by this preparation.
func (p Preparation) Initialize(seed InitAuthority) (Result, error) {
	if p.publisher == nil {
		return Result{}, errors.New("publisher: unbound preparation")
	}
	return p.publisher.sync(&seed, &p.plan)
}

// InitAuthority is the explicit provenance supplied only by first adoption.
type InitAuthority struct{ InitializedWithVersion string }

// Result records only mutations committed before the terminal outcome.
type Result struct {
	backups []Backup
	changes []Change
	pruned  []string
}

func newResult(backups []Backup, changes []Change, pruned []string) Result {
	return Result{backups: slices.Clone(backups), changes: slices.Clone(changes), pruned: slices.Clone(pruned)}
}
func (r Result) Backups() []Backup { return slices.Clone(r.backups) }
func (r Result) Changes() []Change { return slices.Clone(r.changes) }
func (r Result) Pruned() []string  { return slices.Clone(r.pruned) }

// Mutation maps this semantic result to the central presentation grammar.
func (r Result) Mutation() (presentation.Mutation, error) {
	return syncMutation(r.Backups(), r.Changes(), r.Pruned())
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
	Remove(string) error
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
	backups, changes, pruned, err := syncReportWithPlan(p.inputs, seed, filesystems, op)
	return newResult(backups, changes, pruned), err
}

func syncReportWithPlan(p renderInputs, seed *InitAuthority, filesystems syncFilesystems, op *outputplan.Plan) (backups []Backup, changes []Change, pruned []string, err error) {
	defer func() {
		slices.Sort(pruned)
		slices.SortFunc(changes, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	}()
	// Refuse before rendering or writing anything: a corrupt lock must never
	// produce a backup, skip a prune, or be overwritten (ADR-0076 Decision 2).
	lockPath := path.Join(config.DirName, "awf.lock")
	lockBytes, lockErr := filesystems.tracked.Read(lockPath)
	var old *manifest.Lock
	found := lockErr == nil
	if found {
		old, err = manifest.Parse(lockBytes)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
		}
	} else if !errors.Is(lockErr, fs.ErrNotExist) {
		return nil, nil, nil, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", lockErr)
	}
	if seed != nil {
		if found {
			return nil, nil, nil, errors.New("first-adoption initialization requires an absent lock")
		}
	} else {
		if !found {
			return nil, nil, nil, errors.New("pre-tracking authority: ordinary sync cannot create lock authority; use the bridge release to attest")
		}
		state, stateErr := old.AuthorityState()
		if stateErr != nil || state != manifest.AuthorityPermanent {
			return nil, nil, nil, errors.New("pre-tracking authority: ordinary sync requires a permanent lock; use the bridge release to attest")
		}
	}
	preservedResidents, err := resident.InspectRoots(p.state.Roots().Resident)
	if err != nil {
		return nil, nil, nil, err
	}
	files := op.Outputs()
	for _, f := range files {
		if f.Policy().ValidateFrontmatter {
			if err := validatePublicationArtifact([]byte(f.Content()), AgentDialect(f.Encoder())); err != nil { // coverage-ignore: rendered catalog skill/agent syntax is template-fixed and cannot be invalid at sync time
				return nil, nil, nil, fmt.Errorf("invalid agent artifact in %s: %w", f.Path(), err)
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
			if err := filesystem.MkdirAll(dir, 0o755); err != nil {
				return backups, changes, pruned, err
			}
		}
		if strings.HasPrefix(f.Path(), config.DirName+"/") && strings.HasSuffix(f.Path(), "/.gitignore") && resident.IsResidentPath(strings.TrimSuffix(f.Path(), "/.gitignore")) {
			if err := filesystem.Chmod(dir, 0o700); err != nil {
				return backups, changes, pruned, err
			}
		}
		info, infoErr := filesystem.LinkInfo(outputPath)
		if infoErr != nil && !errors.Is(infoErr, fs.ErrNotExist) {
			return backups, changes, pruned, infoErr
		}
		if !prior[f.Path()] && infoErr == nil {
			// touches-state: rendering/sync-and-drift:sync-backs-up-foreign - foreign-file backup on sync; proof in publication_sync_test.go
			bak, err := backupFileConfined(outputPath, filesystem)
			if err != nil {
				return backups, changes, pruned, fmt.Errorf("back up %s: %w", f.Path(), err)
			}
			backups = append(backups, Backup{Path: f.Path(), Bak: bak, Index: f.RegenChecked()})
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
				return backups, changes, pruned, readErr
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
		if err := filesystem.Replace(outputPath, []byte(f.Content()), perm); err != nil {
			return backups, changes, pruned, err
		}
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
		}
		dirs := map[string]cleanupDir{}
		for path, entry := range old.Files {
			if want[path] || resident.PreserveRemoval(path, preservedResidents) {
				continue
			}
			// A non-local entry (corrupted or malicious lock) would delete outside
			// the root and send the ancestor walk below it, never reaching p.root().
			if !filepath.IsLocal(filepath.FromSlash(path)) {
				continue
			}
			filesystem, outputPath := filesystems.output(path)
			// The outgoing co-owned runner is the one pruned output an adopter
			// hand-authored inside (its in-place verb bodies), so it is backed
			// up before removal for the one-time hand-port instead of vanishing
			// into git history (ADR-0156 item 9). A backup failure aborts the
			// prune - never a silent fall-through to deletion.
			switch entry.TemplateID {
			case localDocTID:
				if info, existsErr := filesystem.LinkInfo(outputPath); existsErr != nil && !errors.Is(existsErr, fs.ErrNotExist) {
					return backups, changes, pruned, fmt.Errorf("inspect pruned local document %s: %w", path, existsErr)
				} else if existsErr == nil {
					if info.Mode()&fs.ModeSymlink != 0 {
						return backups, changes, pruned, fmt.Errorf("unsafe pruned local document %s", path)
					}
					bak, bakErr := backupFileConfined(outputPath, filesystem)
					if bakErr != nil {
						return backups, changes, pruned, fmt.Errorf("back up pruned local document %s: %w", path, bakErr)
					}
					backups = append(backups, Backup{Path: path, Bak: bak})
				}
			case coOwnedRunnerTID:
				if info, existsErr := filesystem.LinkInfo(outputPath); existsErr == nil && info.Mode()&fs.ModeSymlink == 0 {
					bak, bakErr := backupFileConfined(outputPath, filesystem)
					if bakErr != nil {
						return backups, changes, pruned, fmt.Errorf("back up pruned runner %s: %w", path, bakErr)
					}
					backups = append(backups, Backup{Path: path, Bak: bak})
				}
			}
			// Report only an actual removal - a path whose file is already gone
			// must not be claimed pruned. Any other failure preserves the old lock
			// so the managed path remains visible and the operation can be retried.
			err := filesystem.Remove(outputPath)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return backups, changes, pruned, fmt.Errorf("remove retired output %s: %w", path, err)
			}
			if err == nil {
				pruned = append(pruned, path)
			}
			for d := filepath.ToSlash(filepath.Dir(filepath.FromSlash(outputPath))); d != "."; d = filepath.ToSlash(filepath.Dir(filepath.FromSlash(d))) {
				key := fmt.Sprintf("%t:%s", resident.IsResidentPath(path), d)
				dirs[key] = cleanupDir{filesystem: filesystem, path: d}
			}
		}
		dirList := slices.Collect(maps.Values(dirs))
		slices.SortFunc(dirList, func(a, b cleanupDir) int { return len(b.path) - len(a.path) })
		for _, d := range dirList {
			_ = d.filesystem.Remove(d.path) // removes only if now empty
		}
	}
	lockBytes, err = lock.Marshal()
	if err != nil { // coverage-ignore: sync constructs only authority-valid lock fields, so marshal failure requires a future representation change
		return backups, changes, pruned, err // coverage-ignore: sync constructs only authority-valid lock fields, so marshal failure requires a future representation change
	}
	return backups, changes, pruned, filesystems.tracked.Replace(lockPath, lockBytes, 0o644)
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
