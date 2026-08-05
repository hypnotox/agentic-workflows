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

// LockRel is the repo-relative lock path every journal ends on.
func LockRel() string { return config.DirName + "/awf.lock" }

// JournalPath returns the fixed journal path under root.
func JournalPath(root string) string {
	return filepath.Join(root, config.DirName, "current-state-upgrade.journal")
}

// JournalPresent reports whether a journal file exists under root. A fault is
// returned rather than folded into absence: answering "no journal" from a read
// that never completed would let the command-state guard permit the commands an
// unrecovered upgrade must block.
func JournalPresent(root string) (bool, error) {
	if _, err := os.Stat(JournalPath(root)); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect current-state upgrade journal: %w", err)
	}
	return true, nil
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
	// Atomic like the journal that records it: a crash mid-restore would
	// otherwise leave a truncated file where the recovery had promised either
	// the prior image or the replacement, and nothing in between.
	return manifest.WriteFileAtomicMode(full, img.Content, os.FileMode(img.Mode))
}

// quarantineTree renames a resident tree aside. An absent source is already in
// the desired state, so a restarted run converges instead of failing. An
// existing destination is never overwritten: that would destroy the only copy
// of whatever a previous interrupted run put there.
func quarantineTree(root string, op Operation) error {
	source := filepath.Join(root, filepath.FromSlash(op.Path))
	destination := filepath.Join(root, filepath.FromSlash(op.Quarantine))
	if _, err := os.Lstat(source); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("quarantine destination %s already exists; %s", op.Quarantine, gitRestorationGuidance)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil { // coverage-ignore: the quarantine root's parent is the awf directory the journal itself lives in
		return err
	}
	return os.Rename(source, destination)
}

// restoreTree renames a quarantined tree back to its resident path. It mirrors
// quarantineTree's restart tolerance: an absent quarantine means the tree is
// already home, and an occupied resident path is never overwritten.
func restoreTree(root string, op Operation) error {
	source := filepath.Join(root, filepath.FromSlash(op.Quarantine))
	destination := filepath.Join(root, filepath.FromSlash(op.Path))
	if _, err := os.Lstat(source); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("cannot restore %s because it already exists; %s", op.Path, gitRestorationGuidance)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil { // coverage-ignore: the resident path's parent is the awf directory
		return err
	}
	return os.Rename(source, destination)
}

// completeQuarantine discards every quarantined tree once the lock has
// committed, then drops the shared quarantine root. It is idempotent so a
// repeated recovery converges rather than failing on already-deleted bytes.
var (
	quarantineRemoveAll = os.RemoveAll
	journalRemove       = os.Remove
	lockImageOf         = imageOf
	restoreImage        = applyImage
)

func completeQuarantine(root string, j Journal) ([]Evidence, error) {
	var evidence []Evidence
	for _, op := range j.Operations {
		if !op.residentTree() {
			continue
		}
		if err := quarantineRemoveAll(filepath.Join(root, filepath.FromSlash(op.Quarantine))); err != nil {
			return evidence, fmt.Errorf("discard quarantine %s: %w", op.Quarantine, err)
		}
		evidence = append(evidence, Evidence{Action: "discarded", Path: op.Path})
	}
	dropQuarantineRoot(root)
	return evidence, nil
}

// dropQuarantineRoot removes the shared quarantine root once it is empty. It is
// deliberately best effort: the root only becomes removable after the last tree
// has left it, and unrelated bytes someone else put under it are not this
// transaction's to delete.
func dropQuarantineRoot(root string) {
	_ = os.Remove(filepath.Join(root, filepath.FromSlash(QuarantineRel())))
}

