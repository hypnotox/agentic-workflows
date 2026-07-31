package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// attestationVersion is the only accepted bridge attestation format version.
const attestationVersion = 1

// Verify checks only the sealed facts of att: its version, that current HEAD
// equals the sealed PreparedHead, and that the recomputed tree digest equals the
// sealed TreeDigest. It reads the tree read-only. The sealed legacy adjudication
// is trusted through this unchanged seal alone, because the current-state binary
// ships no inventory, approval parser, or cross-schema adapter to recompute it.
func Verify(ctx context.Context, root string, att *manifest.BridgeAttestation) error {
	if att.Version != attestationVersion {
		return fmt.Errorf("unsupported current-state attestation version %d", att.Version)
	}
	repo, _, err := git.OpenContaining(root)
	if err != nil {
		return err
	}
	head, err := repo.HeadHash(ctx)
	if err != nil {
		return err
	}
	if head != att.PreparedHead {
		return fmt.Errorf("HEAD %s does not match the sealed prepared head %s; %s", head, att.PreparedHead, gitRestorationGuidance)
	}
	digest, err := treeDigest(root)
	if err != nil { // coverage-ignore: a matching PreparedHead means the sealed config parsed at seal time; re-reading that same committed tree does not fault here
		return err
	}
	if digest != att.TreeDigest {
		return fmt.Errorf("prepared tree digest %s does not match the sealed digest %s; %s", digest, att.TreeDigest, gitRestorationGuidance)
	}
	return nil
}

// FinalUpgrade consumes a sealed bridge attestation. It verifies only the sealed
// facts, then journals the cutover output plan: the single deletion of the
// migration approval file and the lock replacement last, which drops the
// consumed attestation and promotes the sealed cutoff/gaps into permanent lock
// fields. The lock replacement is the transaction commit point; a pre-commit
// failure rolls back, a post-commit failure leaves a recoverable journal.
func FinalUpgrade(ctx context.Context, root string, lock *manifest.Lock, log io.Writer) error {
	state, err := lock.AuthorityState()
	if err != nil {
		return fmt.Errorf("invalid authority: restore .awf/awf.lock from version control; run `awf upgrade --recover` if a journal exists: %w", err)
	}
	if state != manifest.AuthorityBridge {
		return errors.New("no current-state attestation to consume")
	}
	att := lock.BridgeAttestation
	if err := Verify(ctx, root, att); err != nil {
		return err
	}
	ops, err := cutoverOperations(root, lock, att)
	if err != nil { // coverage-ignore: Verify already required the approval file present via the sealed digest, so cutoverOperations' only reachable error branch cannot fire here
		return err
	}
	return commitTransaction(root, ops, log)
}

// ResetLegacyResidents commits a schema advance that discards resident state,
// as one journaled transaction: every already-proven legacy resident is
// quarantined, then the lock is replaced last. The lock replacement is the
// commit point in both directions - a failure before it restores every
// quarantined resident and leaves the old generation authoritative, and a
// failure after it discards them and leaves the new generation authoritative.
// No binary older than the new generation runs against this tree again from
// the moment that lock lands.
//
// The residents must already have been proven obsolete by a read-only
// preflight; this function moves bytes and asks no questions about them. Only
// the schema generation is stamped, exactly as every other migration leaves the
// release version to the terminal sync; the generation alone is what makes an
// older binary refuse this tree.
func ResetLegacyResidents(root string, residents []string, schema int, log io.Writer) error {
	lockPrior, err := imageOf(root, LockRel())
	if err != nil { // coverage-ignore: the caller loaded this same lock immediately before
		return err
	}
	if !lockPrior.Present {
		// A tree with no lock yet is the legacy layout port's output, whose
		// terminal sync stamps the first lock; Generation already reads it as
		// current. There is nothing to advance and, on such a tree, nothing a
		// modern binary could have left behind to reset.
		if len(residents) == 0 {
			return nil
		}
		// Residents without a lock have no commit point to hang the reset on,
		// so the transaction refuses rather than discarding them unprotected.
		return fmt.Errorf("cannot reset %d legacy resident(s) because %s is absent; %s", len(residents), LockRel(), gitRestorationGuidance)
	}
	lock, err := manifest.Parse(lockPrior.Content)
	if err != nil { // coverage-ignore: the caller parsed this same lock immediately before
		return fmt.Errorf("invalid authority: restore %s from version control: %w", LockRel(), err)
	}
	lock.SchemaVersion = schema
	finalBytes, err := lock.Marshal()
	if err != nil { // coverage-ignore: the lock marshals cleanly; see manifest.Marshal
		return err
	}
	ops := make([]Operation, 0, len(residents)+1)
	for _, resident := range residents {
		ops = append(ops, Operation{Path: resident, Kind: KindResidentTree, Quarantine: quarantinePath(resident)})
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].Path < ops[j].Path })
	ops = append(ops, Operation{Path: LockRel(), Prior: lockPrior, Replacement: Image{Present: true, Mode: 0o644, Content: finalBytes}})
	if err := validateOperations(ops); err != nil {
		return fmt.Errorf("invalid resident reset plan: %w; %s", err, gitRestorationGuidance)
	}
	return commitTransaction(root, ops, log)
}

// quarantinePath maps a resident path to its destination under the quarantine
// root. The awf directory prefix is dropped and the remaining separators are
// folded, so the destination is a single flat leaf whose name still reads as
// the resident it came from. Two residents can never fold onto one name in
// practice, and the journal contract refuses the transaction outright if they
// ever did.
func quarantinePath(resident string) string {
	flat := strings.ReplaceAll(strings.TrimPrefix(resident, config.DirName+"/"), "/", "__")
	return QuarantineRel() + "/" + flat
}

// cutoverOperations builds the two-operation cutover plan: delete the sealed
// migration approval file, then replace the lock last. The replacement lock
// drops the attestation and stores the sealed cutoff/gaps permanently. The
// approval file must be present so the transaction journals exactly one
// deletion of it.
func cutoverOperations(root string, lock *manifest.Lock, att *manifest.BridgeAttestation) ([]Operation, error) {
	final := *lock
	final.BridgeAttestation = nil
	final.ADRFormatV1From = att.ADRFormatV1From
	final.LegacyADRGaps = att.LegacyADRGaps
	finalBytes, err := final.Marshal()
	if err != nil { // coverage-ignore: the lock marshals cleanly; see manifest.Marshal
		return nil, err
	}
	approvalPrior, err := imageOf(root, approvalPath)
	if err != nil { // coverage-ignore: the approval path reads cleanly unless a concurrent removal races
		return nil, err
	}
	if !approvalPrior.Present {
		return nil, fmt.Errorf("the sealed migration approval file %s is absent; %s", approvalPath, gitRestorationGuidance)
	}
	lockPrior, err := imageOf(root, LockRel())
	if err != nil { // coverage-ignore: the lock was read by LoadOptional immediately before this call
		return nil, err
	}
	ops := []Operation{
		{Path: approvalPath, Prior: approvalPrior, Replacement: Image{Present: false}},
		{Path: LockRel(), Prior: lockPrior, Replacement: Image{Present: true, Mode: 0o644, Content: finalBytes}},
	}
	if err := validateOperations(ops); err != nil { // coverage-ignore: the two-op cutover plan is well-formed by construction
		return nil, err
	}
	return ops, nil
}
