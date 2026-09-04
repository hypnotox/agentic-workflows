// Package upgrade plans supported live-schema migrations and applies their
// output through a root-confined recoverable journal. The journal publishes the
// replacement lock last and preserves rollback, quarantine, postcommit cleanup,
// and recovery without interpreting historical authority.
package upgrade

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

// JournalVersion is the only accepted current-state upgrade journal schema.
const JournalVersion = 1

// Journal phases. A precommit phase (prepared, applying, rolling-back) has not
// necessarily written the final lock; lock-committed marks the lock as the
// committed authority. Every valid phase blocks all project commands except
// `awf upgrade --recover`.
const (
	phasePrepared      = "prepared"
	phaseApplying      = "applying"
	phaseRollingBack   = "rolling-back"
	phaseLockCommitted = "lock-committed"
)

// gitRestorationGuidance is the deterministic escape hatch printed when a
// journal is unusable: restore the tree from Git before retrying from a
// supported live source.
const gitRestorationGuidance = "restore the working tree from Git (git restore + git clean) and retry from a supported live source"

// Operation kinds. An empty kind is a file replacement, so every journal written
// before resident quarantine existed keeps its exact meaning. A resident-tree
// operation moves a whole directory aside instead of imaging its bytes: a
// resident tree holds unbounded ephemeral descendants, so recording it as file
// images would be both enormous and lossy, and deleting it outright would leave
// nothing to roll back to.
const (
	KindFile           = ""
	KindResidentTree   = "resident-tree"
	KindEmptyDirectory = "empty-directory"
)

// QuarantineRel is the repo-relative root every quarantined resident tree moves
// under. It sits inside the awf directory so one tracked-authority restore
// reaches it, and it is never a resident root itself.
func QuarantineRel() string { return config.DirName + "/.upgrade-quarantine" }

// Image is one file's exact recorded state: present with an octal permission
// mode and content, or absent (present:false, mode 0, empty content).
type Image struct {
	Present bool   `json:"present"`
	Mode    uint32 `json:"mode"`
	Content []byte `json:"content"`
}

// Operation records one path's prior and replacement images. The final journal
// operation is always the lock replacement. A file operation temporarily names
// an indeterminate exchange residue when committed cleanup could not restore a
// mismatch. A resident-tree operation carries no images and instead names the
// quarantine path its tree is renamed to; the rename is the mutation, so it is
// reversible before the lock commits and only needs deleting after.
type Operation struct {
	Path            string   `json:"path"`
	Kind            string   `json:"kind,omitempty"`
	Prior           Image    `json:"prior"`
	Replacement     Image    `json:"replacement"`
	ExpectedEntries []string `json:"expectedEntries,omitempty"`
	Quarantine      string   `json:"quarantine,omitempty"`
	Residue         string   `json:"residue,omitempty"`
	PossibleResidue bool     `json:"possibleResidue,omitempty"`
}

// residentTree reports whether op quarantines a tree rather than imaging a file.
func (o Operation) residentTree() bool   { return o.Kind == KindResidentTree }
func (o Operation) emptyDirectory() bool { return o.Kind == KindEmptyDirectory }

// Evidence is one ordered, terminally proven journal fact. It is collected by
// the transaction owner and rendered only at the command boundary.
type Evidence struct {
	Action string
	Path   string
}

// Outcome is the ordered terminal evidence from one journal operation.
type Outcome struct {
	// Evidence is the ordered history of proven transaction actions. It may
	// include restored actions that are no longer true when the call returns.
	Evidence []Evidence
	// Changed names only axes still changed when the call returns. Failure
	// diagnostics use this set, never historical Evidence.
	Changed []Evidence
}

// Journal is the durable transaction record. Version is always 1; Operations
// are unique, sorted, and end with the lock operation; FinalLockSHA256 seals
// that operation's content while its recorded image also binds the exact mode.
type Journal struct {
	Version         int         `json:"version"`
	Phase           string      `json:"phase"`
	FinalLockSHA256 string      `json:"finalLockSHA256"`
	Operations      []Operation `json:"operations"`
}

// JournalRel is the fixed repo-relative current-state upgrade journal path.
const JournalRel = config.DirName + "/current-state-upgrade.journal"

// LockRel is the repo-relative lock path every journal ends on.
func LockRel() string { return config.DirName + "/awf.lock" }

// JournalPresent reports whether a journal file exists under root. A fault is
// returned rather than folded into absence: answering "no journal" from a read
// that never completed would let the command-state guard permit the commands an
// unrecovered upgrade must block.
func JournalPresent(root string) (bool, error) {
	var present bool
	err := withJournalFilesystem(root, func(files *filesystem.Handle) error {
		expected, err := files.ExpectedIdentity(JournalRel)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer expected.Release() //nolint:errcheck // read-only journal inspection owns no mutation
		if !expected.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file: %w", JournalRel, filesystem.ErrIdentityChanged)
		}
		present = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("inspect current-state upgrade journal: %w", err)
	}
	return present, nil
}

func withJournalFilesystem(root string, run func(*filesystem.Handle) error) error {
	files, err := filesystem.Open(root)
	if err != nil {
		return err
	}
	return errors.Join(run(files), files.Close())
}

type directoryExpectation struct {
	apply    *filesystem.ExpectedIdentity
	rollback *filesystem.ExpectedIdentity
}

type journalFilesystemState struct {
	files                 *filesystem.Handle
	directoryExpectations map[string]directoryExpectation
}

func (s *journalFilesystemState) with(root string, run func(*filesystem.Handle) error) error {
	if s.files != nil {
		return run(s.files)
	}
	return withJournalFilesystem(root, run)
}

// withBoundJournalOperation holds one confined root open for a complete journal
// transaction or recovery. Production callbacks share this state; injected
// callbacks remain local to the operation value used by a test.
func withBoundJournalOperation(root string, operation journalOperation, run func(journalOperation) error) error {
	files, err := filesystem.Open(root)
	if err != nil {
		return err
	}
	if operation.state == nil {
		operation.state = &journalFilesystemState{}
	}
	operation.state.files = files
	operation.state.directoryExpectations = make(map[string]directoryExpectation)
	runErr := run(operation)
	var releaseErr error
	for _, expected := range operation.state.directoryExpectations {
		releaseErr = errors.Join(releaseErr, expected.apply.Release(), expected.rollback.Release())
	}
	operation.state.directoryExpectations = nil
	operation.state.files = nil
	return errors.Join(runErr, releaseErr, files.Close())
}