// applyOperation performs one operation's mutation according to its kind.
func applyOperation(root string, op Operation) error {
	if op.residentTree() {
		return quarantineTree(root, op)
	}
	return applyImage(root, op.Path, op.Replacement)
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
var journalWrite = writeJournal

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
// recoverable journal. It returns ordered typed terminal evidence; the command
// owner renders it only after the outcome is known.
func retainedJournal(root string) Evidence {
	return Evidence{Action: "retained", Path: JournalPath(root)}
}

func appendEvidence(values []Evidence, evidence ...Evidence) []Evidence {
	return append(append([]Evidence(nil), values...), evidence...)
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

func commitTransaction(root string, ops []Operation) (Outcome, error) {
	if err := validateOperations(ops); err != nil { // coverage-ignore: the final-upgrade planner validated the same set before this call
		return Outcome{}, err
	}
	j := Journal{Version: JournalVersion, Phase: phasePrepared, FinalLockSHA256: imageSHA(ops[len(ops)-1].Replacement), Operations: ops}
	if err := journalWrite(root, j); err != nil {
		return Outcome{}, err
	}
	j.Phase = phaseApplying
	if err := journalWrite(root, j); err != nil {
		return Outcome{Changed: []Evidence{retainedJournal(root)}}, err
	}
	evidence := make([]Evidence, 0, len(ops)+1)
	lockOp := ops[len(ops)-1]
	for _, op := range ops[:len(ops)-1] {
		if err := applyOperation(root, op); err != nil {
			candidate := appendEvidence(evidence, Evidence{Action: "pending", Path: op.Path})
			return rollBack(root, j, fmt.Errorf("apply %s: %w", op.Path, err), candidate)
		}
		evidence = append(evidence, Evidence{Action: "applied", Path: op.Path})
	}
	if err := applyImage(root, lockOp.Path, lockOp.Replacement); err != nil {
		candidate := appendEvidence(evidence, Evidence{Action: "pending", Path: lockOp.Path})
		return rollBack(root, j, fmt.Errorf("apply %s: %w", lockOp.Path, err), candidate)
	}
	evidence = append(evidence, Evidence{Action: "applied", Path: lockOp.Path})
	evidence = append(evidence, Evidence{Action: "committed", Path: LockRel()})
	j.Phase = phaseLockCommitted
	if err := journalWrite(root, j); err != nil {
		return Outcome{Evidence: evidence, Changed: appendEvidence(evidence, retainedJournal(root))}, fmt.Errorf("lock committed but journal update failed (%w); run `awf upgrade --recover`", err)
	}
	// Authority is committed from here on, so quarantined trees are discarded
	// rather than restored. A fault leaves a recoverable journal, never a rollback.
	discarded, err := completeQuarantine(root, j)
	evidence = append(evidence, discarded...)
	changed := committedChanged(j, evidence)
	if err != nil {
		return Outcome{Evidence: evidence, Changed: appendEvidence(changed, retainedJournal(root))}, fmt.Errorf("lock committed but quarantine cleanup failed (%w); run `awf upgrade --recover`", err)
	}
	if err := journalRemove(JournalPath(root)); err != nil {
		return Outcome{Evidence: evidence, Changed: appendEvidence(changed, retainedJournal(root))}, fmt.Errorf("lock committed but journal cleanup failed (%w); run `awf upgrade --recover`", err)
	}
	return Outcome{Evidence: evidence, Changed: changed}, nil
}

// rollBack enters the rolling-back phase and restores every prior image in
// reverse. It preserves the journal and reports the exact path on a third-party
// image or a failed restore; on full restoration it clears the journal.
func rollBack(root string, j Journal, cause error, applied []Evidence) (Outcome, error) {
	j.Phase = phaseRollingBack
	if err := journalWrite(root, j); err != nil {
		return Outcome{Evidence: applied, Changed: appendEvidence(applied, retainedJournal(root))}, fmt.Errorf("%w; and the journal could not record rollback: %w", cause, err)
	}
	restored, remaining, err := restorePriors(root, j, applied)
	evidence := appendEvidence(applied, restored...)
	if err != nil {
		return Outcome{Evidence: evidence, Changed: appendEvidence(remaining, retainedJournal(root))}, fmt.Errorf("%w; rollback halted: %w", cause, err)
	}
	if err := journalRemove(JournalPath(root)); err != nil {
		return Outcome{Evidence: evidence, Changed: []Evidence{retainedJournal(root)}}, fmt.Errorf("%w; rollback done but journal cleanup failed: %w", cause, err)
	}
	return Outcome{Evidence: evidence, Changed: []Evidence{}}, cause
}

// restorePriors restores only operations known to have applied, walking them in
// reverse. The remaining set is the terminal changed set if restoration halts.
func restorePriors(root string, j Journal, applied []Evidence) ([]Evidence, []Evidence, error) {
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
			if err := restoreTree(root, op); err != nil {
				return evidence, remaining, fmt.Errorf("restore %s: %w", op.Path, err)
			}
			evidence = append(evidence, Evidence{Action: "restored", Path: op.Path})
			remaining = removeChangedPath(remaining, op.Path)
			continue
		}
		current, err := imageOf(root, op.Path)
		if err != nil {
			return evidence, remaining, fmt.Errorf("read %s: %w", op.Path, err)
		}
		if !imagesEqual(current, op.Prior) && !imagesEqual(current, op.Replacement) {
			return evidence, remaining, fmt.Errorf("path %s was modified outside the transaction; %s", op.Path, gitRestorationGuidance)
		}
		if err := restoreImage(root, op.Path, op.Prior); err != nil {
			return evidence, remaining, fmt.Errorf("restore %s: %w", op.Path, err)
		}
		evidence = append(evidence, Evidence{Action: "restored", Path: op.Path})
		remaining = removeChangedPath(remaining, op.Path)
	}
	// Every tree is home again, so a fully restored transaction leaves no
	// quarantine residue behind for the next run to trip over.
	dropQuarantineRoot(root)
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
	j, err := LoadJournal(root)
	if err != nil {
		return Outcome{}, err
	}
	current, err := lockImageOf(root, LockRel())
	if err != nil {
		return Outcome{Changed: []Evidence{retainedJournal(root)}}, err
	}
	lockIsFinal := current.Present && imageSHA(current) == j.FinalLockSHA256
	if j.Phase == phaseLockCommitted {
		if lockIsFinal {
			return finishCommitted(root, j)
		}
		return Outcome{Changed: []Evidence{retainedJournal(root)}}, fmt.Errorf("journal is lock-committed but the lock hash differs; refusing to roll committed authority back; %s", gitRestorationGuidance)
	}
	if lockIsFinal {
		// The lock was written before the phase advanced; treat it as committed.
		return finishCommitted(root, j)
	}
	applied, err := appliedOperations(root, j)
	if err != nil {
		return Outcome{Changed: []Evidence{retainedJournal(root)}}, err
	}
	j.Phase = phaseRollingBack
	if err := journalWrite(root, j); err != nil {
		return Outcome{Evidence: applied, Changed: appendEvidence(applied, retainedJournal(root))}, err
	}
	restored, remaining, err := restorePriors(root, j, applied)
	evidence := appendEvidence(applied, restored...)
	if err != nil {
		return Outcome{Evidence: evidence, Changed: appendEvidence(remaining, retainedJournal(root))}, err
	}
	return cleanupJournal(root, evidence, nil)
}

