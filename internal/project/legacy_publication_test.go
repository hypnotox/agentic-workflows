package project

import (
	"errors"
	"fmt"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

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

// SyncReport renders and writes the project, additionally backing up any
// foreign file (on disk but absent from the start-of-sync lock) before overwriting
// it - plus a pruned co-owned runner before removing it (ADR-0156 item 9) -
// and returning those backups (ADR-0035) plus the per-file provenance of
// output that changed against the prior lock and the lock-relative paths of the
// files its prune actually removed (both path-sorted; a file whose output is
// byte-identical, and first-adoption initialization with no prior lock reports
// no change - a routine re-sync stays silent).
func syncReportOperation(p renderInputs, op *OutputPlan) ([]Backup, []Change, []string, error) {
	// Refuse an unresolvable hook-command wiring before rendering anything
	// (ADR-0156 Decision 5); first-adoption InitializeReport stays exempt.
	if err := validateCommandWiring(p.cfg); err != nil {
		return nil, nil, nil, err
	}
	return syncReport(p, nil, op)
}

// InitAuthority is the explicit provenance supplied only by first adoption.
type InitAuthority struct {
	InitializedWithVersion string
}

// InitializeReport renders a first adoption while sealing its existing ADR
// identities. It has the same reporting contract as SyncReport.
func initializeReport(p renderInputs, seed InitAuthority, op *OutputPlan) ([]Backup, []Change, []string, error) {
	return syncReport(p, &seed, op)
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
	tracked, err := filesystem.Open(p.residentRoots().Tracked)
	if err != nil {
		return syncFilesystems{}, nil, err
	}
	closeAll := func() { _ = tracked.Close() }
	if p.residentRoots().Resident == p.residentRoots().Tracked {
		return syncFilesystems{tracked: tracked, resident: tracked}, closeAll, nil
	}
	residentHandle, err := filesystem.Open(p.residentRoots().Resident)
	if err != nil {
		closeAll()
		return syncFilesystems{}, nil, err
	}
	return syncFilesystems{tracked: tracked, resident: residentHandle}, func() {
		_ = residentHandle.Close()
		_ = tracked.Close()
	}, nil
}

func syncReport(p renderInputs, seed *InitAuthority, op *OutputPlan) (backups []Backup, changes []Change, pruned []string, err error) {
	filesystems, closeAll, err := openSyncFilesystems(p)
	if err != nil {
		return nil, nil, nil, err
	}
	defer closeAll()
	return syncReportWithPlan(p, seed, filesystems, op)
}

func syncReportWithPlan(p renderInputs, seed *InitAuthority, filesystems syncFilesystems, op *OutputPlan) (backups []Backup, changes []Change, pruned []string, err error) {
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
	preservedResidents, err := resident.InspectRoots(p.residentRoots().Resident)
	if err != nil {
		return nil, nil, nil, err
	}
	files := planWriteFiles(op)
	for _, f := range files {
		if f.Policy.ValidateFrontmatter {
			if err := validateArtifact([]byte(f.Content), f.Encoder); err != nil { // coverage-ignore: rendered catalog skill/agent syntax is template-fixed and cannot be invalid at sync time
				return nil, nil, nil, fmt.Errorf("invalid agent artifact in %s: %w", f.Path, err)
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

	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if old != nil {
		lock.InitializedWithVersion = old.InitializedWithVersion
	} else {
		lock.InitializedWithVersion = seed.InitializedWithVersion
	}
	want := map[string]bool{}
	for _, f := range files {
		filesystem, outputPath := filesystems.output(f.Path)
		dir := path.Dir(outputPath)
		if dir != "." {
			if err := filesystem.MkdirAll(dir, 0o755); err != nil {
				return backups, changes, pruned, err
			}
		}
		if strings.HasPrefix(f.Path, config.DirName+"/") && strings.HasSuffix(f.Path, "/.gitignore") && resident.IsResidentPath(strings.TrimSuffix(f.Path, "/.gitignore")) {
			if err := filesystem.Chmod(dir, 0o700); err != nil {
				return backups, changes, pruned, err
			}
		}
		info, infoErr := filesystem.LinkInfo(outputPath)
		if infoErr != nil && !errors.Is(infoErr, fs.ErrNotExist) {
			return backups, changes, pruned, infoErr
		}
		if !prior[f.Path] && infoErr == nil {
			// touches-state: rendering/sync-and-drift:sync-backs-up-foreign - foreign-file backup on sync; proof in project_test.go
			bak, err := backupFileConfined(outputPath, filesystem)
			if err != nil {
				return backups, changes, pruned, fmt.Errorf("back up %s: %w", f.Path, err)
			}
			backups = append(backups, Backup{Path: f.Path, Bak: bak, Index: f.RegenChecked})
		}
		perm := fs.FileMode(0o644)
		if strings.HasPrefix(f.Content, "#!") {
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
			changedOutput = string(before) != f.Content
		} else if infoErr == nil {
			// A managed final symlink is replaced as its entry without target access.
			changedOutput = true
		}
		recordChange := func() {
			if old == nil {
				return
			}
			oldE, ok := old.Files[f.Path]
			if !ok {
				changes = append(changes, Change{Path: f.Path, Cause: "added"})
				return
			}
			tMoved, cMoved := f.TemplateHash != oldE.TemplateHash, f.ConfigHash != oldE.ConfigHash
			cause := "internal"
			switch {
			case tMoved && cMoved:
				cause = "template+config"
			case tMoved:
				cause = "template"
			case cMoved:
				cause = "config"
			case f.RegenChecked:
				cause = "regenerated"
			}
			changes = append(changes, Change{Path: f.Path, Cause: cause})
		}
		if err := filesystem.Replace(outputPath, []byte(f.Content), perm); err != nil {
			return backups, changes, pruned, err
		}
		// Replacement commits bytes and final mode together, so a change becomes
		// reportable only after the namespace commit succeeds.
		if changedOutput || modeChanged {
			recordChange()
		}
		lock.Files[f.Path] = manifest.Entry{
			TemplateID: f.TemplateID, TemplateHash: f.TemplateHash,
			ConfigHash: f.ConfigHash, OutputHash: manifest.Hash([]byte(f.Content)),
			RegenChecked: f.RegenChecked,
		}
		want[f.Path] = true
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

// IsLocalDocTemplate is the bounded recognition policy outer composition passes
// to uninstall without leaking the template identity into resident.
func IsLocalDocTemplate(templateID string) bool { return templateID == localDocTID }