func (o journalOperation) captureDirectoryExpectation(root, path string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	err := o.state.with(root, func(files *filesystem.Handle) (returnErr error) {
		apply, err := files.ExpectedIdentity(path)
		if err != nil {
			return err
		}
		defer func() {
			if returnErr != nil {
				returnErr = errors.Join(returnErr, apply.Release())
			}
		}()
		if !apply.IsDir() {
			return fmt.Errorf("%s is not a directory: %w", path, filesystem.ErrIdentityChanged)
		}
		entries, err = files.ReadDirExpected(path, apply)
		if err != nil {
			return err
		}
		rollback, err := files.ExpectedIdentity(path)
		if err != nil {
			return err
		}
		current, err := files.LinkInfo(path)
		if err != nil || !apply.SameFile(current) || !rollback.SameFile(current) {
			return errors.Join(fmt.Errorf("%s changed while capturing its directory identity", path), err, rollback.Release(), filesystem.ErrIdentityChanged)
		}
		o.state.directoryExpectations[path] = directoryExpectation{apply: apply, rollback: rollback}
		return nil
	})
	return entries, err
}

// quarantineTree renames a resident tree aside. An absent source is already in
// the desired state, so a restarted run converges instead of failing. An
// existing destination is never overwritten: that would destroy the only copy
// of whatever a previous interrupted run put there.
func quarantineTree(root string, op Operation, operation journalOperation) error {
	if _, err := operation.lstat(root, op.Path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if _, err := operation.lstat(root, op.Quarantine); err == nil {
		return fmt.Errorf("quarantine destination %s already exists; %s", op.Quarantine, gitRestorationGuidance)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(op.Quarantine)))
	if err := operation.mkdirAll(root, parent, 0o755); err != nil {
		return err
	}
	return operation.rename(root, op.Path, op.Quarantine)
}

// restoreTree renames a quarantined tree back to its resident path. It mirrors
// quarantineTree's restart tolerance: an absent quarantine means the tree is
// already home, and an occupied resident path is never overwritten.
func restoreTree(root string, op Operation, operation journalOperation) error {
	if _, err := operation.lstat(root, op.Quarantine); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if _, err := operation.lstat(root, op.Path); err == nil {
		return fmt.Errorf("cannot restore %s because it already exists; %s", op.Path, gitRestorationGuidance)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(op.Path)))
	if err := operation.mkdirAll(root, parent, 0o755); err != nil {
		return err
	}
	return operation.rename(root, op.Quarantine, op.Path)
}

// completeQuarantine discards every quarantined tree once the lock has
// committed, then drops the shared quarantine root. It is idempotent so a
// repeated recovery converges rather than failing on already-deleted bytes.
// journalOperation owns volatile filesystem dependencies for one transaction
// or recovery. Tests compose a faulting value without mutable package state.
type journalOperation struct {
	state           *journalFilesystemState
	removeAll       func(string, string) error
	remove          func(string, string) error
	imageOf         func(string, string) (Image, error)
	applyExpected   func(string, string, Image, Image) error
	write           func(string, Journal) error
	lstat           func(string, string) (os.FileInfo, error)
	mkdirAll        func(string, string, os.FileMode) error
	createDirectory func(string, string, os.FileMode) error
	rename          func(string, string, string) error
	readDir         func(string, string) ([]fs.DirEntry, error)
}

func productionJournalOperation() journalOperation {
	state := &journalFilesystemState{}
	return journalOperation{state: state,
		removeAll: func(root, path string) error {
			return state.with(root, func(files *filesystem.Handle) error { return files.RemoveAll(path) })
		},
		remove: func(root, path string) error {
			return state.with(root, func(files *filesystem.Handle) error { return files.Remove(path) })
		},
		imageOf: func(root, path string) (image Image, err error) {
			err = state.with(root, func(files *filesystem.Handle) error {
				expected, identityErr := files.ExpectedIdentity(path)
				if errors.Is(identityErr, fs.ErrNotExist) {
					image = Image{}
					return nil
				}
				if identityErr != nil {
					return identityErr
				}
				defer expected.Release() //nolint:errcheck // read-only image capture owns no mutation
				if !expected.Mode().IsRegular() {
					return fmt.Errorf("%s is not a regular file: %w", path, filesystem.ErrIdentityChanged)
				}
				content, mode, readErr := files.ReadExpected(path, expected)
				if readErr != nil {
					return readErr
				}
				image = Image{Present: true, Mode: uint32(mode.Perm()), Content: content}
				return nil
			})
			return image, err
		},
		applyExpected: func(root, path string, prior, replacement Image) error {
			return state.with(root, func(files *filesystem.Handle) error {
				if !prior.Present {
					if _, err := files.LinkInfo(path); err == nil {
						return fmt.Errorf("%s: %w", path, filesystem.ErrIdentityChanged)
					} else if !errors.Is(err, fs.ErrNotExist) {
						return err
					}
					if !replacement.Present {
						return nil
					}
					parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
					if err := files.MkdirAll(parent, 0o755); err != nil {
						return err
					}
					return files.ReplaceExpected(path, nil, replacement.Content, os.FileMode(replacement.Mode))
				}
				expected, err := files.ExpectedIdentity(path)
				if err != nil {
					return err
				}
				content, mode, err := files.ReadExpected(path, expected)
				if err != nil {
					_ = expected.Release()
					return err
				}
				current := Image{Present: true, Mode: uint32(mode.Perm()), Content: content}
				if !imagesEqual(current, prior) {
					_ = expected.Release()
					return fmt.Errorf("%s: %w", path, filesystem.ErrIdentityChanged)
				}
				if !replacement.Present {
					return files.RemoveExpectedRegularFile(path, expected, prior.Content, os.FileMode(prior.Mode))
				}
				return files.ReplaceExpectedRegularFile(path, expected, prior.Content, os.FileMode(prior.Mode), replacement.Content, os.FileMode(replacement.Mode))
			})
		},
		write: func(root string, j Journal) error {
			b, err := json.MarshalIndent(j, "", "  ")
			if err != nil {
				return err
			}
			return state.with(root, func(files *filesystem.Handle) error { return files.Replace(JournalRel, append(b, '\n'), 0o644) })
		},
		lstat: func(root, path string) (info os.FileInfo, err error) {
			err = state.with(root, func(files *filesystem.Handle) error {
				info, err = files.LinkInfo(path)
				return err
			})
			return info, err
		},
		mkdirAll: func(root, path string, mode os.FileMode) error {
			return state.with(root, func(files *filesystem.Handle) error { return files.MkdirAll(path, mode) })
		},
		createDirectory: func(root, path string, mode os.FileMode) error {
			return state.with(root, func(files *filesystem.Handle) error {
				created, err := files.CreateDirectory(path, mode)
				if err != nil {
					return err
				}
				return created.Release()
			})
		},
		rename: func(root, oldPath, newPath string) error {
			return state.with(root, func(files *filesystem.Handle) error { return files.Rename(oldPath, newPath) })
		},
		readDir: func(root, path string) (entries []fs.DirEntry, err error) {
			err = state.with(root, func(files *filesystem.Handle) error {
				entries, err = files.ReadDir(path)
				return err
			})
			return entries, err
		},
	}
}