// finishCommitted completes a transaction whose lock is already the sealed
// authority: quarantined trees are discarded, never restored, and then the
// journal residue is cleared.
func finishCommitted(root string, j Journal) (Outcome, error) {
	// A final lock hash is the commitment proof. Reconstruct the journal's
	// ordered operations even when the crash happened before its phase update.
	evidence := make([]Evidence, 0, len(j.Operations)+2)
	for _, op := range j.Operations {
		evidence = append(evidence, Evidence{Action: "applied", Path: op.Path})
	}
	evidence = append(evidence, Evidence{Action: "committed", Path: LockRel()})
	discarded, err := completeQuarantine(root, j)
	evidence = append(evidence, discarded...)
	changed := committedChanged(j, evidence)
	if err != nil {
		return Outcome{Evidence: evidence, Changed: appendEvidence(changed, retainedJournal(root))}, err
	}
	return cleanupJournal(root, evidence, changed)
}

// cleanupJournal removes the journal residue idempotently.
func cleanupJournal(root string, evidence, changed []Evidence) (Outcome, error) {
	if err := journalRemove(JournalPath(root)); err != nil && !os.IsNotExist(err) {
		return Outcome{Evidence: evidence, Changed: appendEvidence(changed, retainedJournal(root))}, err
	}
	evidence = append(evidence, Evidence{Action: "recovered", Path: JournalPath(root)})
	return Outcome{Evidence: evidence, Changed: changed}, nil
}

var (
	appliedLstat   = os.Lstat
	appliedImageOf = imageOf
)

func appliedOperations(root string, j Journal) ([]Evidence, error) {
	var applied []Evidence
	for _, op := range j.Operations {
		if op.residentTree() {
			if _, err := appliedLstat(filepath.Join(root, filepath.FromSlash(op.Quarantine))); err == nil {
				applied = append(applied, Evidence{Action: "applied", Path: op.Path})
			} else if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		current, err := appliedImageOf(root, op.Path)
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
