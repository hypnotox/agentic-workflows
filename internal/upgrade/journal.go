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
	KindFile         = ""
	KindResidentTree = "resident-tree"
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
// operation is always the lock replacement. A resident-tree operation carries no
// images and instead names the quarantine path its tree is renamed to; the
// rename is the mutation, so it is reversible before the lock commits and only
// needs deleting after.
type Operation struct {
	Path        string `json:"path"`
	Kind        string `json:"kind,omitempty"`
	Prior       Image  `json:"prior"`
	Replacement Image  `json:"replacement"`
	Quarantine  string `json:"quarantine,omitempty"`
}

// residentTree reports whether op quarantines a tree rather than imaging a file.
func (o Operation) residentTree() bool { return o.Kind == KindResidentTree }

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
// are unique, sorted, and end with the lock operation; FinalLockSHA256 is the
// SHA-256 of the sealed lock content the transaction commits.
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
		_, err := files.Info(JournalRel)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
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

type journalFilesystemState struct{ files *filesystem.Handle }

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
	runErr := run(operation)
	operation.state.files = nil
	return errors.Join(runErr, files.Close())
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
	state      *journalFilesystemState
	removeAll  func(string, string) error
	remove     func(string, string) error
	imageOf    func(string, string) (Image, error)
	applyImage func(string, string, Image) error
	write      func(string, Journal) error
	lstat      func(string, string) (os.FileInfo, error)
	mkdirAll   func(string, string, os.FileMode) error
	rename     func(string, string, string) error
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
				content, mode, readErr := files.ReadWithMode(path)
				if errors.Is(readErr, fs.ErrNotExist) {
					image = Image{}
					return nil
				}
				if readErr != nil {
					return readErr
				}
				image = Image{Present: true, Mode: uint32(mode.Perm()), Content: content}
				return nil
			})
			return image, err
		},
		applyImage: func(root, path string, img Image) error {
			return state.with(root, func(files *filesystem.Handle) error {
				if !img.Present {
					if err := files.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
						return err
					}
					return nil
				}
				parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
				if err := files.MkdirAll(parent, 0o755); err != nil {
					return err
				}
				return files.Replace(path, img.Content, os.FileMode(img.Mode))
			})
		},
		write: func(root string, j Journal) error {
			b, err := json.MarshalIndent(j, "", "  ")
			if err != nil { // coverage-ignore: Journal holds only JSON-representable scalars, slices, and byte content
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
		rename: func(root, oldPath, newPath string) error {
			return state.with(root, func(files *filesystem.Handle) error { return files.Rename(oldPath, newPath) })
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
	return operation.applyImage(root, op.Path, op.Replacement)
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
	var last string
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
		if i == len(ops)-1 {
			break
		}
		if op.Path == lockRel {
			return fmt.Errorf("the lock operation %q may appear only last", lockRel)
		}
		if i > 0 && op.Path <= last {
			return fmt.Errorf("journal operations are not unique and sorted at %q", op.Path)
		}
		last = op.Path
	}
	return nil
}

// validateKind enforces the per-kind contract. A file operation carries no
// quarantine; a resident-tree operation carries no images, quarantines under the
// dedicated root, and never shares a quarantine destination with another
// operation, so one interrupted run can never fold two trees into one name.
func validateKind(op Operation, seen map[string]bool) error {
	switch op.Kind {
	case KindFile:
		if op.Quarantine != "" {
			return fmt.Errorf("a file operation carries quarantine %q", op.Quarantine)
		}
		return nil
	case KindResidentTree:
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

// LoadJournal reads and validates the journal under root. A malformed or
// contract-violating journal is a hard error naming the Git-restoration escape,
// so no caller mutates the tree on a journal it cannot trust.
func LoadJournal(root string) (Journal, error) {
	var b []byte
	err := withJournalFilesystem(root, func(files *filesystem.Handle) error {
		var readErr error
		b, readErr = files.Read(JournalRel)
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
	if err := validateFinalLock(j); err != nil {
		return Journal{}, fmt.Errorf("invalid upgrade journal: %w; %s", err, gitRestorationGuidance)
	}
	return j, nil
}

// validateFinalLock binds recovery's commitment proof to the actual final lock replacement.
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
	if err := validateOperations(ops); err != nil { // coverage-ignore: the supported-migration planner validated the same set before this call
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
	lockOp := ops[len(ops)-1]
	for _, op := range ops[:len(ops)-1] {
		if err := applyOperation(root, op, operation); err != nil {
			candidate := appendEvidence(evidence, Evidence{Action: "pending", Path: op.Path})
			return rollBack(root, j, fmt.Errorf("apply %s: %w", op.Path, err), candidate, operation)
		}
		evidence = append(evidence, Evidence{Action: "applied", Path: op.Path})
	}
	if err := operation.applyImage(root, lockOp.Path, lockOp.Replacement); err != nil {
		candidate := appendEvidence(evidence, Evidence{Action: "pending", Path: lockOp.Path})
		return rollBack(root, j, fmt.Errorf("apply %s: %w", lockOp.Path, err), candidate, operation)
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

// rollBack enters the rolling-back phase and restores every prior image in
// reverse. It preserves the journal and reports the exact path on a third-party
// image or a failed restore; on full restoration it clears the journal.
func rollBack(root string, j Journal, cause error, applied []Evidence, operation journalOperation) (Outcome, error) {
	j.Phase = phaseRollingBack
	if err := operation.write(root, j); err != nil {
		return outcomeWithRetainedJournal(root, applied, applied), fmt.Errorf("%w; and the journal could not record rollback: %w", cause, err)
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

// restorePriors restores only operations known to have applied, walking them in
// reverse. The remaining set is the terminal changed set if restoration halts.
func restorePriors(root string, j Journal, applied []Evidence, operation journalOperation) ([]Evidence, []Evidence, error) {
	byPath := map[string]bool{}
	for _, fact := range applied {
		if fact.Action == "applied" || fact.Action == "pending" {
			byPath[fact.Path] = true
		}
	}
	remaining := append([]Evidence(nil), applied...)
	var evidence []Evidence
	for i := len(j.Operations) - 1; i >= 0; i-- {
		op := j.Operations[i]
		if !byPath[op.Path] {
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
		if !imagesEqual(current, op.Prior) && !imagesEqual(current, op.Replacement) {
			return evidence, remaining, fmt.Errorf("path %s was modified outside the transaction; %s", op.Path, gitRestorationGuidance)
		}
		if err := operation.applyImage(root, op.Path, op.Prior); err != nil {
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
	return changed // coverage-ignore: no caller provides a changed set without its path
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
		bytes, readErr = files.Read(JournalRel)
		return readErr
	}); err != nil {
		return Outcome{}, err
	}
	j, err := ParseJournal(bytes)
	if err != nil {
		return Outcome{}, err
	}
	current, err := operation.imageOf(root, LockRel())
	if err != nil {
		return outcomeWithRetainedJournal(root, nil, nil), err
	}
	lockIsFinal := current.Present && imageSHA(current) == j.FinalLockSHA256
	if j.Phase == phaseLockCommitted {
		if lockIsFinal {
			return finishCommitted(root, j, operation)
		}
		return outcomeWithRetainedJournal(root, nil, nil), fmt.Errorf("journal is lock-committed but the lock hash differs; refusing to roll committed authority back; %s", gitRestorationGuidance)
	}
	if lockIsFinal {
		// The lock was written before the phase advanced; treat it as committed.
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

// finishCommitted completes a transaction whose lock is already the sealed
// authority: quarantined trees are discarded, never restored, and then the
// journal residue is cleared.
func finishCommitted(root string, j Journal, operation journalOperation) (Outcome, error) {
	// A final lock hash is the commitment proof. Reconstruct the journal's
	// ordered operations even when the crash happened before its phase update.
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
	return applied, nil
}

// imageSHA is the SHA-256 of a present image's content (empty for an absent
// image), used to compare a committed lock against the journal's final hash.
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