func completeQuarantine(root string, j Journal, operation journalOperation) ([]Evidence, error) {
	var evidence []Evidence
	for _, op := range j.Operations {
		if !op.residentTree() {
			continue
		}
		if err := operation.removeAll(root, op.Quarantine); err != nil {
			return evidence, fmt.Errorf("discard quarantine %s: %w", op.Quarantine, err)
		}
		evidence = append(evidence, Evidence{Action: "discarded", Path: op.Path})
	}
	dropQuarantineRoot(root, operation)
	return evidence, nil
}

// dropQuarantineRoot removes the shared quarantine root once it is empty. It is
// deliberately best effort: the root only becomes removable after the last tree
// has left it, and unrelated bytes someone else put under it are not this
// transaction's to delete.
func dropQuarantineRoot(root string, operation journalOperation) {
	_ = operation.remove(root, QuarantineRel())
}

// applyOperation performs one operation's mutation according to its kind.
func applyOperation(root string, op Operation, operation journalOperation) error {
	if op.residentTree() {
		return quarantineTree(root, op, operation)
	}
	if op.emptyDirectory() {
		expected, ok := operation.state.directoryExpectations[op.Path]
		if !ok || expected.apply == nil {
			return fmt.Errorf("%s has no retained preflight directory identity: %w", op.Path, filesystem.ErrIdentityChanged)
		}
		return operation.state.with(root, func(files *filesystem.Handle) error {
			return files.RemoveExpectedEmptyDirectory(op.Path, expected.apply, os.FileMode(op.Prior.Mode))
		})
	}
	return operation.applyExpected(root, op.Path, op.Prior, op.Replacement)
}

// applyJournaledOperation predeclares the possibility of an exchange residue
// before a file mutation. A successful application clears the marker durably.
// The returned booleans report whether filesystem application was attempted
// and whether it succeeded. cleanup is populated only by the target mutation,
// never by a journal-marker write.
func applyJournaledOperation(root string, j *Journal, index int, operation journalOperation) (attempted, applied bool, cleanup *filepublication.CommittedCleanupError, err error) {
	op := j.Operations[index]
	if op.residentTree() {
		err := applyOperation(root, op, operation)
		return true, err == nil, nil, err
	}
	j.Operations[index].PossibleResidue = true
	if err := operation.write(root, *j); err != nil {
		return false, false, nil, fmt.Errorf("predeclare possible exchange residue for %s: %w", op.Path, err)
	}
	if err := applyOperation(root, op, operation); err != nil {
		var cleanup *filepublication.CommittedCleanupError
		_ = errors.As(err, &cleanup)
		return true, false, cleanup, err
	}
	j.Operations[index].PossibleResidue = false
	if err := operation.write(root, *j); err != nil {
		return true, true, nil, fmt.Errorf("clear possible exchange residue for %s: %w", op.Path, err)
	}
	return true, true, nil, nil
}

// imagesEqual reports whether two images are byte-for-byte and mode-for-mode
// identical, so recovery can tell an untouched or already-restored path from a
// third-party edit.
func imagesEqual(a, b Image) bool {
	if a.Present != b.Present {
		return false
	}
	if !a.Present {
		return true
	}
	return a.Mode == b.Mode && string(a.Content) == string(b.Content)
}

// validateOperations enforces the structural contract: every path is safe and
// carries valid images, the non-lock operations are unique and sorted, and the
// final operation is the lock (which appears nowhere else). The lock path sorts
// before ordinary paths, so it is a distinguished last entry rather than part of
// the sorted run.
func validateOperations(ops []Operation) error {
	if len(ops) == 0 {
		return errors.New("journal has no operations")
	}
	lockRel := LockRel()
	if ops[len(ops)-1].Path != lockRel {
		return fmt.Errorf("journal does not end with the lock operation %q", lockRel)
	}
	var previous Operation
	paths := map[string]bool{}
	quarantines := map[string]bool{}
	for i, op := range ops {
		if !safeRelPath(op.Path) {
			return fmt.Errorf("journal operation %d has an unsafe path %q", i, op.Path)
		}
		if err := validateImage(op.Prior); err != nil {
			return fmt.Errorf("journal operation %q prior image: %w", op.Path, err)
		}
		if err := validateImage(op.Replacement); err != nil {
			return fmt.Errorf("journal operation %q replacement image: %w", op.Path, err)
		}
		if err := validateKind(op, quarantines); err != nil {
			return fmt.Errorf("journal operation %q: %w", op.Path, err)
		}
		if paths[op.Path] {
			return fmt.Errorf("journal operations contain duplicate path %q", op.Path)
		}
		paths[op.Path] = true
		if i == len(ops)-1 {
			break
		}
		if op.Path == lockRel {
			return fmt.Errorf("the lock operation %q may appear only last", lockRel)
		}
		if i > 0 && !operationLess(previous, op) {
			return fmt.Errorf("journal operations are not uniquely sorted in application order at %q", op.Path)
		}
		previous = op
	}
	return nil
}

