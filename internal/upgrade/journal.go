// Package upgrade runs the permanent final current-state upgrade: it verifies
// only the sealed facts of a bridge attestation and applies the cutover output
// plan through a recoverable journal. It ships no legacy inventory, approval
// parser, or cross-schema adapter; the sealed attestation is the sole trust
// anchor for the legacy adjudication the current-state binary can no longer
// recompute.
package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// JournalVersion is the only accepted current-state upgrade journal schema.
const JournalVersion = 1

// Journal phases. A precommit phase (prepared, applying, rolling-back) has not
// necessarily written the final lock; lock-committed marks the lock as the
// sealed authority. Every valid phase blocks all project commands except
// `awf upgrade --recover`.
const (
	phasePrepared      = "prepared"
	phaseApplying      = "applying"
	phaseRollingBack   = "rolling-back"
	phaseLockCommitted = "lock-committed"
)

// gitRestorationGuidance is the deterministic escape hatch printed when a
// journal is unusable: the tree must be restored from Git and the bridge
// reinstalled rather than left half-migrated.
const gitRestorationGuidance = "restore the working tree from Git (git restore + git clean) and reinstall the bridge release before retrying"

// Image is one file's exact recorded state: present with an octal permission
// mode and content, or absent (present:false, mode 0, empty content).
type Image struct {
	Present bool   `json:"present"`
	Mode    uint32 `json:"mode"`
	Content []byte `json:"content"`
}

// Operation records one path's prior and replacement images. The final journal
// operation is always the lock replacement.
type Operation struct {
	Path        string `json:"path"`
	Prior       Image  `json:"prior"`
	Replacement Image  `json:"replacement"`
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

// LockRel is the repo-relative lock path every journal ends on.
func LockRel() string { return config.DirName + "/awf.lock" }

// JournalPath returns the fixed journal path under root.
func JournalPath(root string) string {
	return filepath.Join(root, config.DirName, "current-state-upgrade.journal")
}

// JournalPresent reports whether a journal file exists under root.
func JournalPresent(root string) bool {
	_, err := os.Stat(JournalPath(root))
	return err == nil
}

// imageOf reads path's current image from the working tree.
func imageOf(root, path string) (Image, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	content, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return Image{Present: false}, nil
		}
		return Image{}, err
	}
	info, err := os.Stat(full)
	if err != nil { // coverage-ignore: ReadFile just succeeded for this path; failure requires a concurrent filesystem race
		return Image{}, err
	}
	return Image{Present: true, Mode: uint32(info.Mode().Perm()), Content: content}, nil
}