func operationLess(a, b Operation) bool {
	if a.emptyDirectory() != b.emptyDirectory() {
		return !a.emptyDirectory()
	}
	if a.emptyDirectory() {
		aDepth := strings.Count(a.Path, "/")
		bDepth := strings.Count(b.Path, "/")
		if aDepth != bDepth {
			return aDepth > bDepth
		}
	}
	return a.Path < b.Path
}

// validateKind enforces the per-kind contract. A file operation carries no
// quarantine; a resident-tree operation carries no images, quarantines under the
// dedicated root, and never shares a quarantine destination with another
// operation, so one interrupted run can never fold two trees into one name.
func validateKind(op Operation, seen map[string]bool) error {
	switch op.Kind {
	case KindFile:
		if len(op.ExpectedEntries) != 0 {
			return errors.New("a file operation carries expected directory entries")
		}
		if op.Residue != "" && op.PossibleResidue {
			return errors.New("a file operation carries both exact and possible residue")
		}
		if op.Quarantine != "" {
			return fmt.Errorf("a file operation carries quarantine %q", op.Quarantine)
		}
		if op.Residue != "" && !safeRelPath(op.Residue) {
			return fmt.Errorf("unsafe indeterminate residue %q", op.Residue)
		}
		return nil
	case KindResidentTree:
		if len(op.ExpectedEntries) != 0 {
			return errors.New("a resident-tree operation carries expected directory entries")
		}
		if op.PossibleResidue {
			return errors.New("a resident-tree operation carries possible residue")
		}
		if op.Residue != "" {
			return fmt.Errorf("a resident-tree operation carries residue %q", op.Residue)
		}
		if op.Prior.Present || op.Replacement.Present {
			return errors.New("a resident-tree operation carries file images")
		}
		if !safeRelPath(op.Quarantine) {
			return fmt.Errorf("unsafe quarantine path %q", op.Quarantine)
		}
		prefix := QuarantineRel() + "/"
		if !strings.HasPrefix(op.Quarantine, prefix) {
			return fmt.Errorf("quarantine %q is outside %s", op.Quarantine, QuarantineRel())
		}
		if seen[op.Quarantine] {
			return fmt.Errorf("duplicate quarantine destination %q", op.Quarantine)
		}
		seen[op.Quarantine] = true
		return nil
	case KindEmptyDirectory:
		if op.Quarantine != "" {
			return fmt.Errorf("an empty-directory operation carries quarantine %q", op.Quarantine)
		}
		if op.Residue != "" && op.PossibleResidue {
			return errors.New("an empty-directory operation carries both exact and possible residue")
		}
		if op.Residue != "" && !safeRelPath(op.Residue) {
			return fmt.Errorf("unsafe indeterminate residue %q", op.Residue)
		}
		if !op.Prior.Present || op.Prior.Mode == 0 || len(op.Prior.Content) != 0 || op.Replacement.Present {
			return errors.New("an empty-directory operation must replace a present directory mode with absence")
		}
		for i, entry := range op.ExpectedEntries {
			if entry == "" || entry == "." || entry == ".." || strings.ContainsAny(entry, "/\\\r\n") {
				return fmt.Errorf("unsafe expected directory entry %q", entry)
			}
			if i > 0 && entry <= op.ExpectedEntries[i-1] {
				return errors.New("expected directory entries are not unique and sorted")
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown operation kind %q", op.Kind)
	}
}

// validateImage rejects a malformed image: a present image needs a nonzero
// permission mode, an absent image must carry no mode or content.
func validateImage(img Image) error {
	if img.Present {
		if img.Mode == 0 || img.Mode&^0o777 != 0 {
			return fmt.Errorf("present image has an invalid mode %#o", img.Mode)
		}
		return nil
	}
	if img.Mode != 0 || len(img.Content) != 0 {
		return errors.New("absent image carries a mode or content")
	}
	return nil
}

// safeRelPath reports whether p is a clean, relative, forward-slash path that
// stays inside the tree.
func safeRelPath(p string) bool {
	if p == "" || strings.ContainsAny(p, "\r\n") || strings.HasPrefix(p, "/") || filepath.IsAbs(filepath.FromSlash(p)) {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

func readJournal(files *filesystem.Handle) ([]byte, error) {
	expected, err := files.ExpectedIdentity(JournalRel)
	if err != nil {
		return nil, err
	}
	defer expected.Release() //nolint:errcheck // read-only journal capture owns no mutation
	content, _, err := files.ReadExpected(JournalRel, expected)
	return content, err
}

// LoadJournal reads and validates the journal under root. A malformed or
// contract-violating journal is a hard error naming the Git-restoration escape,
// so no caller mutates the tree on a journal it cannot trust.
func LoadJournal(root string) (Journal, error) {
	var b []byte
	err := withJournalFilesystem(root, func(files *filesystem.Handle) error {
		var readErr error
		b, readErr = readJournal(files)
		return readErr
	})
	if err != nil {
		return Journal{}, err
	}
	return ParseJournal(b)
}

func rejectDuplicateJSONFields(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := rejectDuplicateJSONValue(dec); err != nil {
		return err
	}
	return requireJSONEOF(dec)
}

func rejectDuplicateJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			nameToken, err := dec.Token()
			if err != nil {
				return err
			}
			name := nameToken.(string)
			if seen[name] {
				return fmt.Errorf("duplicate field %q", name)
			}
			seen[name] = true
			if err := rejectDuplicateJSONValue(dec); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := rejectDuplicateJSONValue(dec); err != nil {
				return err
			}
		}
	}
	_, err = dec.Token()
	return err
}

// ParseJournal validates a journal captured from an immutable snapshot. It is
// the staged-check counterpart of LoadJournal, sharing the exact journal
// contract without materializing index bytes into the working tree.
func ParseJournal(b []byte) (Journal, error) {
	if err := rejectDuplicateJSONFields(b); err != nil {
		return Journal{}, fmt.Errorf("malformed upgrade journal: %w; %s", err, gitRestorationGuidance)
	}
	var j Journal
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&j); err != nil {
		return Journal{}, fmt.Errorf("malformed upgrade journal: %w; %s", err, gitRestorationGuidance)
	}
	if j.Version != JournalVersion {
		return Journal{}, fmt.Errorf("unknown upgrade journal version %d; %s", j.Version, gitRestorationGuidance)
	}
	switch j.Phase {
	case phasePrepared, phaseApplying, phaseRollingBack, phaseLockCommitted:
	default:
		return Journal{}, fmt.Errorf("unknown upgrade journal phase %q; %s", j.Phase, gitRestorationGuidance)
	}
	if err := validateOperations(j.Operations); err != nil {
		return Journal{}, fmt.Errorf("invalid upgrade journal: %w; %s", err, gitRestorationGuidance)
	}
	if err := validateIndeterminateResidue(j); err != nil {
		return Journal{}, fmt.Errorf("invalid upgrade journal: %w; %s", err, gitRestorationGuidance)
	}
	if err := validateFinalLock(j); err != nil {
		return Journal{}, fmt.Errorf("invalid upgrade journal: %w; %s", err, gitRestorationGuidance)
	}
	return j, nil
}

func validateIndeterminateResidue(j Journal) error {
	count := 0
	for _, op := range j.Operations {
		if op.Residue != "" {
			count++
			if j.Phase != phaseRollingBack {
				return fmt.Errorf("phase %q carries an exact indeterminate residue", j.Phase)
			}
		}
		if op.PossibleResidue {
			count++
			if j.Phase != phaseApplying && j.Phase != phaseRollingBack {
				return fmt.Errorf("phase %q carries a possible indeterminate residue", j.Phase)
			}
		}
	}
	if count > 1 {
		return errors.New("journal carries more than one indeterminate residue")
	}
	return nil
}

// validateFinalLock binds the content seal to the recorded final lock image.
func validateFinalLock(j Journal) error {
	lock := j.Operations[len(j.Operations)-1]
	if lock.Kind != KindFile || !lock.Replacement.Present {
		return errors.New("final operation must replace a present lock file")
	}
	if j.FinalLockSHA256 == "" || j.FinalLockSHA256 != imageSHA(lock.Replacement) {
		return errors.New("final lock digest does not match the final lock replacement")
	}
	return nil
}

// commitTransaction journals ops, makes the journal durable, applies every
// non-lock replacement, writes the lock last, marks the journal lock-committed,
// and deletes it. A failure before the lock is written rolls back to the prior
// images and clears the journal; a failure after leaves the sealed lock plus a
// recoverable journal. It returns ordered typed terminal evidence; the command
// owner renders it only after the outcome is known.
func retainedJournal(string) Evidence {
	return Evidence{Action: "retained", Path: JournalRel}
}

func appendEvidence(values []Evidence, evidence ...Evidence) []Evidence {
	return append(append([]Evidence(nil), values...), evidence...)
}

func outcomeWithRetainedJournal(root string, evidence, changed []Evidence) Outcome {
	retained := retainedJournal(root)
	return Outcome{Evidence: appendEvidence(evidence, retained), Changed: appendEvidence(changed, retained)}
}

// committedChanged reduces committed evidence to axes that remain true when the
// transaction returns. Evidence remains the complete ordered history; a
// discarded resident replaces its earlier applied fact in Changed, while an
// undiscarded resident remains applied after partial cleanup.
func committedChanged(j Journal, evidence []Evidence) []Evidence {
	discarded := map[string]bool{}
	residents := map[string]bool{}
	for _, op := range j.Operations {
		if op.residentTree() {
			residents[op.Path] = true
		}
	}
	for _, fact := range evidence {
		if fact.Action == "discarded" && residents[fact.Path] {
			discarded[fact.Path] = true
		}
	}
	changed := make([]Evidence, 0, len(evidence))
	for _, fact := range evidence {
		if fact.Action == "applied" && discarded[fact.Path] {
			continue
		}
		changed = append(changed, fact)
	}
	return changed
}

func commitTransactionBound(root string, ops []Operation, operation journalOperation) (Outcome, error) {
	if err := validateOperations(ops); err != nil {
		return Outcome{}, err
	}
	j := Journal{Version: JournalVersion, Phase: phasePrepared, FinalLockSHA256: imageSHA(ops[len(ops)-1].Replacement), Operations: ops}
	if err := operation.write(root, j); err != nil {
		return Outcome{}, err
	}
	j.Phase = phaseApplying
	if err := operation.write(root, j); err != nil {
		return outcomeWithRetainedJournal(root, nil, nil), err
	}
	evidence := make([]Evidence, 0, len(ops)+1)
	lockIndex := len(j.Operations) - 1
	lockOp := j.Operations[lockIndex]
	for i := range j.Operations[:lockIndex] {
		op := j.Operations[i]
		attempted, applied, cleanup, err := applyJournaledOperation(root, &j, i, operation)
		if err != nil {
			candidate := evidence
			if applied {
				candidate = appendEvidence(evidence, Evidence{Action: "applied", Path: op.Path})
			} else if attempted && !guaranteedUncommittedIdentityRefusal(err) {
				candidate = appendEvidence(evidence, Evidence{Action: "pending", Path: op.Path})
			}
			return rollBack(root, j, fmt.Errorf("apply %s: %w", op.Path, err), candidate, cleanup, operation)
		}
		evidence = append(evidence, Evidence{Action: "applied", Path: op.Path})
	}
	attempted, applied, cleanup, err := applyJournaledOperation(root, &j, lockIndex, operation)
	if err != nil {
		candidate := evidence
		if applied {
			candidate = appendEvidence(evidence, Evidence{Action: "applied", Path: lockOp.Path})
		} else if attempted && !guaranteedUncommittedIdentityRefusal(err) {
			candidate = appendEvidence(evidence, Evidence{Action: "pending", Path: lockOp.Path})
		}
		return rollBack(root, j, fmt.Errorf("apply %s: %w", lockOp.Path, err), candidate, cleanup, operation)
	}
	evidence = append(evidence, Evidence{Action: "applied", Path: lockOp.Path})
	evidence = append(evidence, Evidence{Action: "committed", Path: LockRel()})
	j.Phase = phaseLockCommitted
	if err := operation.write(root, j); err != nil {
		return outcomeWithRetainedJournal(root, evidence, evidence), fmt.Errorf("lock committed but journal update failed (%w); run `awf upgrade --recover`", err)
	}
	// Authority is committed from here on, so quarantined trees are discarded
	// rather than restored. A fault leaves a recoverable journal, never a rollback.
	discarded, err := completeQuarantine(root, j, operation)
	evidence = append(evidence, discarded...)
	changed := committedChanged(j, evidence)
	if err != nil {
		return outcomeWithRetainedJournal(root, evidence, changed), fmt.Errorf("lock committed but quarantine cleanup failed (%w); run `awf upgrade --recover`", err)
	}
	if err := operation.remove(root, JournalRel); err != nil {
		return outcomeWithRetainedJournal(root, evidence, changed), fmt.Errorf("lock committed but journal cleanup failed (%w); run `awf upgrade --recover`", err)
	}
	return Outcome{Evidence: evidence, Changed: changed}, nil
}