// applyImage writes or removes path so it exactly matches img, chmod included.
func applyImage(root, path string, img Image) error {
	full := filepath.Join(root, filepath.FromSlash(path))
	if !img.Present {
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { // coverage-ignore: the parent of every journaled path exists in the prepared tree
		return err
	}
	if err := os.WriteFile(full, img.Content, os.FileMode(img.Mode)); err != nil {
		return err
	}
	return os.Chmod(full, os.FileMode(img.Mode))
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
	if p == "" || strings.HasPrefix(p, "/") || filepath.IsAbs(filepath.FromSlash(p)) {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// writeJournal serializes j and writes it durably (atomic temp-file rename)
// before any tree mutation observes the phase.
func writeJournal(root string, j Journal) error {
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil { // coverage-ignore: Journal holds only JSON-representable scalars, slices, and byte content
		return err
	}
	return manifest.WriteFileAtomic(JournalPath(root), append(b, '\n'))
}

// LoadJournal reads and validates the journal under root. A malformed or
// contract-violating journal is a hard error naming the Git-restoration escape,
// so no caller mutates the tree on a journal it cannot trust.
func LoadJournal(root string) (Journal, error) {
	b, err := os.ReadFile(JournalPath(root))
	if err != nil {
		return Journal{}, err
	}
	return ParseJournal(b)
}

// ParseJournal validates a journal captured from an immutable snapshot. It is
// the staged-check counterpart of LoadJournal, sharing the exact journal
// contract without materializing index bytes into the working tree.
func ParseJournal(b []byte) (Journal, error) {
	var j Journal
	dec := json.NewDecoder(strings.NewReader(string(b)))
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
	return j, nil
}

// commitTransaction journals ops, makes the journal durable, applies every
// non-lock replacement, writes the lock last, marks the journal lock-committed,
// and deletes it. A failure before the lock is written rolls back to the prior
// images and clears the journal; a failure after leaves the sealed lock plus a
// recoverable journal. It prints one deterministic operation line per applied
// path.
func commitTransaction(root string, ops []Operation, log io.Writer) error {
	if err := validateOperations(ops); err != nil { // coverage-ignore: the final-upgrade planner validated the same set before this call
		return err
	}
	j := Journal{Version: JournalVersion, Phase: phasePrepared, FinalLockSHA256: imageSHA(ops[len(ops)-1].Replacement), Operations: ops}
	if err := writeJournal(root, j); err != nil {
		return err
	}
	j.Phase = phaseApplying
	if err := writeJournal(root, j); err != nil { // coverage-ignore: the journal path just wrote durably; a second write fails only on a concurrent filesystem fault
		return err
	}
	lockOp := ops[len(ops)-1]
	for _, op := range ops[:len(ops)-1] {
		if err := applyImage(root, op.Path, op.Replacement); err != nil {
			return rollBack(root, j, fmt.Errorf("apply %s: %w", op.Path, err), log)
		}
		fmt.Fprintf(log, "operation: applied %s\n", op.Path)
	}
	if err := applyImage(root, lockOp.Path, lockOp.Replacement); err != nil {
		return rollBack(root, j, fmt.Errorf("apply %s: %w", lockOp.Path, err), log)
	}
	fmt.Fprintf(log, "operation: applied %s\n", lockOp.Path)
	j.Phase = phaseLockCommitted
	if err := writeJournal(root, j); err != nil { // coverage-ignore: the lock is committed; a phase-write fault leaves a recoverable journal that --recover cleans
		return fmt.Errorf("lock committed but journal update failed (%w); run `awf upgrade --recover`", err)
	}
	if err := os.Remove(JournalPath(root)); err != nil { // coverage-ignore: the lock is committed; a cleanup fault leaves a recoverable journal that --recover removes
		return fmt.Errorf("lock committed but journal cleanup failed (%w); run `awf upgrade --recover`", err)
	}
	fmt.Fprintln(log, "operation: upgrade committed")
	return nil
}

// rollBack enters the rolling-back phase and restores every prior image in
// reverse. It preserves the journal and reports the exact path on a third-party
// image or a failed restore; on full restoration it clears the journal.
func rollBack(root string, j Journal, cause error, log io.Writer) error {
	j.Phase = phaseRollingBack
	if err := writeJournal(root, j); err != nil { // coverage-ignore: the journal path is writable throughout the transaction
		return fmt.Errorf("%w; and the journal could not record rollback: %w", cause, err)
	}
	if err := restorePriors(root, j, log); err != nil {
		return fmt.Errorf("%w; rollback halted: %w", cause, err)
	}
	if err := os.Remove(JournalPath(root)); err != nil { // coverage-ignore: the journal was just written durably and the directory is writable
		return fmt.Errorf("%w; rollback done but journal cleanup failed: %w", cause, err)
	}
	fmt.Fprintln(log, "operation: rolled back")
	return cause
}

// restorePriors verifies each current image equals the prior or replacement,
// then restores the prior, walking operations in reverse.
func restorePriors(root string, j Journal, log io.Writer) error {
	for i := len(j.Operations) - 1; i >= 0; i-- {
		op := j.Operations[i]
		current, err := imageOf(root, op.Path)
		if err != nil {
			return fmt.Errorf("read %s: %w", op.Path, err)
		}
		if !imagesEqual(current, op.Prior) && !imagesEqual(current, op.Replacement) {
			return fmt.Errorf("path %s was modified outside the transaction; %s", op.Path, gitRestorationGuidance)
		}
		if err := applyImage(root, op.Path, op.Prior); err != nil { // coverage-ignore: the prior image was readable at journal time and the directory is writable
			return fmt.Errorf("restore %s: %w", op.Path, err)
		}
		fmt.Fprintf(log, "operation: restored %s\n", op.Path)
	}
	return nil
}

// Recover applies the journal recovery decision table. It is the only project
// mode permitted while a journal exists.
func Recover(root string, log io.Writer) error {
	j, err := LoadJournal(root)
	if err != nil {
		return err
	}
	current, err := imageOf(root, LockRel())
	if err != nil { // coverage-ignore: the lock path reads cleanly unless a concurrent removal races
		return err
	}
	lockIsFinal := current.Present && imageSHA(current) == j.FinalLockSHA256
	if j.Phase == phaseLockCommitted {
		if lockIsFinal {
			return cleanupJournal(root, log)
		}
		return fmt.Errorf("journal is lock-committed but the lock hash differs; refusing to roll committed authority back; %s", gitRestorationGuidance)
	}
	if lockIsFinal {
		// The lock was written before the phase advanced; treat it as committed.
		return cleanupJournal(root, log)
	}
	j.Phase = phaseRollingBack
	if err := writeJournal(root, j); err != nil { // coverage-ignore: the journal directory is writable during recovery
		return err
	}
	if err := restorePriors(root, j, log); err != nil {
		return err
	}
	return cleanupJournal(root, log)
}

// cleanupJournal removes the journal residue idempotently.
func cleanupJournal(root string, log io.Writer) error {
	if err := os.Remove(JournalPath(root)); err != nil && !os.IsNotExist(err) { // coverage-ignore: the journal was just loaded and the directory is writable
		return err
	}
	fmt.Fprintln(log, "operation: recovered")
	return nil
}

// imageSHA is the SHA-256 of a present image's content (empty for an absent
// image), used to compare a committed lock against the journal's final hash.
func imageSHA(img Image) string {
	sum := sha256.Sum256(img.Content)
	return hex.EncodeToString(sum[:])
}