func guaranteedUncommittedIdentityRefusal(err error) bool {
	if !errors.Is(err, filesystem.ErrIdentityChanged) && !errors.Is(err, filesystem.ErrDirectoryNotEmpty) {
		return false
	}
	var committed *filepublication.CommittedCleanupError
	return !errors.As(err, &committed)
}

// rollBack enters the rolling-back phase and restores every prior image in
// reverse. It preserves the journal and reports the exact path on a third-party
// image or a failed restore; on full restoration it clears the journal.
func rollBack(root string, j Journal, cause error, applied []Evidence, committed *filepublication.CommittedCleanupError, operation journalOperation) (Outcome, error) {
	j.Phase = phaseRollingBack
	if committed != nil {
		if err := recordIndeterminateResidue(&j, committed); err != nil {
			return outcomeWithRetainedJournal(root, applied, applied), fmt.Errorf("%w; and committed cleanup could not be recorded: %w", cause, err)
		}
	} else if err := clearUnmaterializedPossibleResidue(root, &j, operation); err != nil {
		changed := appendPossibleResidueEvidence(applied, j)
		writeErr := operation.write(root, j)
		return outcomeWithRetainedJournal(root, changed, changed), fmt.Errorf("%w; possible exchange cleanup is indeterminate: %w", cause, errors.Join(err, writeErr))
	}
	if err := operation.write(root, j); err != nil {
		changed := appendPossibleResidueEvidence(applied, j)
		return outcomeWithRetainedJournal(root, changed, changed), fmt.Errorf("%w; and the journal could not record rollback: %w", cause, err)
	}
	if committed != nil {
		return outcomeWithRetainedJournal(root, applied, applied), fmt.Errorf("%w; committed cleanup is indeterminate at %s; run `awf upgrade --recover`", cause, committed.ResiduePath)
	}
	restored, remaining, err := restorePriors(root, j, applied, operation)
	evidence := appendEvidence(applied, restored...)
	if err != nil {
		return outcomeWithRetainedJournal(root, evidence, remaining), fmt.Errorf("%w; rollback halted: %w", cause, err)
	}
	if err := operation.remove(root, JournalRel); err != nil {
		return outcomeWithRetainedJournal(root, evidence, nil), fmt.Errorf("%w; rollback done but journal cleanup failed: %w", cause, err)
	}
	return Outcome{Evidence: evidence, Changed: []Evidence{}}, cause
}

func clearUnmaterializedPossibleResidue(root string, j *Journal, operation journalOperation) error {
	for i := range j.Operations {
		op := &j.Operations[i]
		if !op.PossibleResidue {
			continue
		}
		paths, err := possibleResiduePaths(root, *op, operation)
		if err != nil {
			return err
		}
		if len(paths) != 0 {
			return fmt.Errorf("possible exchange residue for %s remains at %s", op.Path, strings.Join(paths, ", "))
		}
		op.PossibleResidue = false
	}
	return nil
}

func possibleResiduePaths(root string, op Operation, operation journalOperation) ([]string, error) {
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(op.Path)))
	prefix := ".awf-remove-"
	if op.Replacement.Present && !op.Prior.Present {
		prefix = ".filepublication-"
	} else if op.Replacement.Present {
		prefix = ".awf-atomic-"
	}
	entries, err := operation.readDir(root, parent)
	if err != nil {
		return nil, fmt.Errorf("inspect possible exchange residues for %s: %w", op.Path, err)
	}
	var matches []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			matches = append(matches, filepath.ToSlash(filepath.Join(filepath.FromSlash(parent), entry.Name())))
		}
	}
	return matches, nil
}

func appendPossibleResidueEvidence(evidence []Evidence, j Journal) []Evidence {
	out := append([]Evidence(nil), evidence...)
	for _, op := range j.Operations {
		if !op.PossibleResidue {
			continue
		}
		found := false
		for _, fact := range out {
			if (fact.Action == "applied" || fact.Action == "pending") && fact.Path == op.Path {
				found = true
				break
			}
		}
		if !found {
			out = append(out, Evidence{Action: "pending", Path: op.Path})
		}
	}
	return out
}

func recordIndeterminateResidue(j *Journal, cleanup *filepublication.CommittedCleanupError) error {
	if cleanup == nil || !safeRelPath(cleanup.DestinationPath) || !safeRelPath(cleanup.ResiduePath) {
		return errors.New("committed cleanup has unsafe or missing paths")
	}
	for i := range j.Operations {
		if j.Operations[i].Path != cleanup.DestinationPath {
			continue
		}
		if j.Operations[i].residentTree() {
			return fmt.Errorf("committed cleanup destination %q is not a file operation", cleanup.DestinationPath)
		}
		j.Operations[i].PossibleResidue = false
		j.Operations[i].Residue = cleanup.ResiduePath
		return nil
	}
	return fmt.Errorf("committed cleanup destination %q is not journaled", cleanup.DestinationPath)
}

func directoryRemovalApplied(root string, op Operation, operation journalOperation) (bool, error) {
	info, err := operation.lstat(root, op.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode().Perm() != os.FileMode(op.Prior.Mode).Perm() {
		return false, fmt.Errorf("path %s no longer has the planned directory shape: %w", op.Path, filesystem.ErrIdentityChanged)
	}
	return false, nil
}

func evidencePaths(evidence []Evidence) map[string]bool {
	paths := make(map[string]bool, len(evidence))
	for _, fact := range evidence {
		if fact.Action == "applied" || fact.Action == "pending" {
			paths[fact.Path] = true
		}
	}
	return paths
}

func verifyDirectoryRestoreBarriers(root string, j Journal, changed map[string]bool, operation journalOperation) error {
	for _, op := range j.Operations {
		if !op.emptyDirectory() || changed[op.Path] {
			continue
		}
		hasChangedDescendant := false
		prefix := op.Path + "/"
		for path := range changed {
			if strings.HasPrefix(path, prefix) {
				hasChangedDescendant = true
				break
			}
		}
		if !hasChangedDescendant {
			continue
		}
		expected, ok := operation.state.directoryExpectations[op.Path]
		if !ok || expected.rollback == nil {
			return fmt.Errorf("cannot safely restore descendants through %s after interruption because its preflight directory identity is unavailable; %s", op.Path, gitRestorationGuidance)
		}
		info, err := operation.lstat(root, op.Path)
		if err != nil {
			return fmt.Errorf("inspect rollback directory %s: %w", op.Path, err)
		}
		if !expected.rollback.SameFile(info) {
			return fmt.Errorf("cannot safely restore descendants through replaced directory %s: %w", op.Path, filesystem.ErrIdentityChanged)
		}
	}
	return nil
}

// restorePriors restores only operations known to have applied, walking them in
// reverse. The remaining set is the terminal changed set if restoration halts.
func restorePriors(root string, j Journal, applied []Evidence, operation journalOperation) ([]Evidence, []Evidence, error) {
	byPath := evidencePaths(applied)
	remaining := append([]Evidence(nil), applied...)
	if err := verifyDirectoryRestoreBarriers(root, j, byPath, operation); err != nil {
		return nil, remaining, err
	}
	var evidence []Evidence
	for i := len(j.Operations) - 1; i >= 0; i-- {
		op := j.Operations[i]
		if !byPath[op.Path] {
			continue
		}
		if op.emptyDirectory() {
			if _, err := operation.lstat(root, op.Path); err == nil {
				return evidence, remaining, fmt.Errorf("cannot restore %s because it already exists; %s", op.Path, gitRestorationGuidance)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return evidence, remaining, fmt.Errorf("inspect %s: %w", op.Path, err)
			}
			if err := operation.createDirectory(root, op.Path, os.FileMode(op.Prior.Mode)); err != nil {
				return evidence, remaining, fmt.Errorf("restore %s: %w", op.Path, err)
			}
			evidence = append(evidence, Evidence{Action: "restored", Path: op.Path})
			remaining = removeChangedPath(remaining, op.Path)
			continue
		}
		if op.residentTree() {
			if err := restoreTree(root, op, operation); err != nil {
				return evidence, remaining, fmt.Errorf("restore %s: %w", op.Path, err)
			}
			evidence = append(evidence, Evidence{Action: "restored", Path: op.Path})
			remaining = removeChangedPath(remaining, op.Path)
			continue
		}
		current, err := operation.imageOf(root, op.Path)
		if err != nil {
			return evidence, remaining, fmt.Errorf("read %s: %w", op.Path, err)
		}
		if imagesEqual(current, op.Prior) {
			evidence = append(evidence, Evidence{Action: "restored", Path: op.Path})
			remaining = removeChangedPath(remaining, op.Path)
			continue
		}
		if !imagesEqual(current, op.Replacement) {
			return evidence, remaining, fmt.Errorf("path %s was modified outside the transaction; %s", op.Path, gitRestorationGuidance)
		}
		if err := operation.applyExpected(root, op.Path, op.Replacement, op.Prior); err != nil {
			return evidence, remaining, fmt.Errorf("restore %s: %w", op.Path, err)
		}
		evidence = append(evidence, Evidence{Action: "restored", Path: op.Path})
		remaining = removeChangedPath(remaining, op.Path)
	}
	// Every tree is home again, so a fully restored transaction leaves no
	// quarantine residue behind for the next run to trip over.
	dropQuarantineRoot(root, operation)
	return evidence, remaining, nil
}

func removeChangedPath(changed []Evidence, path string) []Evidence {
	for i := len(changed) - 1; i >= 0; i-- {
		if (changed[i].Action == "applied" || changed[i].Action == "pending") && changed[i].Path == path {
			return append(changed[:i:i], changed[i+1:]...)
		}
	}
	return changed
}

// Recover applies the journal recovery decision table. It is the only project
// mode permitted while a journal exists.
func Recover(root string) (Outcome, error) {
	return recoverWith(root, productionJournalOperation())
}

func recoverWith(root string, operation journalOperation) (outcome Outcome, err error) {
	err = withBoundJournalOperation(root, operation, func(bound journalOperation) error {
		var runErr error
		outcome, runErr = recoverBound(root, bound)
		return runErr
	})
	return outcome, err
}

func recoverBound(root string, operation journalOperation) (Outcome, error) {
	var j Journal
	var bytes []byte
	if err := operation.state.with(root, func(files *filesystem.Handle) error {
		var readErr error
		bytes, readErr = readJournal(files)
		return readErr
	}); err != nil {
		return Outcome{}, err
	}
	j, err := ParseJournal(bytes)
	if err != nil {
		return Outcome{}, err
	}
	if outcome, blocked, err := reconcileIndeterminateResidue(root, &j, operation); blocked || err != nil {
		return outcome, err
	}
	current, err := operation.imageOf(root, LockRel())
	if err != nil {
		return outcomeWithRetainedJournal(root, nil, nil), err
	}
	lockIsFinal := imagesEqual(current, j.Operations[len(j.Operations)-1].Replacement)
	if j.Phase == phaseLockCommitted {
		if lockIsFinal {
			return finishCommitted(root, j, operation)
		}
		return outcomeWithRetainedJournal(root, nil, nil), fmt.Errorf("journal is lock-committed but the exact lock image differs; refusing to roll committed authority back; %s", gitRestorationGuidance)
	}
	if lockIsFinal && j.Phase != phaseRollingBack {
		// The lock was written before the applying phase advanced; treat it as committed.
		return finishCommitted(root, j, operation)
	}
	applied, err := appliedOperations(root, j, operation)
	if err != nil {
		return outcomeWithRetainedJournal(root, nil, nil), err
	}
	j.Phase = phaseRollingBack
	if err := operation.write(root, j); err != nil {
		return outcomeWithRetainedJournal(root, applied, applied), err
	}
	restored, remaining, err := restorePriors(root, j, applied, operation)
	evidence := appendEvidence(applied, restored...)
	if err != nil {
		return outcomeWithRetainedJournal(root, evidence, remaining), err
	}
	return cleanupJournal(root, evidence, nil, operation)
}

func reconcileIndeterminateResidue(root string, j *Journal, operation journalOperation) (Outcome, bool, error) {
	for i := range j.Operations {
		op := &j.Operations[i]
		if op.Residue == "" && !op.PossibleResidue {
			continue
		}
		pending := []Evidence{{Action: "pending", Path: op.Path}}
		if op.Residue != "" {
			if _, err := operation.lstat(root, op.Residue); err == nil {
				outcome := outcomeWithRetainedJournal(root, nil, pending)
				return outcome, true, fmt.Errorf("indeterminate exchange residue %s is preserved for %s; move or reconcile it, then rerun `awf upgrade --recover`", op.Residue, op.Path)
			} else if !errors.Is(err, fs.ErrNotExist) {
				outcome := outcomeWithRetainedJournal(root, nil, pending)
				return outcome, true, fmt.Errorf("inspect indeterminate exchange residue %s: %w", op.Residue, err)
			}
			op.Residue = ""
			j.Phase = phaseRollingBack
		} else {
			paths, err := possibleResiduePaths(root, *op, operation)
			if err != nil {
				outcome := outcomeWithRetainedJournal(root, nil, pending)
				return outcome, true, err
			}
			if len(paths) != 0 {
				outcome := outcomeWithRetainedJournal(root, nil, pending)
				return outcome, true, fmt.Errorf("possible exchange residue for %s remains at %s; move or reconcile it, then rerun `awf upgrade --recover`", op.Path, strings.Join(paths, ", "))
			}
			op.PossibleResidue = false
			// A durable in-flight marker predates proof that the operation
			// completed. Recovery therefore resolves it toward rollback even
			// when the destination happens to match the replacement.
			j.Phase = phaseRollingBack
		}
		if err := operation.write(root, *j); err != nil {
			outcome := outcomeWithRetainedJournal(root, nil, pending)
			return outcome, true, fmt.Errorf("record reconciled exchange residue %s: %w", op.Path, err)
		}
		return Outcome{}, false, nil
	}
	return Outcome{}, false, nil
}

// finishCommitted completes a transaction whose lock is already the sealed
// authority: quarantined trees are discarded, never restored, and then the
// journal residue is cleared.
func finishCommitted(root string, j Journal, operation journalOperation) (Outcome, error) {
	// An exact final lock image is the commitment proof. Reconstruct the
	// journal's ordered operations even when the crash happened before its phase update.
	evidence := make([]Evidence, 0, len(j.Operations)+2)
	for _, op := range j.Operations {
		evidence = append(evidence, Evidence{Action: "applied", Path: op.Path})
	}
	evidence = append(evidence, Evidence{Action: "committed", Path: LockRel()})
	discarded, err := completeQuarantine(root, j, operation)
	evidence = append(evidence, discarded...)
	changed := committedChanged(j, evidence)
	if err != nil {
		return outcomeWithRetainedJournal(root, evidence, changed), err
	}
	return cleanupJournal(root, evidence, changed, operation)
}

// cleanupJournal removes the journal residue idempotently.
func cleanupJournal(root string, evidence, changed []Evidence, operation journalOperation) (Outcome, error) {
	if err := operation.remove(root, JournalRel); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return outcomeWithRetainedJournal(root, evidence, changed), fmt.Errorf("remove current-state upgrade journal: %w", err)
	}
	evidence = append(evidence, Evidence{Action: "recovered", Path: JournalRel})
	return Outcome{Evidence: evidence, Changed: changed}, nil
}

func appliedOperations(root string, j Journal, operation journalOperation) ([]Evidence, error) {
	var applied []Evidence
	for _, op := range j.Operations {
		if op.emptyDirectory() {
			removed, err := directoryRemovalApplied(root, op, operation)
			if err != nil {
				return nil, err
			}
			if removed {
				applied = append(applied, Evidence{Action: "applied", Path: op.Path})
			}
			continue
		}
		if op.residentTree() {
			if _, err := operation.lstat(root, op.Quarantine); err == nil {
				applied = append(applied, Evidence{Action: "applied", Path: op.Path})
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("inspect quarantine %s: %w", op.Quarantine, err)
			}
			continue
		}
		current, err := operation.imageOf(root, op.Path)
		if err != nil {
			return nil, err
		}
		if imagesEqual(current, op.Replacement) {
			applied = append(applied, Evidence{Action: "applied", Path: op.Path})
		} else if !imagesEqual(current, op.Prior) {
			// A non-prior image may be a failed atomic operation or a third-party
			// edit; retain it as a safety candidate so restoration verifies it.
			applied = append(applied, Evidence{Action: "pending", Path: op.Path})
		}
	}
	if err := verifyDirectoryRestoreBarriers(root, j, evidencePaths(applied), operation); err != nil {
		return nil, err
	}
	return applied, nil
}

// imageSHA is the SHA-256 of a present image's content (empty for an absent
// image), used to seal the journal's recorded final lock content.
func imageSHA(img Image) string {
	sum := sha256.Sum256(img.Content)
	return hex.EncodeToString(sum[:])
}

// requireJSONEOF rejects a second JSON value or any trailing non-whitespace input.
func requireJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return fmt.Errorf("trailing input: %w", err)
	}
	return nil
}
